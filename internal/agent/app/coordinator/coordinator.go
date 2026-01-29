package coordinator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/app/cost"
	"alex/internal/agent/app/hooks"
	"alex/internal/agent/app/preparation"
	sessiontitle "alex/internal/agent/app/sessiontitle"
	"alex/internal/agent/domain"
	react "alex/internal/agent/domain/react"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/agent/presets"
	"alex/internal/async"
	"alex/internal/logging"
	materialports "alex/internal/materials/ports"
	"alex/internal/utils/clilatency"
	id "alex/internal/utils/id"
)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
	llmFactory   llm.LLMClientFactory
	toolRegistry tools.ToolRegistry
	sessionStore storage.SessionStore
	contextMgr   agent.ContextManager
	historyMgr   storage.HistoryManager
	parser       tools.FunctionCallParser
	costTracker  storage.CostTracker
	config       appconfig.Config
	logger       agent.Logger
	clock        agent.Clock

	prepService        preparationService
	costDecorator      *cost.CostTrackingDecorator
	attachmentMigrator materialports.Migrator
	hookRegistry       *hooks.Registry
}

type preparationService interface {
	Prepare(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error)
	SetEnvironmentSummary(summary string)
	ResolveAgentPreset(ctx context.Context, preset string) string
	ResolveToolPreset(ctx context.Context, preset string) string
}

func NewAgentCoordinator(
	llmFactory llm.LLMClientFactory,
	toolRegistry tools.ToolRegistry,
	sessionStore storage.SessionStore,
	contextMgr agent.ContextManager,
	historyManager storage.HistoryManager,
	parser tools.FunctionCallParser,
	costTracker storage.CostTracker,
	config appconfig.Config,
	opts ...CoordinatorOption,
) *AgentCoordinator {
	if len(config.StopSequences) > 0 {
		config.StopSequences = append([]string(nil), config.StopSequences...)
	}

	coordinator := &AgentCoordinator{
		llmFactory:   llmFactory,
		toolRegistry: toolRegistry,
		sessionStore: sessionStore,
		contextMgr:   contextMgr,
		historyMgr:   historyManager,
		parser:       parser,
		costTracker:  costTracker,
		config:       config,
		logger:       logging.NewComponentLogger("Coordinator"),
		clock:        agent.SystemClock{},
	}

	for _, opt := range opts {
		opt(coordinator)
	}

	// Create services only if not provided via options
	if coordinator.costDecorator == nil {
		coordinator.costDecorator = cost.NewCostTrackingDecorator(costTracker, coordinator.logger, coordinator.clock)
	}

	coordinator.prepService = preparation.NewExecutionPreparationService(preparation.ExecutionPreparationDeps{
		LLMFactory:    llmFactory,
		ToolRegistry:  toolRegistry,
		SessionStore:  sessionStore,
		ContextMgr:    contextMgr,
		HistoryMgr:    historyManager,
		Parser:        parser,
		Config:        config,
		Logger:        coordinator.logger,
		Clock:         coordinator.clock,
		CostDecorator: coordinator.costDecorator,
		CostTracker:   coordinator.costTracker,
	})

	if coordinator.contextMgr != nil {
		if err := coordinator.contextMgr.Preload(context.Background()); err != nil {
			coordinator.logger.Warn("Context preload failed: %v", err)
		}
	}

	return coordinator
}

type planSessionTitleRecorder struct {
	mu      sync.Mutex
	title   string
	sink    agent.EventListener
	onTitle func(string)
}

func (r *planSessionTitleRecorder) OnEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}

	if tc, ok := event.(*domain.WorkflowToolCompletedEvent); ok {
		if tc.Error == nil && strings.EqualFold(strings.TrimSpace(tc.ToolName), "plan") {
			if title := extractPlanSessionTitle(tc.Metadata); title != "" {
				shouldNotify := false
				r.mu.Lock()
				if r.title == "" {
					r.title = title
					shouldNotify = true
				}
				r.mu.Unlock()
				if shouldNotify && r.onTitle != nil {
					r.onTitle(title)
				}
			}
		}
	}

	if r.sink != nil {
		r.sink.OnEvent(event)
	}
}

func (r *planSessionTitleRecorder) Title() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.title
}

