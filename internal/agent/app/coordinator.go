package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
	"alex/internal/agent/types"
	"alex/internal/llm"
	"alex/internal/prompts"
	"alex/internal/utils"
)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
	llmFactory   *llm.Factory
	toolRegistry ports.ToolRegistry
	sessionStore ports.SessionStore
	contextMgr   ports.ContextManager
	parser       ports.FunctionCallParser
	promptLoader *prompts.Loader
	costTracker  ports.CostTracker
	config       Config
	logger       *utils.Logger
}

type Config struct {
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MaxTokens     int
	MaxIterations int
	AgentPreset   string // Agent persona preset (default, code-expert, etc.)
	ToolPreset    string // Tool access preset (full, read-only, etc.)
}

func NewAgentCoordinator(
	llmFactory *llm.Factory,
	toolRegistry ports.ToolRegistry,
	sessionStore ports.SessionStore,
	contextMgr ports.ContextManager,
	parser ports.FunctionCallParser,
	costTracker ports.CostTracker,
	config Config,
) *AgentCoordinator {
	return &AgentCoordinator{
		llmFactory:   llmFactory,
		toolRegistry: toolRegistry,
		sessionStore: sessionStore,
		contextMgr:   contextMgr,
		parser:       parser,
		promptLoader: prompts.New(),
		costTracker:  costTracker,
		config:       config,
		logger:       utils.NewComponentLogger("Coordinator"),
	}
}

// ExecuteTask executes a task with optional event listener for streaming output
func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
	listener any,
) (*ports.TaskResult, error) {
	c.logger.Info("ExecuteTask called: task='%s', sessionID='%s'", task, sessionID)

	// Prepare execution environment
	env, err := c.PrepareExecution(ctx, task, sessionID)
	if err != nil {
		return nil, err
	}

	// Create ReactEngine and configure listener
	c.logger.Info("Delegating to ReactEngine...")
	reactEngine := domain.NewReactEngine(c.config.MaxIterations)

	// Type assert listener to domain.EventListener if provided
	var domainListener domain.EventListener
	if listener != nil {
		c.logger.Debug("Listener provided: type=%T", listener)
		if dl, ok := listener.(domain.EventListener); ok {
			domainListener = dl
			reactEngine.SetEventListener(domainListener)
			c.logger.Info("Event listener successfully set on ReactEngine")
		} else {
			c.logger.Warn("Listener type assertion failed - listener is not domain.EventListener")
		}
	} else {
		c.logger.Warn("No listener provided to ExecuteTask")
	}

	// If there's task analysis, emit the event before starting execution
	if env.TaskAnalysis != nil && env.TaskAnalysis.ActionName != "" && domainListener != nil {
		// Get agent level from context
		agentLevel := types.LevelCore
		if outCtx := types.GetOutputContext(ctx); outCtx != nil {
			agentLevel = outCtx.Level
		}

		event := domain.NewTaskAnalysisEvent(agentLevel, env.Session.ID, env.TaskAnalysis.ActionName, env.TaskAnalysis.Goal)
		domainListener.OnEvent(event)
	}

	state := env.State.(*domain.TaskState)
	services := env.Services.(domain.Services)
	result, err := reactEngine.SolveTask(ctx, task, state, services)
	if err != nil {
		c.logger.Error("Task execution failed: %v", err)
		return nil, fmt.Errorf("task execution failed: %w", err)
	}
	c.logger.Info("Task execution completed: iterations=%d, tokens=%d, reason=%s",
		result.Iterations, result.TokensUsed, result.StopReason)

	// Save session
	if err := c.SaveSessionAfterExecution(ctx, env.Session, result); err != nil {
		return nil, err
	}

	// Convert domain.TaskResult to ports.TaskResult
	return &ports.TaskResult{
		Answer:     result.Answer,
		Iterations: result.Iterations,
		TokensUsed: result.TokensUsed,
		StopReason: result.StopReason,
		SessionID:  env.Session.ID,
	}, nil
}

