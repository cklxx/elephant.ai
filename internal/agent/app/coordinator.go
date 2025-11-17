package app

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
	llmFactory   ports.LLMClientFactory
	toolRegistry ports.ToolRegistry
	sessionStore ports.SessionStore
	contextMgr   ports.ContextManager
	parser       ports.FunctionCallParser
	promptLoader ports.PromptLoader
	costTracker  ports.CostTracker
	config       Config
	logger       ports.Logger
	clock        ports.Clock

	prepService     *ExecutionPreparationService
	analysisService *TaskAnalysisService
	costDecorator   *CostTrackingDecorator
	ragGate         ports.RAGGate
	ragExecutor     *ragPreloader
	autoReviewer    *ResultAutoReviewer
}

type Config struct {
	LLMProvider         string
	LLMModel            string
	APIKey              string
	BaseURL             string
	MaxTokens           int
	MaxIterations       int
	Temperature         float64
	TemperatureProvided bool
	TopP                float64
	StopSequences       []string
	AgentPreset         string // Agent persona preset (default, code-expert, etc.)
	ToolPreset          string // Tool access preset (full, read-only, etc.)
	EnvironmentSummary  string
	AutoReview          *AutoReviewOptions
}

func NewAgentCoordinator(
	llmFactory ports.LLMClientFactory,
	toolRegistry ports.ToolRegistry,
	sessionStore ports.SessionStore,
	contextMgr ports.ContextManager,
	parser ports.FunctionCallParser,
	promptLoader ports.PromptLoader,
	costTracker ports.CostTracker,
	config Config,
	opts ...CoordinatorOption,
) *AgentCoordinator {
	if !config.TemperatureProvided {
		if config.Temperature != 0 {
			config.TemperatureProvided = true
		} else {
			config.Temperature = 0.7
		}
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
		promptLoader: promptLoader,
		costTracker:  costTracker,
		config:       config,
		logger:       utils.NewComponentLogger("Coordinator"),
		clock:        ports.SystemClock{},
		ragGate:      nil,
		ragExecutor:  nil,
	}

	for _, opt := range opts {
		opt(coordinator)
	}

	if coordinator.config.AutoReview == nil {
		coordinator.config.AutoReview = defaultAutoReviewOptions()
	}
	coordinator.SetAutoReviewOptions(coordinator.config.AutoReview)

	if coordinator.ragExecutor == nil {
		coordinator.ragExecutor = newRAGPreloader(coordinator.logger)
	}

	// Create services only if not provided via options
	if coordinator.analysisService == nil {
		coordinator.analysisService = NewTaskAnalysisService(coordinator.logger)
	}
	if coordinator.costDecorator == nil {
		coordinator.costDecorator = NewCostTrackingDecorator(costTracker, coordinator.logger, coordinator.clock)
	}

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
		CostTracker:   coordinator.costTracker,
		RAGGate:       coordinator.ragGate,
	})

	return coordinator
}

// SetAutoReviewOptions replaces the current auto-review configuration and refreshes
// the reviewer instance. Passing nil disables the reviewer entirely until a new
// configuration is applied.
func (c *AgentCoordinator) SetAutoReviewOptions(options *AutoReviewOptions) {
	if c == nil {
		return
	}
	if options == nil {
		c.config.AutoReview = nil
		c.autoReviewer = nil
		return
	}
	c.config.AutoReview = cloneAutoReviewOptions(options)
	if c.autoReviewer == nil {
		c.autoReviewer = NewResultAutoReviewer(c.config.AutoReview)
		return
	}
	c.autoReviewer.UpdateOptions(c.config.AutoReview)
}