func extractPlanSessionTitle(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}

	if raw, ok := metadata["session_title"].(string); ok {
		if title := sessiontitle.NormalizeSessionTitle(raw); title != "" {
			return title
		}
	}

	if raw, ok := metadata["overall_goal_ui"].(string); ok {
		return sessiontitle.NormalizeSessionTitle(raw)
	}

	return ""
}

func (c *AgentCoordinator) persistSessionTitle(ctx context.Context, sessionID string, title string) {
	if c == nil || c.sessionStore == nil {
		return
	}
	title = sessiontitle.NormalizeSessionTitle(title)
	if strings.TrimSpace(sessionID) == "" || title == "" {
		return
	}

	logger := c.loggerFor(ctx)
	async.Go(logger, "session-title-update", func() {
		updateCtx := context.Background()
		if logID := id.LogIDFromContext(ctx); logID != "" {
			updateCtx = id.WithLogID(updateCtx, logID)
		}
		updateCtx, cancel := context.WithTimeout(updateCtx, 2*time.Second)
		defer cancel()

		session, err := c.sessionStore.Get(updateCtx, sessionID)
		if err != nil {
			logger.Warn("Failed to load session for title update: %v", err)
			return
		}
		if session.Metadata == nil {
			session.Metadata = make(map[string]string)
		}
		if strings.TrimSpace(session.Metadata["title"]) != "" {
			return
		}
		session.Metadata["title"] = title
		if err := c.sessionStore.Save(updateCtx, session); err != nil {
			logger.Warn("Failed to persist session title: %v", err)
		}
	})
}