// PrepareExecution prepares the execution environment without running the task
func (c *AgentCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	c.logger.Info("ExecuteTaskWithTUI called: task='%s', sessionID='%s'", task, sessionID)

	// 1. Load or create session
	c.logger.Debug("Loading session...")
	session, err := c.getSession(ctx, sessionID)
	if err != nil {
		c.logger.Error("Failed to get session: %v", err)
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	c.logger.Debug("Session loaded: id=%s, messages=%d", session.ID, len(session.Messages))

	// 2. Check context limits & compress if needed
	if c.contextMgr.ShouldCompress(session.Messages, c.config.MaxTokens) {
		c.logger.Info("Context limit reached, compressing...")
		compressed, err := c.contextMgr.Compress(session.Messages, c.config.MaxTokens*80/100)
		if err != nil {
			c.logger.Error("Failed to compress context: %v", err)
			return nil, fmt.Errorf("failed to compress context: %w", err)
		}
		c.logger.Info("Compression complete: %d -> %d messages", len(session.Messages), len(compressed))
		session.Messages = compressed
	}

	// 3. Get appropriate LLM client
	c.logger.Debug("Getting LLM client: provider=%s, model=%s", c.config.LLMProvider, c.config.LLMModel)
	llmClient, err := c.llmFactory.GetClient(c.config.LLMProvider, c.config.LLMModel, llm.Config{
		APIKey:  c.config.APIKey,
		BaseURL: c.config.BaseURL,
	})
	if err != nil {
		c.logger.Error("Failed to get LLM client: %v", err)
		return nil, fmt.Errorf("failed to get LLM client: %w", err)
	}
	c.logger.Debug("LLM client obtained successfully")

	// 3.5. Set up cost tracking callback if cost tracker is available
	if c.costTracker != nil {
		if trackingClient, ok := llmClient.(ports.UsageTrackingClient); ok {
			trackingClient.SetUsageCallback(func(usage ports.TokenUsage, model string, provider string) {
				record := ports.UsageRecord{
					SessionID:    sessionID,
					Model:        model,
					Provider:     provider,
					InputTokens:  usage.PromptTokens,
					OutputTokens: usage.CompletionTokens,
					TotalTokens:  usage.TotalTokens,
					Timestamp:    time.Now(),
				}

				// Calculate costs
				inputCost, outputCost, totalCost := ports.CalculateCost(
					usage.PromptTokens,
					usage.CompletionTokens,
					model,
				)
				record.InputCost = inputCost
				record.OutputCost = outputCost
				record.TotalCost = totalCost

				// Record usage (non-blocking, log errors only)
				if err := c.costTracker.RecordUsage(ctx, record); err != nil {
					c.logger.Warn("Failed to record cost: %v", err)
				}
			})
			c.logger.Debug("Cost tracking enabled for session: %s", sessionID)
		}
	}

	// 4. Pre-analyze task
	taskAnalysis := c.performTaskPreAnalysisStructured(ctx, task, llmClient)
	var analysisInfo *prompts.TaskAnalysisInfo
	if taskAnalysis != nil && taskAnalysis.ActionName != "" {
		c.logger.Debug("Task pre-analysis: action=%s, goal=%s", taskAnalysis.ActionName, taskAnalysis.Goal)
		analysisInfo = &prompts.TaskAnalysisInfo{
			Action:   taskAnalysis.ActionName,
			Goal:     taskAnalysis.Goal,
			Approach: taskAnalysis.Approach,
		}
	} else {
		c.logger.Debug("Task pre-analysis skipped or failed")
	}

	// 5. Generate system prompt based on agent preset
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	var systemPrompt string

	// Check for preset in context (takes priority over config)
	agentPreset := c.config.AgentPreset
	if presetCfg, ok := ctx.Value(PresetContextKey{}).(PresetConfig); ok && presetCfg.AgentPreset != "" {
		agentPreset = presetCfg.AgentPreset
		c.logger.Debug("Using agent preset from context: %s", agentPreset)
	}

	// Use preset system prompt if specified, otherwise use default prompt loader
	if agentPreset != "" && presets.IsValidPreset(agentPreset) {
		presetConfig, err := presets.GetPromptConfig(presets.AgentPreset(agentPreset))
		if err != nil {
			c.logger.Warn("Failed to load preset prompt: %v, using default", err)
			systemPrompt, _ = c.promptLoader.GetSystemPrompt(workingDir, task, analysisInfo)
		} else {
			c.logger.Info("Using preset system prompt: %s", presetConfig.Name)
			systemPrompt = presetConfig.SystemPrompt
		}
	} else {
		systemPrompt, err = c.promptLoader.GetSystemPrompt(workingDir, task, analysisInfo)
		if err != nil {
			c.logger.Warn("Failed to load system prompt: %v, using default", err)
			systemPrompt = "You are ALEX, a helpful AI coding assistant. Use available tools to help solve the user's task."
		}
	}
	c.logger.Debug("System prompt loaded: %d bytes", len(systemPrompt))

	// 5. Prepare task state from session
	state := &domain.TaskState{
		SystemPrompt: systemPrompt,
		Messages:     convertSessionMessages(session.Messages),
		SessionID:    session.ID,
	}
	c.logger.Debug("Task state prepared: %d messages, system_prompt=%d bytes", len(state.Messages), len(systemPrompt))

	// 6. Create services bundle for domain layer
	// Check if this is a subagent context - use filtered registry if so
	toolRegistry := c.toolRegistry
	if c.isSubagentContext(ctx) {
		toolRegistry = c.GetToolRegistryWithoutSubagent()
		c.logger.Debug("Using filtered registry (subagent excluded) for nested call")
	}

	// Check for tool preset in context (takes priority over config)
	toolPreset := c.config.ToolPreset
	if presetCfg, ok := ctx.Value(PresetContextKey{}).(PresetConfig); ok && presetCfg.ToolPreset != "" {
		toolPreset = presetCfg.ToolPreset
		c.logger.Debug("Using tool preset from context: %s", toolPreset)
	}

	// Apply tool preset filtering if specified
	if toolPreset != "" && presets.IsValidToolPreset(toolPreset) {
		filteredRegistry, err := presets.NewFilteredToolRegistry(toolRegistry, presets.ToolPreset(toolPreset))
		if err != nil {
			c.logger.Warn("Failed to create filtered registry: %v, using default", err)
		} else {
			toolRegistry = filteredRegistry
			toolConfig, _ := presets.GetToolConfig(presets.ToolPreset(toolPreset))
			c.logger.Info("Using tool preset: %s", toolConfig.Name)
		}
	}

	services := domain.Services{
		LLM:          llmClient,
		ToolExecutor: toolRegistry,
		Parser:       c.parser,
		Context:      c.contextMgr,
	}
	c.logger.Debug("Services bundle created")

	c.logger.Info("Execution environment prepared successfully")

	// Return everything needed for the caller to run the task
	var taskAnalysisInfo *ports.TaskAnalysis
	if taskAnalysis != nil && taskAnalysis.ActionName != "" {
		taskAnalysisInfo = &ports.TaskAnalysis{
			ActionName: taskAnalysis.ActionName,
			Goal:       taskAnalysis.Goal,
			Approach:   taskAnalysis.Approach,
		}
	}

	return &ports.ExecutionEnvironment{
		State:        state,
		Services:     services,
		Session:      session,
		SystemPrompt: systemPrompt,
		TaskAnalysis: taskAnalysisInfo,
	}, nil
}

// SaveSessionAfterExecution saves session state after task completion
func (c *AgentCoordinator) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result any) error {
	domainResult := result.(*domain.TaskResult)

	// Update session with results
	session.Messages = convertDomainMessages(domainResult.Messages)
	session.UpdatedAt = time.Now()

	c.logger.Debug("Saving session...")
	if err := c.sessionStore.Save(ctx, session); err != nil {
		c.logger.Error("Failed to save session: %v", err)
		return fmt.Errorf("failed to save session: %w", err)
	}
	c.logger.Debug("Session saved successfully")

	return nil
}

