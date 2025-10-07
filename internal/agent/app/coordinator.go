package app

import (
	"context"
	"fmt"
	"os"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
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
	logger       ports.Logger
	clock        ports.Clock

	prepService     *ExecutionPreparationService
	analysisService *TaskAnalysisService
	costDecorator   *CostTrackingDecorator
}

type Config struct {
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MaxTokens     int
	MaxIterations int
	Temperature   float64
	TopP          float64
	StopSequences []string
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
	opts ...CoordinatorOption,
) *AgentCoordinator {
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if len(config.StopSequences) > 0 {
		config.StopSequences = append([]string(nil), config.StopSequences...)
	}

	coordinator := &AgentCoordinator{
		llmFactory:   llmFactory,
		toolRegistry: toolRegistry,
		sessionStore: sessionStore,
		contextMgr:   contextMgr,
		parser:       parser,
		promptLoader: prompts.New(),
		costTracker:  costTracker,
		config:       config,
		logger:       utils.NewComponentLogger("Coordinator"),
		clock:        ports.SystemClock{},
	}

	for _, opt := range opts {
		opt(coordinator)
	}

	coordinator.analysisService = NewTaskAnalysisService(coordinator.logger)
	coordinator.costDecorator = NewCostTrackingDecorator(costTracker, coordinator.logger, coordinator.clock)
	coordinator.prepService = NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:    llmFactory,
		ToolRegistry:  toolRegistry,
		SessionStore:  sessionStore,
		ContextMgr:    contextMgr,
		Parser:        parser,
		PromptLoader:  coordinator.promptLoader,
		Config:        config,
		Logger:        coordinator.logger,
		Clock:         coordinator.clock,
		Analysis:      coordinator.analysisService,
		CostDecorator: coordinator.costDecorator,
	})

	return coordinator
}

// ExecuteTask executes a task with optional event listener for streaming output
func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
	listener ports.EventListener,
) (*ports.TaskResult, error) {
	c.logger.Info("ExecuteTask called: task='%s', sessionID='%s'", task, sessionID)

	// Prepare execution environment
	env, err := c.PrepareExecution(ctx, task, sessionID)
	if err != nil {
		return nil, err
	}

	// Create ReactEngine and configure listener
	c.logger.Info("Delegating to ReactEngine...")
	completionDefaults := domain.CompletionDefaults{}
	if c.config.Temperature > 0 {
		temp := c.config.Temperature
		completionDefaults.Temperature = &temp
	}
	if c.config.MaxTokens > 0 {
		maxTokens := c.config.MaxTokens
		completionDefaults.MaxTokens = &maxTokens
	}
	if c.config.TopP > 0 {
		topP := c.config.TopP
		completionDefaults.TopP = &topP
	}
	if len(c.config.StopSequences) > 0 {
		completionDefaults.StopSequences = append([]string(nil), c.config.StopSequences...)
	}

	reactEngine := domain.NewReactEngine(domain.ReactEngineConfig{
		MaxIterations:      c.config.MaxIterations,
		Logger:             c.logger,
		Clock:              c.clock,
		CompletionDefaults: completionDefaults,
	})

	if listener != nil {
		c.logger.Debug("Listener provided: type=%T", listener)
		reactEngine.SetEventListener(listener)
		c.logger.Info("Event listener successfully set on ReactEngine")
	} else {
		c.logger.Warn("No listener provided to ExecuteTask")
	}

	// If there's task analysis, emit the event before starting execution
	if env.TaskAnalysis != nil && env.TaskAnalysis.ActionName != "" && listener != nil {
		// Get agent level from context
		agentLevel := ports.GetOutputContext(ctx).Level

		event := domain.NewTaskAnalysisEvent(agentLevel, env.Session.ID, env.TaskAnalysis.ActionName, env.TaskAnalysis.Goal, c.clock.Now())
		listener.OnEvent(event)
	}

	result, err := reactEngine.SolveTask(ctx, task, env.State, env.Services)
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

	return &ports.TaskResult{
		Answer:     result.Answer,
		Messages:   result.Messages,
		Iterations: result.Iterations,
		TokensUsed: result.TokensUsed,
		StopReason: result.StopReason,
		SessionID:  env.Session.ID,
	}, nil
}

// PrepareExecution prepares the execution environment without running the task
func (c *AgentCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	return c.prepService.Prepare(ctx, task, sessionID)
}

// SaveSessionAfterExecution saves session state after task completion
func (c *AgentCoordinator) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result *ports.TaskResult) error {
	// Update session with results
	session.Messages = append([]ports.Message(nil), result.Messages...)
	session.UpdatedAt = c.clock.Now()

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

// GetConfig returns the coordinator configuration
func (c *AgentCoordinator) GetConfig() ports.AgentConfig {
	return ports.AgentConfig{
		LLMProvider:   c.config.LLMProvider,
		LLMModel:      c.config.LLMModel,
		MaxTokens:     c.config.MaxTokens,
		MaxIterations: c.config.MaxIterations,
		Temperature:   c.config.Temperature,
		TopP:          c.config.TopP,
		StopSequences: append([]string(nil), c.config.StopSequences...),
		AgentPreset:   c.config.AgentPreset,
		ToolPreset:    c.config.ToolPreset,
	}
}

// GetLLMClient returns an LLM client
func (c *AgentCoordinator) GetLLMClient() (ports.LLMClient, error) {
	client, err := c.llmFactory.GetClient(c.config.LLMProvider, c.config.LLMModel, llm.Config{
		APIKey:  c.config.APIKey,
		BaseURL: c.config.BaseURL,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetParser returns the function call parser
func (c *AgentCoordinator) GetParser() ports.FunctionCallParser {
	return c.parser
}

// GetContextManager returns the context manager
func (c *AgentCoordinator) GetContextManager() ports.ContextManager {
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