// ExecuteTask executes a task with optional event listener for streaming output
func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
	listener agent.EventListener,
) (*agent.TaskResult, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := c.loggerFor(ctx)
	prepareStarted := time.Now()
	// Decorate the listener with the workflow envelope translator so downstream
	// consumers receive workflow event envelopes.
	eventListener := wrapWithWorkflowEnvelope(listener, nil)
	var planTitleRecorder *planSessionTitleRecorder
	if eventListener != nil && !appcontext.IsSubagentContext(ctx) {
		planTitleRecorder = &planSessionTitleRecorder{
			sink: eventListener,
			onTitle: func(title string) {
				c.persistSessionTitle(ctx, sessionID, title)
			},
		}
		eventListener = planTitleRecorder
	}

	ctx = id.WithSessionID(ctx, sessionID)
	ctx, ensuredRunID := id.EnsureRunID(ctx, id.NewRunID)
	if ensuredRunID == "" {
		ensuredRunID = id.RunIDFromContext(ctx)
	}
	parentRunID := id.ParentRunIDFromContext(ctx)

	// Core run: set correlation_id = own run_id as root of the causal chain.
	// Subagent runs inherit this via subagent.executeSubtask.
	if id.CorrelationIDFromContext(ctx) == "" && ensuredRunID != "" {
		ctx = id.WithCorrelationID(ctx, ensuredRunID)
	}
	outCtx := agent.GetOutputContext(ctx)
	if outCtx == nil {
		outCtx = &agent.OutputContext{Level: agent.LevelCore}
	} else {
		cloned := *outCtx
		outCtx = &cloned
	}
	ids := id.IDsFromContext(ctx)
	if outCtx.SessionID == "" {
		outCtx.SessionID = ids.SessionID
	}
	if outCtx.TaskID == "" {
		outCtx.TaskID = ids.RunID
	}
	if outCtx.ParentTaskID == "" {
		outCtx.ParentTaskID = ids.ParentRunID
	}
	if outCtx.LogID == "" {
		outCtx.LogID = ids.LogID
	}
	outCtx.TaskID = ensuredRunID
	ctx = agent.WithOutputContext(ctx, outCtx)
	logger.Info("ExecuteTask called: task='%s', session='%s'", task, obfuscateSessionID(sessionID))

	wf := newAgentWorkflow(ensuredRunID, slog.Default(), eventListener, outCtx)
	wf.start(stagePrepare)

	attachWorkflow := func(result *agent.TaskResult, env *agent.ExecutionEnvironment) *agent.TaskResult {
		session := sessionID
		if env != nil && env.Session != nil && env.Session.ID != "" {
			session = env.Session.ID
		}
		return attachWorkflowSnapshot(result, wf, session, ensuredRunID, parentRunID)
	}

	// Prepare execution environment with event listener support
	env, err := c.prepareExecutionWithListener(ctx, task, sessionID, eventListener)
	if err != nil {
		wf.fail(stagePrepare, err)
		return attachWorkflow(nil, env), err
	}
	if sessionID == "" {
		sessionID = env.Session.ID
	}
	clilatency.PrintfWithContext(ctx,
		"[latency] prepare_ms=%.2f session=%s\n",
		float64(time.Since(prepareStarted))/float64(time.Millisecond),
		env.Session.ID,
	)
	outCtx.SessionID = env.Session.ID
	outCtx.TaskID = ensuredRunID
	outCtx.ParentTaskID = parentRunID
	wf.setContext(outCtx)
	prepareOutput := map[string]any{
		"session": env.Session.ID,
		"task":    task,
	}
	if env.TaskAnalysis != nil {
		if env.TaskAnalysis.ActionName != "" {
			prepareOutput["action_name"] = env.TaskAnalysis.ActionName
		}
		if env.TaskAnalysis.Complexity != "" {
			prepareOutput["complexity"] = env.TaskAnalysis.Complexity
		}
		if env.TaskAnalysis.Goal != "" {
			prepareOutput["goal"] = env.TaskAnalysis.Goal
		}
		if env.TaskAnalysis.Approach != "" {
			prepareOutput["approach"] = env.TaskAnalysis.Approach
		}
	}
	wf.succeed(stagePrepare, prepareOutput)
	ctx = id.WithSessionID(ctx, env.Session.ID)

	// Run proactive hooks (pre-task memory recall, etc.)
	if c.hookRegistry != nil && !appcontext.IsSubagentContext(ctx) {
		hookTask := hooks.TaskInfo{
			TaskInput: task,
			SessionID: env.Session.ID,
			RunID:     ensuredRunID,
			UserID:    c.resolveUserID(env.Session),
		}
		injections := c.hookRegistry.RunOnTaskStart(ctx, hookTask)
		if len(injections) > 0 {
			proactiveContext := hooks.FormatInjectionsAsContext(injections)
			if proactiveContext != "" {
				env.State.Messages = append(env.State.Messages, ports.Message{
					Role:    "user",
					Content: proactiveContext,
					Source:  ports.MessageSourceProactive,
				})
				logger.Info("Injected %d proactive context items", len(injections))
			}
		}
	}

	// Create ReactEngine and configure listener
	logger.Info("Delegating to ReactEngine...")
	completionDefaults := buildCompletionDefaultsFromConfig(c.config)

	reactEngine := react.NewReactEngine(react.ReactEngineConfig{
		MaxIterations:      c.config.MaxIterations,
		Logger:             logger,
		Clock:              c.clock,
		CompletionDefaults: completionDefaults,
		AttachmentMigrator: c.attachmentMigrator,
		Workflow:           wf,
		BackgroundExecutor: func(bgCtx context.Context, prompt, sessionID string,
			listener agent.EventListener) (*agent.TaskResult, error) {
			bgCtx = appcontext.MarkSubagentContext(bgCtx)
			return c.ExecuteTask(bgCtx, prompt, sessionID, listener)
		},
	})

	if eventListener != nil {
		// DO NOT log listener objects to avoid leaking sensitive information.
		logger.Debug("Listener provided")
		reactEngine.SetEventListener(eventListener)
		logger.Info("Event listener successfully set on ReactEngine")
	} else {
		logger.Warn("No listener provided to ExecuteTask")
	}

	wf.start(stageExecute)
	result, executionErr := reactEngine.SolveTask(ctx, task, env.State, env.Services)
	if result == nil {
		result = &agent.TaskResult{
			Answer:      "",
			Messages:    env.State.Messages,
			Iterations:  env.State.Iterations,
			TokensUsed:  env.State.TokenCount,
			StopReason:  "error",
			SessionID:   env.State.SessionID,
			RunID:       env.State.RunID,
			ParentRunID: env.State.ParentRunID,
		}
	}

	if ctx.Err() != nil {
		logger.Info("Task execution cancelled: %v", ctx.Err())
		c.persistSessionSnapshot(ctx, env, ensuredRunID, parentRunID, "cancelled")
		return attachWorkflow(result, env), ctx.Err()
	}

	if executionErr != nil {
		wf.fail(stageExecute, executionErr)
	} else {
		wf.succeed(stageExecute, map[string]any{
			"iterations": result.Iterations,
			"stop":       result.StopReason,
		})
	}

	wf.start(stageSummarize)
	answerPreview := strings.TrimSpace(result.Answer)
	if executionErr != nil {
		answerPreview = ""
	}
	wf.succeed(stageSummarize, map[string]any{"answer_preview": answerPreview})

	if executionErr != nil {
		logger.Error("Task execution failed: %v", executionErr)
	}

	// Log session-level cost/token metrics
	if c.costTracker != nil {
		sessionStats, err := c.costTracker.GetSessionStats(ctx, env.Session.ID)
		if err != nil {
			logger.Warn("Failed to get session stats: %v", err)
		} else {
			logger.Info("Session summary: requests=%d, total_tokens=%d (input=%d, output=%d), cost=$%.6f, duration=%v",
				sessionStats.RequestCount, sessionStats.TotalTokens,
				sessionStats.InputTokens, sessionStats.OutputTokens,
				sessionStats.TotalCost, sessionStats.Duration)
		}
	}

	// Run proactive hooks (post-task memory capture, etc.)
	if c.hookRegistry != nil && !appcontext.IsSubagentContext(ctx) && executionErr == nil {
		hookResult := hooks.TaskResultInfo{
			TaskInput:  task,
			Answer:     result.Answer,
			SessionID:  env.Session.ID,
			RunID:      ensuredRunID,
			UserID:     c.resolveUserID(env.Session),
			Iterations: result.Iterations,
			StopReason: result.StopReason,
			ToolCalls:  extractToolCallInfo(result),
		}
		c.hookRegistry.RunOnTaskCompleted(ctx, hookResult)
	}

	// Save session unless this is a delegated subagent run (which should not
	// mutate the parent session state).
	wf.start(stagePersist)
	if appcontext.IsSubagentContext(ctx) {
		logger.Debug("Skipping session persistence for subagent execution")
		wf.succeed(stagePersist, "skipped (subagent context)")
	} else {
		if planTitleRecorder != nil {
			if title := strings.TrimSpace(planTitleRecorder.Title()); title != "" {
				if env.Session.Metadata == nil {
					env.Session.Metadata = make(map[string]string)
				}
				if strings.TrimSpace(env.Session.Metadata["title"]) == "" {
					env.Session.Metadata["title"] = title
				}
			}
		}
		if err := c.SaveSessionAfterExecution(ctx, env.Session, result); err != nil {
			wf.fail(stagePersist, err)
			return attachWorkflow(result, env), err
		}
		wf.succeed(stagePersist, map[string]string{"session": env.Session.ID})
	}

	result.SessionID = env.Session.ID
	result.RunID = defaultString(result.RunID, ensuredRunID)
	result.ParentRunID = defaultString(result.ParentRunID, parentRunID)

	if executionErr != nil {
		return attachWorkflow(result, env), fmt.Errorf("task execution failed: %w", executionErr)
	}

	return attachWorkflow(result, env), nil
}