// ExecuteTask executes a task with optional event listener for streaming output
func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
	listener ports.EventListener,
) (*ports.TaskResult, error) {
	ctx = id.WithSessionID(ctx, sessionID)
	ctx, ensuredTaskID := id.EnsureTaskID(ctx, id.NewTaskID)
	if ensuredTaskID == "" {
		ensuredTaskID = id.TaskIDFromContext(ctx)
	}
	parentTaskID := id.ParentTaskIDFromContext(ctx)
	c.logger.Info("ExecuteTask called: task='%s', sessionID='%s'", task, sessionID)

	// Prepare execution environment with event listener support
	env, err := c.prepareExecutionWithListener(ctx, task, sessionID, listener)
	if err != nil {
		return nil, err
	}

	ctx = id.WithSessionID(ctx, env.Session.ID)

	if c.ragExecutor != nil {
		if err := c.ragExecutor.apply(ctx, env); err != nil {
			c.logger.Warn("RAG preloading encountered issues: %v", err)
		}
	}

	// Create ReactEngine and configure listener
	reactEngine := c.newReactEngine()
	c.logger.Info("Delegating to ReactEngine...")

	if listener != nil {
		// DO NOT log listener objects to avoid leaking sensitive information.
		c.logger.Debug("Listener provided")
		reactEngine.SetEventListener(listener)
		c.logger.Info("Event listener successfully set on ReactEngine")
	} else {
		c.logger.Warn("No listener provided to ExecuteTask")
	}

	// If there's task analysis, emit the event before starting execution
	if env.TaskAnalysis != nil && env.TaskAnalysis.ActionName != "" && listener != nil {
		// Get agent level from context
		agentLevel := ports.GetOutputContext(ctx).Level

		event := domain.NewTaskAnalysisEvent(agentLevel, env.Session.ID, ensuredTaskID, parentTaskID, env.TaskAnalysis, c.clock.Now())
		listener.OnEvent(event)
	}

	result, err := reactEngine.SolveTask(ctx, task, env.State, env.Services)
	if err != nil {
		// Check if it's a context cancellation error
		if ctx.Err() != nil {
			c.logger.Info("Task execution cancelled: %v", ctx.Err())
			return nil, ctx.Err()
		}
		c.logger.Error("Task execution failed: %v", err)
		return nil, fmt.Errorf("task execution failed: %w", err)
	}
	c.logger.Info("Task execution completed: iterations=%d, tokens=%d, reason=%s",
		result.Iterations, result.TokensUsed, result.StopReason)

	// Log session-level cost/token metrics
	if c.costTracker != nil {
		sessionStats, err := c.costTracker.GetSessionStats(ctx, env.Session.ID)
		if err != nil {
			c.logger.Warn("Failed to get session stats: %v", err)
		} else {
			c.logger.Info("Session summary: requests=%d, total_tokens=%d (input=%d, output=%d), cost=$%.6f, duration=%v",
				sessionStats.RequestCount, sessionStats.TotalTokens,
				sessionStats.InputTokens, sessionStats.OutputTokens,
				sessionStats.TotalCost, sessionStats.Duration)
		}
	}

	finalEnv := env
	finalResult := result
	if c.autoReviewer != nil {
		reviewEnv, reviewedResult, reviewReport, reviewErr := c.processAutoReview(ctx, task, env, result, listener)
		if reviewErr != nil {
			c.logger.Warn("Auto review failed: %v", reviewErr)
		} else {
			finalEnv = reviewEnv
			finalResult = reviewedResult
			finalResult.Review = reviewReport
		}
	}

	// Save session
	if err := c.SaveSessionAfterExecution(ctx, finalEnv.Session, finalResult); err != nil {
		return nil, err
	}

	taskResultTaskID := finalResult.TaskID
	if taskResultTaskID == "" {
		taskResultTaskID = ensuredTaskID
	}
	parentResultID := finalResult.ParentTaskID
	if parentResultID == "" {
		parentResultID = parentTaskID
	}

	return &ports.TaskResult{
		Answer:       finalResult.Answer,
		Messages:     finalResult.Messages,
		Iterations:   finalResult.Iterations,
		TokensUsed:   finalResult.TokensUsed,
		StopReason:   finalResult.StopReason,
		SessionID:    finalEnv.Session.ID,
		TaskID:       taskResultTaskID,
		ParentTaskID: parentResultID,
		Review:       finalResult.Review,
	}, nil
}

func (c *AgentCoordinator) newReactEngine() *domain.ReactEngine {
	completionDefaults := buildCompletionDefaultsFromConfig(c.config)
	return domain.NewReactEngine(domain.ReactEngineConfig{
		MaxIterations:      c.config.MaxIterations,
		Logger:             c.logger,
		Clock:              c.clock,
		CompletionDefaults: completionDefaults,
	})
}

