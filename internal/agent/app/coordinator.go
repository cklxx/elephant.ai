package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/llm"
	"alex/internal/prompts"
	"alex/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
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
}

func NewAgentCoordinator(
	llmFactory *llm.Factory,
	toolRegistry ports.ToolRegistry,
	sessionStore ports.SessionStore,
	contextMgr ports.ContextManager,
	parser ports.FunctionCallParser,
	messageQueue ports.MessageQueue,
	reactEngine *domain.ReactEngine,
	costTracker ports.CostTracker,
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
		costTracker:  costTracker,
		config:       config,
		logger:       utils.NewComponentLogger("Coordinator"),
	}
}

func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
) (*ports.TaskResult, error) {
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

	// 6.5. Show simple analyzing indicator
	dimStyle := "\033[90m"  // Gray
	resetStyle := "\033[0m" // Reset
	fmt.Printf("\n%s👾 Analyzing...%s\n\n", dimStyle, resetStyle)

	// 6.6. Ultra Think: Pre-analyze task (optional, non-blocking) - silent for user
	taskAnalysisStruct := c.performTaskPreAnalysisStructured(ctx, task, llmClient)
	if taskAnalysisStruct != nil {
		c.logger.Debug("Task pre-analysis: action=%s, goal=%s", taskAnalysisStruct.ActionName, taskAnalysisStruct.Goal)
		// Internal analysis - don't display to user
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

	// 8. Update session with results (result.Messages contains ALL messages including history)
	session.Messages = convertDomainMessages(result.Messages)
	session.UpdatedAt = time.Now()

	c.logger.Debug("Saving session...")
	if err := c.sessionStore.Save(ctx, session); err != nil {
		c.logger.Error("Failed to save session: %v", err)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	c.logger.Debug("Session saved successfully")

	// Convert domain.TaskResult to ports.TaskResult
	return &ports.TaskResult{
		Answer:     result.Answer,
		Iterations: result.Iterations,
		TokensUsed: result.TokensUsed,
		StopReason: result.StopReason,
	}, nil
}

// ExecuteTaskWithListener runs a task with a custom event listener
func (c *AgentCoordinator) ExecuteTaskWithListener(
	ctx context.Context,
	task string,
	sessionID string,
	listener domain.EventListener,
) (*domain.TaskResult, error) {
	return c.executeTaskWithListener(ctx, task, sessionID, listener)
}

// ExecuteTaskWithTUI runs a task with TUI event streaming
func (c *AgentCoordinator) ExecuteTaskWithTUI(
	ctx context.Context,
	task string,
	sessionID string,
	tuiProgram *tea.Program,
) (*domain.TaskResult, error) {
	bridge := NewEventBridge(tuiProgram)
	return c.executeTaskWithListener(ctx, task, sessionID, bridge)
}

// executeTaskWithListener is the internal implementation
func (c *AgentCoordinator) executeTaskWithListener(
	ctx context.Context,
	task string,
	sessionID string,
	listener domain.EventListener,
) (*domain.TaskResult, error) {
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
	// Check if this is a subagent context - use filtered registry if so
	toolRegistry := c.toolRegistry
	if c.isSubagentContext(ctx) {
		toolRegistry = c.GetToolRegistryWithoutSubagent()
		c.logger.Debug("Using filtered registry (subagent excluded) for nested call")
	}

	services := domain.Services{
		LLM:          llmClient,
		ToolExecutor: toolRegistry,
		Parser:       c.parser,
		Context:      c.contextMgr,
	}
	c.logger.Debug("Services bundle created")

	// 7. Set up event listener
	c.reactEngine.SetEventListener(listener)
	defer c.reactEngine.SetEventListener(nil) // Clear listener when done

	// 7.5. Show simple analyzing indicator
	dimStyle := "\033[90m"  // Gray
	resetStyle := "\033[0m" // Reset
	fmt.Printf("\n%s👾 Analyzing...%s\n\n", dimStyle, resetStyle)

	// 7.6. Pre-analyze task (silent for user, only for internal logging)
	taskAnalysisStruct := c.performTaskPreAnalysisStructured(ctx, task, llmClient)
	if taskAnalysisStruct != nil {
		c.logger.Debug("Task pre-analysis: action=%s, goal=%s", taskAnalysisStruct.ActionName, taskAnalysisStruct.Goal)
		// Internal analysis - don't display to user
	}

	// 8. Execute task (events will stream to TUI)
	c.logger.Info("Delegating to ReactEngine with TUI streaming...")
	result, err := c.reactEngine.SolveTask(ctx, task, state, services)
	if err != nil {
		c.logger.Error("Task execution failed: %v", err)
		return nil, fmt.Errorf("task execution failed: %w", err)
	}
	c.logger.Info("Task execution completed: iterations=%d, tokens=%d, reason=%s",
		result.Iterations, result.TokensUsed, result.StopReason)

	// 9. Update session with results
	session.Messages = convertDomainMessages(result.Messages)
	session.UpdatedAt = time.Now()

	c.logger.Debug("Saving session...")
	if err := c.sessionStore.Save(ctx, session); err != nil {
		c.logger.Error("Failed to save session: %v", err)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	c.logger.Debug("Session saved successfully")

	return result, nil
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

// isSubagentContext checks if this is a nested subagent call
func (c *AgentCoordinator) isSubagentContext(ctx context.Context) bool {
	return ctx.Value(subagentCtxKey{}) != nil
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

	// Display result preview (always show formatted summary)
	if err != nil || (result != nil && result.Error != nil) {
		formatted := t.formatter.FormatToolResult(call.Name, "", false)
		fmt.Printf("\033[90m%s\033[0m\n", formatted)
	} else if result != nil {
		formatted := t.formatter.FormatToolResult(call.Name, result.Content, true)
		fmt.Printf("\033[90m%s\033[0m\n", formatted)

		// For verbose mode, show full output for certain tools
		// Check environment variable ALEX_VERBOSE
		if os.Getenv("ALEX_VERBOSE") == "1" || os.Getenv("ALEX_VERBOSE") == "true" {
			// Show first 500 chars of actual result
			if len(result.Content) > 0 {
				preview := result.Content
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				fmt.Printf("\033[90m  Full output:\n%s\033[0m\n", preview)
			}
		}
	}

	return result, err
}

func (t *toolExecutorDisplay) Definition() ports.ToolDefinition {
	return t.inner.Definition()
}

func (t *toolExecutorDisplay) Metadata() ports.ToolMetadata {
	return t.inner.Metadata()
}

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

	// Non-blocking timeout
	analyzeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := llmClient.Complete(analyzeCtx, req)
	if err != nil {
		c.logger.Warn("Task pre-analysis failed: %v", err)
		return nil
	}

	if resp == nil || resp.Content == "" {
		return nil
	}

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

	// Fallback if parsing failed
	if analysis.ActionName == "" {
		analysis.ActionName = "Processing request"
	}

	return analysis
}