func buildCompletionDefaultsFromConfig(cfg appconfig.Config) react.CompletionDefaults {
	defaults := react.CompletionDefaults{}

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

func attachWorkflowSnapshot(result *agent.TaskResult, wf *agentWorkflow, sessionID, runID, parentRunID string) *agent.TaskResult {
	if result == nil {
		result = &agent.TaskResult{}
	}
	result.SessionID = defaultString(result.SessionID, sessionID)
	result.RunID = defaultString(result.RunID, runID)
	result.ParentRunID = defaultString(result.ParentRunID, parentRunID)

	if wf != nil {
		snapshot := wf.snapshot()
		result.Workflow = &snapshot
	}

	return result
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (c *AgentCoordinator) loggerFor(ctx context.Context) agent.Logger {
	return logging.FromContext(ctx, c.logger)
}

// PrepareExecution prepares the execution environment without running the task
func (c *AgentCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return c.prepareExecutionWithListener(ctx, task, sessionID, nil)
}

// prepareExecutionWithListener prepares execution with event emission support
func (c *AgentCoordinator) prepareExecutionWithListener(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.ExecutionEnvironment, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	if listener == nil {
		if _, ok := c.prepService.(*preparation.ExecutionPreparationService); !ok && c.prepService != nil {
			return c.prepService.Prepare(ctx, task, sessionID)
		}
	}
	logger := c.loggerFor(ctx)
	prepService := preparation.NewExecutionPreparationService(preparation.ExecutionPreparationDeps{
		LLMFactory:    c.llmFactory,
		ToolRegistry:  c.toolRegistry,
		SessionStore:  c.sessionStore,
		ContextMgr:    c.contextMgr,
		HistoryMgr:    c.historyMgr,
		Parser:        c.parser,
		Config:        c.config,
		Logger:        logger,
		Clock:         c.clock,
		CostDecorator: cost.NewCostTrackingDecorator(c.costTracker, logger, c.clock),
		EventEmitter:  listener,
		CostTracker:   c.costTracker,
	})
	return prepService.Prepare(ctx, task, sessionID)
}

// SaveSessionAfterExecution saves session state after task completion
func (c *AgentCoordinator) SaveSessionAfterExecution(ctx context.Context, session *storage.Session, result *agent.TaskResult) error {
	logger := c.loggerFor(ctx)
	if c.historyMgr != nil && session != nil && result != nil {
		previousHistory, _ := c.historyMgr.Replay(ctx, session.ID, 0)
		incoming := append(agent.CloneMessages(previousHistory), stripUserHistoryMessages(result.Messages)...)
		if err := c.historyMgr.AppendTurn(ctx, session.ID, incoming); err != nil && logger != nil {
			logger.Warn("Failed to append turn history: %v", err)
		}
	}

	// Update session with results
	sanitizedMessages, attachmentStore := sanitizeMessagesForPersistence(result.Messages)
	if c.attachmentMigrator != nil && len(attachmentStore) > 0 {
		normalized, err := c.attachmentMigrator.Normalize(ctx, materialports.MigrationRequest{
			Attachments: attachmentStore,
			Origin:      "session_persist",
		})
		if err != nil && logger != nil {
			logger.Warn("Failed to migrate attachments for session persistence: %v", err)
		} else if normalized != nil {
			attachmentStore = normalized
		}
	}
	session.Messages = sanitizedMessages
	if len(attachmentStore) > 0 {
		session.Attachments = attachmentStore
	} else {
		session.Attachments = nil
	}
	if len(result.Important) > 0 {
		session.Important = ports.CloneImportantNotes(result.Important)
	} else {
		session.Important = nil
	}
	session.UpdatedAt = c.clock.Now()

	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	if result.SessionID != "" {
		session.Metadata["session_id"] = result.SessionID
	}
	if result.RunID != "" {
		session.Metadata["last_task_id"] = result.RunID
	}
	if result.ParentRunID != "" {
		session.Metadata["last_parent_task_id"] = result.ParentRunID
	} else {
		delete(session.Metadata, "last_parent_task_id")
	}

	logger.Debug("Saving session...")
	if err := c.sessionStore.Save(ctx, session); err != nil {
		logger.Error("Failed to save session: %v", err)
		return fmt.Errorf("failed to save session: %w", err)
	}
	logger.Debug("Session saved successfully")

	return nil
}

func (c *AgentCoordinator) persistSessionSnapshot(
	ctx context.Context,
	env *agent.ExecutionEnvironment,
	fallbackRunID string,
	parentRunID string,
	stopReason string,
) {
	logger := c.loggerFor(ctx)
	if env == nil || env.State == nil || env.Session == nil {
		return
	}

	state := env.State
	result := &agent.TaskResult{
		Answer:      "",
		Messages:    state.Messages,
		Iterations:  state.Iterations,
		TokensUsed:  state.TokenCount,
		StopReason:  stopReason,
		SessionID:   state.SessionID,
		RunID:       state.RunID,
		ParentRunID: state.ParentRunID,
		Important:   ports.CloneImportantNotes(state.Important),
	}

	if result.SessionID == "" {
		result.SessionID = env.Session.ID
	}
	if result.RunID == "" {
		result.RunID = fallbackRunID
	}
	if result.ParentRunID == "" {
		result.ParentRunID = parentRunID
	}

	if err := c.SaveSessionAfterExecution(ctx, env.Session, result); err != nil {
		logger.Error("Failed to persist session after failure: %v", err)
	}
}

// GetSession retrieves or creates a session (public method)
func (c *AgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return c.getSession(ctx, id)
}

// EnsureSession returns an existing session or creates one with the provided ID.
func (c *AgentCoordinator) EnsureSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return c.sessionStore.Create(ctx)
	}
	session, err := c.sessionStore.Get(ctx, id)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, storage.ErrSessionNotFound) {
		return nil, err
	}

	now := c.clock.Now()
	session = &storage.Session{
		ID:        id,
		Messages:  []ports.Message{},
		Todos:     []storage.Todo{},
		Metadata:  map[string]string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := c.sessionStore.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (c *AgentCoordinator) getSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return c.sessionStore.Create(ctx)
	}
	return c.sessionStore.Get(ctx, id)
}

