package app

import (
	"context"
	"fmt"
	"os"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
	"alex/internal/llm"
	"alex/internal/prompts"
)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory    *llm.Factory
	ToolRegistry  ports.ToolRegistry
	SessionStore  ports.SessionStore
	ContextMgr    ports.ContextManager
	Parser        ports.FunctionCallParser
	PromptLoader  *prompts.Loader
	Config        Config
	Logger        ports.Logger
	Clock         ports.Clock
	Analysis      *TaskAnalysisService
	CostDecorator *CostTrackingDecorator
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory    *llm.Factory
	toolRegistry  ports.ToolRegistry
	sessionStore  ports.SessionStore
	contextMgr    ports.ContextManager
	parser        ports.FunctionCallParser
	promptLoader  *prompts.Loader
	config        Config
	logger        ports.Logger
	clock         ports.Clock
	analysis      *TaskAnalysisService
	costDecorator *CostTrackingDecorator
}

// NewExecutionPreparationService creates a service instance.
func NewExecutionPreparationService(deps ExecutionPreparationDeps) *ExecutionPreparationService {
	logger := deps.Logger
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	clock := deps.Clock
	if clock == nil {
		clock = ports.SystemClock{}
	}

	promptLoader := deps.PromptLoader
	if promptLoader == nil {
		promptLoader = prompts.New()
	}

	analysis := deps.Analysis
	if analysis == nil {
		analysis = NewTaskAnalysisService(logger)
	}

	costDecorator := deps.CostDecorator
	if costDecorator == nil {
		costDecorator = NewCostTrackingDecorator(nil, logger, clock)
	}

	return &ExecutionPreparationService{
		llmFactory:    deps.LLMFactory,
		toolRegistry:  deps.ToolRegistry,
		sessionStore:  deps.SessionStore,
		contextMgr:    deps.ContextMgr,
		parser:        deps.Parser,
		promptLoader:  promptLoader,
		config:        deps.Config,
		logger:        logger,
		clock:         clock,
		analysis:      analysis,
		costDecorator: costDecorator,
	}
}