// GetSession retrieves or creates a session (public method)
func (c *AgentCoordinator) GetSession(ctx context.Context, id string) (*ports.Session, error) {
	return c.getSession(ctx, id)
}

func (c *AgentCoordinator) getSession(ctx context.Context, id string) (*ports.Session, error) {
	if id == "" {
		return c.sessionStore.Create(ctx)
	}
	return c.sessionStore.Get(ctx, id)
}

func convertSessionMessages(msgs []ports.Message) []domain.Message {
	result := make([]domain.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = domain.Message{
			Role:     msg.Role,
			Content:  msg.Content,
			Metadata: msg.Metadata,
		}
	}
	return result
}

func convertDomainMessages(msgs []domain.Message) []ports.Message {
	result := make([]ports.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = ports.Message{
			Role:     msg.Role,
			Content:  msg.Content,
			Metadata: msg.Metadata,
		}
	}
	return result
}

func (c *AgentCoordinator) ListSessions(ctx context.Context) ([]string, error) {
	return c.sessionStore.List(ctx)
}

// GetCostTracker returns the cost tracker instance
func (c *AgentCoordinator) GetCostTracker() ports.CostTracker {
	return c.costTracker
}

// GetToolRegistryWithoutSubagent returns a filtered registry that excludes subagent
// This is used by subagent tool to prevent nested subagent calls
func (c *AgentCoordinator) GetToolRegistryWithoutSubagent() ports.ToolRegistry {
	// Check if the registry implements WithoutSubagent method
	type registryWithFilter interface {
		WithoutSubagent() ports.ToolRegistry
	}

	if filtered, ok := c.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	// Fallback: return original registry if filtering not supported
	return c.toolRegistry
}