func (c *AgentCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return c.sessionStore.List(ctx, limit, offset)
}

// GetCostTracker returns the cost tracker instance
func (c *AgentCoordinator) GetCostTracker() storage.CostTracker {
	return c.costTracker
}

// GetToolRegistry returns the tool registry instance
func (c *AgentCoordinator) GetToolRegistry() tools.ToolRegistry {
	return c.toolRegistry
}

// GetToolRegistryWithoutSubagent returns a filtered registry that excludes subagent
// This is used by subagent tool to prevent nested subagent calls
func (c *AgentCoordinator) GetToolRegistryWithoutSubagent() tools.ToolRegistry {
	// Check if the registry implements WithoutSubagent method
	type registryWithFilter interface {
		WithoutSubagent() tools.ToolRegistry
	}

	if filtered, ok := c.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	// Fallback: return original registry if filtering not supported
	return c.toolRegistry
}

// GetConfig returns the coordinator configuration
func (c *AgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{
		LLMProvider:   c.config.LLMProvider,
		LLMModel:      c.config.LLMModel,
		MaxTokens:     c.config.MaxTokens,
		MaxIterations: c.config.MaxIterations,
		Temperature:   c.config.Temperature,
		TopP:          c.config.TopP,
		StopSequences: append([]string(nil), c.config.StopSequences...),
		AgentPreset:   c.config.AgentPreset,
		ToolPreset:    c.config.ToolPreset,
		ToolMode:      c.config.ToolMode,
	}
}