func (c *AgentCoordinator) processAutoReview(
	ctx context.Context,
	originalTask string,
	initialEnv *ports.ExecutionEnvironment,
	initialResult *ports.TaskResult,
	listener ports.EventListener,
) (*ports.ExecutionEnvironment, *ports.TaskResult, *ports.AutoReviewReport, error) {
	if c.autoReviewer == nil || c.config.AutoReview == nil || !c.config.AutoReview.Enabled {
		return initialEnv, initialResult, nil, nil
	}

	assessment := c.autoReviewer.Review(initialResult)
	report := &ports.AutoReviewReport{Assessment: assessment}
	if !assessment.NeedsRework || !c.config.AutoReview.EnableAutoRework || c.config.AutoReview.MaxReworkAttempts <= 0 {
		c.emitAutoReviewEvent(ctx, initialEnv, initialResult, listener, report)
		return initialEnv, initialResult, report, nil
	}

	sessionID := ""
	if initialEnv != nil && initialEnv.Session != nil {
		sessionID = initialEnv.Session.ID
	}
	bestEnv := initialEnv
	bestResult := initialResult
	bestAssessment := assessment
	summary := &ports.ReworkSummary{}

	for attempt := 0; attempt < c.config.AutoReview.MaxReworkAttempts; attempt++ {
		summary.Attempted++
		reworkTask := buildReworkPrompt(originalTask, bestResult.Answer, bestAssessment, attempt)
		reworkEnv, err := c.prepareExecutionWithListener(ctx, reworkTask, sessionID, listener)
		if err != nil {
			note := fmt.Sprintf("attempt %d: failed to prepare environment (%v)", attempt+1, err)
			summary.Notes = append(summary.Notes, note)
			c.logger.Warn("Auto rework prepare failed: %v", err)
			continue
		}
		reworkEngine := c.newReactEngine()
		if listener != nil {
			reworkEngine.SetEventListener(listener)
		}
		reworkResult, err := reworkEngine.SolveTask(ctx, reworkTask, reworkEnv.State, reworkEnv.Services)
		if err != nil {
			note := fmt.Sprintf("attempt %d: execution failed (%v)", attempt+1, err)
			summary.Notes = append(summary.Notes, note)
			c.logger.Warn("Auto rework execution failed: %v", err)
			continue
		}
		attemptAssessment := c.autoReviewer.Review(reworkResult)
		summary.Notes = append(summary.Notes, fmt.Sprintf(
			"attempt %d: grade %s (%.2f)",
			attempt+1,
			attemptAssessment.Grade,
			attemptAssessment.Score,
		))
		if attemptAssessment.Score > bestAssessment.Score {
			bestAssessment = attemptAssessment
			bestResult = reworkResult
			bestEnv = reworkEnv
		}
		if attemptAssessment.Score >= c.config.AutoReview.MinPassingScore {
			summary.Applied = bestResult == reworkResult
			summary.FinalGrade = attemptAssessment.Grade
			summary.FinalScore = attemptAssessment.Score
			break
		}
	}

	if summary.Attempted > 0 {
		if summary.FinalGrade == "" {
			summary.FinalGrade = bestAssessment.Grade
			summary.FinalScore = bestAssessment.Score
			summary.Applied = bestResult != initialResult
		}
		report.Rework = summary
	}
	report.Assessment = bestAssessment
	c.emitAutoReviewEvent(ctx, bestEnv, bestResult, listener, report)
	return bestEnv, bestResult, report, nil
}

func (c *AgentCoordinator) emitAutoReviewEvent(
	ctx context.Context,
	env *ports.ExecutionEnvironment,
	result *ports.TaskResult,
	listener ports.EventListener,
	report *ports.AutoReviewReport,
) {
	if listener == nil || report == nil {
		return
	}
	outCtx := ports.GetOutputContext(ctx)
	sessionID := ""
	if env != nil && env.Session != nil {
		sessionID = env.Session.ID
	}
	taskID := ""
	parentTaskID := ""
	if result != nil {
		if result.TaskID != "" {
			taskID = result.TaskID
		}
		parentTaskID = result.ParentTaskID
	}
	event := domain.NewAutoReviewEvent(
		outCtx.Level,
		sessionID,
		taskID,
		parentTaskID,
		report,
		c.clock.Now(),
	)
	listener.OnEvent(event)
}

