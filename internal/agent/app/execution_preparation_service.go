package app

import (
	"context"
	"fmt"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory      ports.LLMClientFactory
	ToolRegistry    ports.ToolRegistry
	SessionStore    ports.SessionStore
	ContextMgr      ports.ContextManager
	Parser          ports.FunctionCallParser
	PromptLoader    ports.PromptLoader
	Config          Config
	Logger          ports.Logger
	Clock           ports.Clock
	Analysis        *TaskAnalysisService
	CostDecorator   *CostTrackingDecorator
	PresetResolver  *PresetResolver // Optional: if nil, one will be created
	EventEmitter    ports.EventListener
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory     ports.LLMClientFactory
	toolRegistry   ports.ToolRegistry
	sessionStore   ports.SessionStore
	contextMgr     ports.ContextManager
	parser         ports.FunctionCallParser
	promptLoader   ports.PromptLoader
	config         Config
	logger         ports.Logger
	clock          ports.Clock
	analysis       *TaskAnalysisService
	costDecorator  *CostTrackingDecorator
	presetResolver *PresetResolver
	eventEmitter   ports.EventListener
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
	// PromptLoader is now a required dependency - must be provided by caller

	analysis := deps.Analysis
	if analysis == nil {
		analysis = NewTaskAnalysisService(logger)
	}

	costDecorator := deps.CostDecorator
	if costDecorator == nil {
		costDecorator = NewCostTrackingDecorator(nil, logger, clock)
	}

	eventEmitter := deps.EventEmitter
	if eventEmitter == nil {
		eventEmitter = ports.NoopEventListener{}
	}

	presetResolver := deps.PresetResolver
	if presetResolver == nil {
		presetResolver = NewPresetResolverWithDeps(PresetResolverDeps{
			PromptLoader: promptLoader,
			Logger:       logger,
			Clock:        clock,
			EventEmitter: eventEmitter,
		})
	}

	return &ExecutionPreparationService{
		llmFactory:     deps.LLMFactory,
		toolRegistry:   deps.ToolRegistry,
		sessionStore:   deps.SessionStore,
		contextMgr:     deps.ContextMgr,
		parser:         deps.Parser,
		promptLoader:   promptLoader,
		config:         deps.Config,
		logger:         logger,
		clock:          clock,
		analysis:       analysis,
		costDecorator:  costDecorator,
		presetResolver: presetResolver,
		eventEmitter:   eventEmitter,
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
		originalCount := len(session.Messages)
		compressed, err := s.contextMgr.Compress(session.Messages, s.config.MaxTokens*80/100)
		if err != nil {
			return nil, fmt.Errorf("failed to compress context: %w", err)
		}
		compressedCount := len(compressed)
		s.logger.Info("Compression complete: %d -> %d messages (%.1f%% retained)",
			originalCount, compressedCount, float64(compressedCount)/float64(originalCount)*100.0)

		// Emit compression metrics event
		compressionEvent := domain.NewContextCompressionEvent(
			ports.LevelCore,
			session.ID,
			originalCount,
			compressedCount,
			s.clock.Now(),
		)
		s.eventEmitter.OnEvent(compressionEvent)

		session.Messages = compressed
	}

	s.logger.Debug("Getting isolated LLM client: provider=%s, model=%s", s.config.LLMProvider, s.config.LLMModel)
	// Use GetIsolatedClient to ensure session-level cost tracking isolation
	llmClient, err := s.llmFactory.GetIsolatedClient(s.config.LLMProvider, s.config.LLMModel, ports.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM client: %w", err)
	}
	s.logger.Debug("Isolated LLM client obtained successfully")

	// Use Wrap instead of Attach to avoid modifying shared client state
	llmClient = s.costDecorator.Wrap(ctx, session.ID, llmClient)

	analysis := s.analysis.Analyze(ctx, task, llmClient)
	var analysisInfo *ports.TaskAnalysisInfo
	var taskAnalysis *ports.TaskAnalysis
	if analysis != nil && analysis.ActionName != "" {
		s.logger.Debug("Task pre-analysis: action=%s, goal=%s", analysis.ActionName, analysis.Goal)
		analysisInfo = &ports.TaskAnalysisInfo{
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

	systemPrompt := s.presetResolver.ResolveSystemPrompt(ctx, task, analysisInfo, s.config.AgentPreset)
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

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context) ports.ToolRegistry {
	// Handle subagent context filtering first
	registry := s.toolRegistry
	if isSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")
	}

	// Apply preset-based filtering
	return s.presetResolver.ResolveToolRegistry(ctx, registry, s.config.ToolPreset)
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