// SetEnvironmentSummary updates the environment context appended to system prompts.
func (c *AgentCoordinator) SetEnvironmentSummary(summary string) {
	c.config.EnvironmentSummary = summary
	if c.prepService != nil {
		c.prepService.SetEnvironmentSummary(summary)
	}
}

// SetAttachmentMigrator wires an attachment migrator for boundary externalization.
// Agent state keeps inline payloads; CDN rewriting happens at HTTP/SSE boundaries.
func (c *AgentCoordinator) SetAttachmentMigrator(migrator materialports.Migrator) {
	c.attachmentMigrator = migrator
}

func sanitizeAttachmentForPersistence(att ports.Attachment) ports.Attachment {
	uri := strings.TrimSpace(att.URI)
	if uri != "" && !strings.HasPrefix(strings.ToLower(uri), "data:") {
		att.Data = ""
	}
	return att
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
				attachments[name] = sanitizeAttachmentForPersistence(att)
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

func stripUserHistoryMessages(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	trimmed := make([]ports.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Source == ports.MessageSourceUserHistory {
			continue
		}
		trimmed = append(trimmed, msg)
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

// obfuscateSessionID masks session identifiers when logging to avoid leaking
// potentially sensitive values. It retains a short prefix and suffix to keep
// logs useful for correlation while hiding the majority of the identifier.
func obfuscateSessionID(id string) string {
	if id == "" {
		return ""
	}

	if len(id) <= 8 {
		return "****"
	}

	return fmt.Sprintf("%s...%s", id[:4], id[len(id)-4:])
}

// GetLLMClient returns an LLM client
func (c *AgentCoordinator) GetLLMClient() (llm.LLMClient, error) {
	client, err := c.llmFactory.GetClient(c.config.LLMProvider, c.config.LLMModel, llm.LLMConfig{
		APIKey:  c.config.APIKey,
		BaseURL: c.config.BaseURL,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetParser returns the function call parser
func (c *AgentCoordinator) GetParser() tools.FunctionCallParser {
	return c.parser
}

// GetContextManager returns the context manager
func (c *AgentCoordinator) GetContextManager() agent.ContextManager {
	return c.contextMgr
}

// PreviewContextWindow constructs the current context window for a session
// without mutating session state. This is intended for diagnostics in
// development flows.
func (c *AgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	preview := agent.ContextWindowPreview{}

	if c.contextMgr == nil {
		return preview, fmt.Errorf("context manager not configured")
	}

	session, err := c.GetSession(ctx, sessionID)
	if err != nil {
		return preview, err
	}

	toolMode := presets.ToolMode(strings.TrimSpace(c.config.ToolMode))
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	toolPreset := strings.TrimSpace(c.config.ToolPreset)
	if c.prepService != nil {
		if resolved := c.prepService.ResolveToolPreset(ctx, toolPreset); resolved != "" {
			toolPreset = resolved
		}
		if resolved := c.prepService.ResolveAgentPreset(ctx, c.config.AgentPreset); resolved != "" {
			preview.PersonaKey = resolved
		}
	}
	if preview.PersonaKey == "" {
		preview.PersonaKey = c.config.AgentPreset
	}
	if toolMode == presets.ToolModeCLI && toolPreset == "" {
		toolPreset = string(presets.ToolPresetFull)
	}

	window, err := c.contextMgr.BuildWindow(ctx, session, agent.ContextWindowConfig{
		TokenLimit:         c.config.MaxTokens,
		PersonaKey:         preview.PersonaKey,
		ToolMode:           string(toolMode),
		ToolPreset:         toolPreset,
		EnvironmentSummary: c.config.EnvironmentSummary,
	})
	if err != nil {
		return preview, fmt.Errorf("build context window: %w", err)
	}

	preview.Window = window
	preview.TokenEstimate = c.contextMgr.EstimateTokens(window.Messages)
	preview.TokenLimit = c.config.MaxTokens
	preview.ToolMode = string(toolMode)
	preview.ToolPreset = toolPreset

	return preview, nil
}

// GetSystemPrompt returns the system prompt
func (c *AgentCoordinator) GetSystemPrompt() string {
	if c.contextMgr == nil {
		return preparation.DefaultSystemPrompt
	}
	personaKey := c.config.AgentPreset
	toolMode := c.config.ToolMode
	if strings.TrimSpace(toolMode) == "" {
		toolMode = string(presets.ToolModeCLI)
	}
	toolPreset := c.config.ToolPreset
	if c.prepService != nil {
		if resolved := c.prepService.ResolveAgentPreset(context.Background(), personaKey); resolved != "" {
			personaKey = resolved
		}
		if resolved := c.prepService.ResolveToolPreset(context.Background(), toolPreset); resolved != "" {
			toolPreset = resolved
		}
	} else if toolPreset == "" {
		toolPreset = string(presets.ToolPresetFull)
	}
	session := &storage.Session{ID: "", Messages: nil}
	window, err := c.contextMgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{
		TokenLimit:         c.config.MaxTokens,
		PersonaKey:         personaKey,
		ToolMode:           toolMode,
		ToolPreset:         toolPreset,
		EnvironmentSummary: c.config.EnvironmentSummary,
	})
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("Failed to build preview context window: %v", err)
		}
		return preparation.DefaultSystemPrompt
	}
	if prompt := strings.TrimSpace(window.SystemPrompt); prompt != "" {
		return prompt
	}
	return preparation.DefaultSystemPrompt
}

// resolveUserID extracts a user identifier from the session metadata.
func (c *AgentCoordinator) resolveUserID(session *storage.Session) string {
	if session == nil || session.Metadata == nil {
		return ""
	}
	if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
		return uid
	}
	// Fallback: use session ID prefix for Lark/WeChat sessions
	if strings.HasPrefix(session.ID, "lark:") || strings.HasPrefix(session.ID, "wechat:") {
		return session.ID
	}
	return ""
}

// extractToolCallInfo extracts tool call information from TaskResult messages.
// It scans assistant messages for ToolCalls (which carry the tool name) and
// matches them with subsequent tool result messages.
func extractToolCallInfo(result *agent.TaskResult) []hooks.ToolResultInfo {
	if result == nil {
		return nil
	}

	// Build a map of call_id → tool name from assistant messages
	callNames := make(map[string]string)
	for _, msg := range result.Messages {
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				callNames[tc.ID] = tc.Name
			}
		}
	}

	// Collect tool results from ToolResult entries in messages
	var calls []hooks.ToolResultInfo
	for _, msg := range result.Messages {
		for _, tr := range msg.ToolResults {
			name := callNames[tr.CallID]
			if name == "" {
				name = "unknown"
			}
			calls = append(calls, hooks.ToolResultInfo{
				ToolName: name,
				Success:  tr.Error == nil,
				Output:   truncateString(tr.Content, 200),
			})
		}
	}
	return calls
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// performTaskPreAnalysis performs quick task analysis using LLM
// executeWithToolDisplay wraps ReactEngine execution with tool call display