// Prepare builds the execution environment for a task.
func (s *ExecutionPreparationService) Prepare(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	s.logger.Info("PrepareExecution called: task='%s', sessionID='%s'", task, sessionID)

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if s.contextMgr.ShouldCompress(session.Messages, s.config.MaxTokens) {
		s.logger.Info("Context limit reached, compressing...")
		compressed, err := s.contextMgr.Compress(session.Messages, s.config.MaxTokens*80/100)
		if err != nil {
			return nil, fmt.Errorf("failed to compress context: %w", err)
		}
		s.logger.Info("Compression complete: %d -> %d messages", len(session.Messages), len(compressed))
		session.Messages = compressed
	}

	s.logger.Debug("Getting LLM client: provider=%s, model=%s", s.config.LLMProvider, s.config.LLMModel)
	llmClient, err := s.llmFactory.GetClient(s.config.LLMProvider, s.config.LLMModel, llm.Config{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM client: %w", err)
	}
	s.logger.Debug("LLM client obtained successfully")

	llmClient = s.costDecorator.Attach(ctx, sessionID, llmClient)

	analysis := s.analysis.Analyze(ctx, task, llmClient)
	var analysisInfo *prompts.TaskAnalysisInfo
	var taskAnalysis *ports.TaskAnalysis
	if analysis != nil && analysis.ActionName != "" {
		s.logger.Debug("Task pre-analysis: action=%s, goal=%s", analysis.ActionName, analysis.Goal)
		analysisInfo = &prompts.TaskAnalysisInfo{
			Action:   analysis.ActionName,
			Goal:     analysis.Goal,
			Approach: analysis.Approach,
		}
		taskAnalysis = &ports.TaskAnalysis{
			ActionName: analysis.ActionName,
			Goal:       analysis.Goal,
			Approach:   analysis.Approach,
		}
	} else {
		s.logger.Debug("Task pre-analysis skipped or failed")
	}

	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	systemPrompt := s.resolveSystemPrompt(ctx, workingDir, task, analysisInfo)
	state := &domain.TaskState{
		SystemPrompt: systemPrompt,
		Messages:     append([]domain.Message(nil), session.Messages...),
		SessionID:    session.ID,
	}

	toolRegistry := s.selectToolRegistry(ctx)
	services := domain.Services{
		LLM:          llmClient,
		ToolExecutor: toolRegistry,
		Parser:       s.parser,
		Context:      s.contextMgr,
	}

	s.logger.Info("Execution environment prepared successfully")

	return &ports.ExecutionEnvironment{
		State:        state,
		Services:     services,
		Session:      session,
		SystemPrompt: systemPrompt,
		TaskAnalysis: taskAnalysis,
	}, nil
}

func (s *ExecutionPreparationService) loadSession(ctx context.Context, id string) (*ports.Session, error) {
	if id == "" {
		session, err := s.sessionStore.Create(ctx)
		if err != nil {
			s.logger.Error("Failed to create session: %v", err)
		}
		return session, err
	}

	session, err := s.sessionStore.Get(ctx, id)
	if err != nil {
		s.logger.Error("Failed to load session: %v", err)
	}
	return session, err
}

func (s *ExecutionPreparationService) resolveSystemPrompt(ctx context.Context, workingDir, task string, analysis *prompts.TaskAnalysisInfo) string {
	agentPreset := s.config.AgentPreset
	presetSource := "config"
	if presetCfg, ok := ctx.Value(PresetContextKey{}).(PresetConfig); ok && presetCfg.AgentPreset != "" {
		agentPreset = presetCfg.AgentPreset
		presetSource = "context"
		s.logger.Debug("Using agent preset from context: %s", agentPreset)
	} else if agentPreset == "" {
		presetSource = ""
	}

	if agentPreset != "" && presets.IsValidPreset(agentPreset) {
		presetConfig, err := presets.GetPromptConfig(presets.AgentPreset(agentPreset))
		if err != nil {
			s.logger.Warn("Failed to load preset prompt: %v, using default", err)
			prompt, _ := s.promptLoader.GetSystemPrompt(workingDir, task, analysis)
			return prompt
		}
		if presetSource == "" {
			presetSource = "config"
		}
		s.logger.Info("Using preset system prompt: %s (source=%s)", presetConfig.Name, presetSource)
		return presetConfig.SystemPrompt
	}

	prompt, err := s.promptLoader.GetSystemPrompt(workingDir, task, analysis)
	if err != nil {
		s.logger.Warn("Failed to load system prompt: %v, using default", err)
		return "You are ALEX, a helpful AI coding assistant. Use available tools to help solve the user's task."
	}
	s.logger.Debug("System prompt loaded: %d bytes", len(prompt))
	return prompt
}

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context) ports.ToolRegistry {
	registry := s.toolRegistry
	if isSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")
	}

	toolPreset := s.config.ToolPreset
	presetSource := "config"
	if presetCfg, ok := ctx.Value(PresetContextKey{}).(PresetConfig); ok && presetCfg.ToolPreset != "" {
		toolPreset = presetCfg.ToolPreset
		presetSource = "context"
		s.logger.Debug("Using tool preset from context: %s", toolPreset)
	} else if toolPreset == "" {
		presetSource = ""
	}

	if toolPreset != "" && presets.IsValidToolPreset(toolPreset) {
		filteredRegistry, err := presets.NewFilteredToolRegistry(registry, presets.ToolPreset(toolPreset))
		if err != nil {
			s.logger.Warn("Failed to create filtered registry: %v, using default", err)
		} else {
			registry = filteredRegistry
			toolConfig, _ := presets.GetToolConfig(presets.ToolPreset(toolPreset))
			if presetSource == "" {
				presetSource = "config"
			}
			toolCount := len(filteredRegistry.List())
			s.logger.Info("Using tool preset: %s (source=%s, tool_count=%d)", toolConfig.Name, presetSource, toolCount)
		}
	}

	return registry
}

func (s *ExecutionPreparationService) getRegistryWithoutSubagent() ports.ToolRegistry {
	type registryWithFilter interface {
		WithoutSubagent() ports.ToolRegistry
	}

	if filtered, ok := s.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	return s.toolRegistry
}