func buildCompletionDefaultsFromConfig(cfg Config) domain.CompletionDefaults {
	defaults := domain.CompletionDefaults{}

	if cfg.TemperatureProvided {
		temp := cfg.Temperature
		defaults.Temperature = &temp
	}
	if cfg.MaxTokens > 0 {
		maxTokens := cfg.MaxTokens
		defaults.MaxTokens = &maxTokens
	}
	if cfg.TopP > 0 {
		topP := cfg.TopP
		defaults.TopP = &topP
	}
	if len(cfg.StopSequences) > 0 {
		defaults.StopSequences = append([]string(nil), cfg.StopSequences...)
	}

	return defaults
}

// PrepareExecution prepares the execution environment without running the task
func (c *AgentCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	return c.prepService.Prepare(ctx, task, sessionID)
}

// prepareExecutionWithListener prepares execution with event emission support
func (c *AgentCoordinator) prepareExecutionWithListener(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.ExecutionEnvironment, error) {
	// Create a preparation service instance with the listener for this execution
	if listener != nil {
		prepService := NewExecutionPreparationService(ExecutionPreparationDeps{
			LLMFactory:    c.llmFactory,
			ToolRegistry:  c.toolRegistry,
			SessionStore:  c.sessionStore,
			ContextMgr:    c.contextMgr,
			Parser:        c.parser,
			PromptLoader:  c.promptLoader,
			Config:        c.config,
			Logger:        c.logger,
			Clock:         c.clock,
			Analysis:      c.analysisService,
			CostDecorator: c.costDecorator,
			EventEmitter:  listener, // Pass the listener for event emission
			CostTracker:   c.costTracker,
			RAGGate:       c.ragGate,
		})
		return prepService.Prepare(ctx, task, sessionID)
	}
	// No listener, use default prep service
	return c.prepService.Prepare(ctx, task, sessionID)
}

// SaveSessionAfterExecution saves session state after task completion
func (c *AgentCoordinator) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result *ports.TaskResult) error {
	// Update session with results
	sanitizedMessages, attachmentStore := sanitizeMessagesForPersistence(result.Messages)
	session.Messages = sanitizedMessages
	if len(attachmentStore) > 0 {
		session.Attachments = attachmentStore
	} else {
		session.Attachments = nil
	}
	session.UpdatedAt = c.clock.Now()

	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	if result.SessionID != "" {
		session.Metadata["session_id"] = result.SessionID
	}
	if result.TaskID != "" {
		session.Metadata["last_task_id"] = result.TaskID
	}
	if result.ParentTaskID != "" {
		session.Metadata["last_parent_task_id"] = result.ParentTaskID
	} else {
		delete(session.Metadata, "last_parent_task_id")
	}

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

// GetToolRegistry returns the tool registry instance
func (c *AgentCoordinator) GetToolRegistry() ports.ToolRegistry {
	return c.toolRegistry
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

// SetEnvironmentSummary updates the environment context appended to system prompts.
func (c *AgentCoordinator) SetEnvironmentSummary(summary string) {
	c.config.EnvironmentSummary = summary
	if c.prepService != nil {
		c.prepService.SetEnvironmentSummary(summary)
	}
}

func sanitizeMessagesForPersistence(messages []ports.Message) ([]ports.Message, map[string]ports.Attachment) {
	if len(messages) == 0 {
		return nil, nil
	}

	sanitized := make([]ports.Message, 0, len(messages))
	attachments := make(map[string]ports.Attachment)

	for _, msg := range messages {
		if msg.Source == ports.MessageSourceUserHistory {
			continue
		}

		cloned := msg
		if len(msg.Attachments) > 0 {
			for key, att := range msg.Attachments {
				name := strings.TrimSpace(key)
				if name == "" {
					name = strings.TrimSpace(att.Name)
				}
				if name == "" {
					continue
				}
				if att.Name == "" {
					att.Name = name
				}
				attachments[name] = att
			}
			cloned.Attachments = nil
		}
		sanitized = append(sanitized, cloned)
	}

	if len(sanitized) == 0 {
		return nil, nil
	}

	if len(attachments) == 0 {
		return sanitized, nil
	}
	return sanitized, attachments
}

// GetLLMClient returns an LLM client
func (c *AgentCoordinator) GetLLMClient() (ports.LLMClient, error) {
	client, err := c.llmFactory.GetClient(c.config.LLMProvider, c.config.LLMModel, ports.LLMConfig{
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
	prompt, _ := c.promptLoader.GetSystemPrompt("", nil)
	return prompt
}

// performTaskPreAnalysis performs quick task analysis using LLM
// executeWithToolDisplay wraps ReactEngine execution with tool call display
