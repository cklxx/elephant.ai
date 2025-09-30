package app

import (
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/llm"
	"alex/internal/prompts"
	"alex/internal/utils"
	"context"
	"fmt"
	"os"
	"time"
)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
	llmFactory   *llm.Factory
	toolRegistry ports.ToolRegistry
	sessionStore ports.SessionStore
	contextMgr   ports.ContextManager
	parser       ports.FunctionCallParser
	messageQueue ports.MessageQueue
	reactEngine  *domain.ReactEngine
	promptLoader *prompts.Loader
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
}

func NewAgentCoordinator(
	llmFactory *llm.Factory,
	toolRegistry ports.ToolRegistry,
	sessionStore ports.SessionStore,
	contextMgr ports.ContextManager,
	parser ports.FunctionCallParser,
	messageQueue ports.MessageQueue,
	reactEngine *domain.ReactEngine,
	config Config,
) *AgentCoordinator {
	return &AgentCoordinator{
		llmFactory:   llmFactory,
		toolRegistry: toolRegistry,
		sessionStore: sessionStore,
		contextMgr:   contextMgr,
		parser:       parser,
		messageQueue: messageQueue,
		reactEngine:  reactEngine,
		promptLoader: prompts.New(),
		config:       config,
		logger:       utils.NewComponentLogger("Coordinator"),
	}
}

func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
) (*domain.TaskResult, error) {
	c.logger.Info("ExecuteTask called: task='%s', sessionID='%s'", task, sessionID)

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

	// 4. Generate system prompt
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}
	systemPrompt, err := c.promptLoader.GetSystemPrompt(workingDir, task)
	if err != nil {
		c.logger.Warn("Failed to load system prompt: %v, using default", err)
		systemPrompt = "You are ALEX, a helpful AI coding assistant. Use available tools to help solve the user's task."
	}
	c.logger.Debug("System prompt loaded: %d bytes", len(systemPrompt))

	// 5. Prepare task state from session
	state := &domain.TaskState{
		SystemPrompt: systemPrompt,
		Messages:     convertSessionMessages(session.Messages),
	}
	c.logger.Debug("Task state prepared: %d messages, system_prompt=%d bytes", len(state.Messages), len(systemPrompt))

	// 6. Create services bundle for domain layer
	services := domain.Services{
		LLM:          llmClient,
		ToolExecutor: c.toolRegistry,
		Parser:       c.parser,
		Context:      c.contextMgr,
	}
	c.logger.Debug("Services bundle created")

	// 6.5. Ultra Think: Pre-analyze task (optional, non-blocking)
	taskAnalysis := c.performTaskPreAnalysis(ctx, task, llmClient)
	if taskAnalysis != "" {
		c.logger.Debug("Task pre-analysis: %s", taskAnalysis)
		fmt.Printf("\nAnalysis: %s\n\n", taskAnalysis)
	}

	// 7. Delegate to domain logic with tool display
	c.logger.Info("Delegating to ReactEngine...")

	// Create a callback-enabled execution wrapper
	result, err := c.executeWithToolDisplay(ctx, task, state, services)
	if err != nil {
		c.logger.Error("Task execution failed: %v", err)
		return nil, fmt.Errorf("task execution failed: %w", err)
	}
	c.logger.Info("Task execution completed: iterations=%d, tokens=%d, reason=%s",
		result.Iterations, result.TokensUsed, result.StopReason)

	// 8. Update session with results
	session.Messages = append(session.Messages, convertDomainMessages(result.Messages)...)
	session.UpdatedAt = time.Now()

	c.logger.Debug("Saving session...")
	if err := c.sessionStore.Save(ctx, session); err != nil {
		c.logger.Error("Failed to save session: %v", err)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	c.logger.Debug("Session saved successfully")

	return result, nil
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

// performTaskPreAnalysis performs quick task analysis using LLM
// executeWithToolDisplay wraps ReactEngine execution with tool call display
func (c *AgentCoordinator) executeWithToolDisplay(
	ctx context.Context,
	task string,
	state *domain.TaskState,
	services domain.Services,
) (*domain.TaskResult, error) {
	// Create a tool-intercepting registry wrapper
	originalRegistry := services.ToolExecutor
	displayRegistry := &toolDisplayWrapper{
		inner:     originalRegistry,
		formatter: domain.NewToolFormatter(),
	}
	services.ToolExecutor = displayRegistry

	// Execute with display wrapper
	return c.reactEngine.SolveTask(ctx, task, state, services)
}

// toolDisplayWrapper intercepts tool execution to display calls
type toolDisplayWrapper struct {
	inner     ports.ToolRegistry
	formatter *domain.ToolFormatter
}

func (w *toolDisplayWrapper) Get(name string) (ports.ToolExecutor, error) {
	tool, err := w.inner.Get(name)
	if err != nil {
		return nil, err
	}

	// Wrap tool with display logic
	return &toolExecutorDisplay{
		inner:     tool,
		formatter: w.formatter,
		name:      name,
	}, nil
}

func (w *toolDisplayWrapper) List() []ports.ToolDefinition {
	return w.inner.List()
}

func (w *toolDisplayWrapper) Register(tool ports.ToolExecutor) error {
	return w.inner.Register(tool)
}

func (w *toolDisplayWrapper) Unregister(name string) error {
	return w.inner.Unregister(name)
}

// toolExecutorDisplay wraps individual tool execution with display
type toolExecutorDisplay struct {
	inner     ports.ToolExecutor
	formatter *domain.ToolFormatter
	name      string
}

func (t *toolExecutorDisplay) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Display tool call
	fmt.Println(t.formatter.FormatToolCall(call.Name, call.Arguments))

	// Execute actual tool
	result, err := t.inner.Execute(ctx, call)

	// Display result preview
	if err != nil || (result != nil && result.Error != nil) {
		formatted := t.formatter.FormatToolResult(call.Name, "", false)
		fmt.Printf("\033[90m%s\033[0m\n", formatted)
	} else if result != nil {
		formatted := t.formatter.FormatToolResult(call.Name, result.Content, true)
		fmt.Printf("\033[90m%s\033[0m\n", formatted)
	}

	return result, err
}

func (t *toolExecutorDisplay) Definition() ports.ToolDefinition {
	return t.inner.Definition()
}

func (t *toolExecutorDisplay) Metadata() ports.ToolMetadata {
	return t.inner.Metadata()
}

func (c *AgentCoordinator) performTaskPreAnalysis(
	ctx context.Context,
	task string,
	llmClient ports.LLMClient,
) string {
	c.logger.Debug("Starting task pre-analysis")

	// Quick analysis prompt
	analysisPrompt := fmt.Sprintf(`Quick 1-line task analysis:
Task: %s

Reply format: "Goal: [what] | Needs: [tools/files]" (max 60 chars)`, task)

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: analysisPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   50,
	}

	// Non-blocking timeout
	analyzeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := llmClient.Complete(analyzeCtx, req)
	if err != nil {
		c.logger.Warn("Task pre-analysis failed: %v", err)
		return ""
	}

	if resp == nil || resp.Content == "" {
		return ""
	}

	c.logger.Debug("Task pre-analysis completed: %s", resp.Content)
	return resp.Content
}