// Context key for subagent detection (must match builtin/subagent.go)
type subagentCtxKey struct{}

// Context key for preset configuration
type PresetContextKey struct{}

// PresetConfig holds preset configuration passed via context
type PresetConfig struct {
	AgentPreset string
	ToolPreset  string
}

// isSubagentContext checks if this is a nested subagent call
func (c *AgentCoordinator) isSubagentContext(ctx context.Context) bool {
	return ctx.Value(subagentCtxKey{}) != nil
}

// GetConfig returns the coordinator configuration
func (c *AgentCoordinator) GetConfig() any {
	return c.config
}

// GetLLMClient returns an LLM client
func (c *AgentCoordinator) GetLLMClient() (any, error) {
	return c.llmFactory.GetClient(c.config.LLMProvider, c.config.LLMModel, llm.Config{
		APIKey:  c.config.APIKey,
		BaseURL: c.config.BaseURL,
	})
}

// GetParser returns the function call parser
func (c *AgentCoordinator) GetParser() any {
	return c.parser
}

// GetContextManager returns the context manager
func (c *AgentCoordinator) GetContextManager() any {
	return c.contextMgr
}

// GetSystemPrompt returns the system prompt
func (c *AgentCoordinator) GetSystemPrompt() string {
	workingDir, _ := os.Getwd()
	if workingDir == "" {
		workingDir = "."
	}
	prompt, _ := c.promptLoader.GetSystemPrompt(workingDir, "", nil)
	return prompt
}

// performTaskPreAnalysis performs quick task analysis using LLM
// executeWithToolDisplay wraps ReactEngine execution with tool call display

// TaskAnalysis contains the structured result of task pre-analysis
type TaskAnalysis struct {
	ActionName  string // The overall action/operation name, e.g., "Analyzing codebase"
	Goal        string // What the task aims to achieve
	Approach    string // High-level approach or strategy
	RawAnalysis string // Full analysis text
}

// performTaskPreAnalysisStructured performs task analysis with structured output
func (c *AgentCoordinator) performTaskPreAnalysisStructured(
	ctx context.Context,
	task string,
	llmClient ports.LLMClient,
) *TaskAnalysis {
	c.logger.Debug("Starting task pre-analysis")

	// Enhanced analysis prompt with action naming
	analysisPrompt := fmt.Sprintf(`Analyze this task and provide a concise structured response:

Task: %s

Respond in this exact format:
Action: [single verb phrase, e.g., "Analyzing codebase", "Implementing feature", "Debugging issue"]
Goal: [what needs to be achieved]
Approach: [brief strategy]

Keep each line under 80 characters. Be specific and actionable.`, task)

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: analysisPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   150,
	}

	// Non-blocking timeout (increased to 5 seconds for more reliable analysis)
	analyzeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := llmClient.Complete(analyzeCtx, req)
	if err != nil {
		c.logger.Warn("Task pre-analysis failed: %v", err)
		return nil
	}

	if resp == nil {
		c.logger.Warn("Task pre-analysis: LLM returned nil response")
		return nil
	}

	if resp.Content == "" {
		c.logger.Warn("Task pre-analysis: LLM returned empty content")
		return nil
	}

	c.logger.Debug("Task pre-analysis LLM response: %s", resp.Content)

	// Parse structured response
	analysis := parseTaskAnalysis(resp.Content)
	c.logger.Debug("Task pre-analysis completed: action=%s, goal=%s", analysis.ActionName, analysis.Goal)
	return analysis
}

// parseTaskAnalysis extracts structured information from analysis response
func parseTaskAnalysis(content string) *TaskAnalysis {
	analysis := &TaskAnalysis{
		RawAnalysis: content,
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Action:") {
			analysis.ActionName = strings.TrimSpace(strings.TrimPrefix(line, "Action:"))
		} else if strings.HasPrefix(line, "Goal:") {
			analysis.Goal = strings.TrimSpace(strings.TrimPrefix(line, "Goal:"))
		} else if strings.HasPrefix(line, "Approach:") {
			analysis.Approach = strings.TrimSpace(strings.TrimPrefix(line, "Approach:"))
		}
	}

	// Fallback if parsing failed - use first line or generic message
	if analysis.ActionName == "" {
		// Try to extract first meaningful line as action
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && len(line) < 80 {
				analysis.ActionName = line
				break
			}
		}
		// Final fallback
		if analysis.ActionName == "" {
			analysis.ActionName = "Processing request"
		}
	}

	return analysis
}
