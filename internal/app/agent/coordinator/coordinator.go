package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	appconfig "alex/internal/app/agent/config"
	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/cost"
	"alex/internal/app/agent/hooks"
	"alex/internal/app/agent/preparation"
	sessiontitle "alex/internal/app/agent/sessiontitle"
	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	react "alex/internal/domain/agent/react"
	"alex/internal/domain/agent/textutil"
	"alex/internal/domain/agent/types"
	materialports "alex/internal/domain/materials/ports"
	infraruntime "alex/internal/infra/runtime"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

type RuntimeConfigResolver func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
	llmFactory       llm.LLMClientFactory
	toolRegistry     tools.ToolRegistry
	sessionStore     storage.SessionStore
	contextMgr       agent.ContextManager
	historyMgr       storage.HistoryManager
	parser           agent.FunctionCallParser
	costTracker      storage.CostTracker
	config           appconfig.Config
	runtimeResolver  RuntimeConfigResolver
	logger           agent.Logger
	clock            agent.Clock
	externalExecutor agent.ExternalAgentExecutor
	bgRegistry       *backgroundTaskRegistry
	iterationHook    agent.IterationHook
	checkpointStore  react.CheckpointStore

	prepService           preparationService
	costDecorator         *cost.CostTrackingDecorator
	attachmentMigrator    materialports.Migrator
	attachmentPersister   ports.AttachmentPersister
	hookRegistry          *hooks.Registry
	okrContextProvider    preparation.OKRContextProvider
	kernelContextProvider preparation.KernelAlignmentContextProvider
	credentialRefresher   preparation.CredentialRefresher
	timerManager          shared.TimerManagerService // injected at bootstrap; tools retrieve via shared.TimerManagerFromContext
	schedulerService      any                        // injected at bootstrap; tools retrieve via shared.SchedulerFromContext
	toolSLACollector      *toolspolicy.SLACollector

	sessionSaveMu sync.Mutex // Protects concurrent session saves
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
	parser agent.FunctionCallParser,
	costTracker storage.CostTracker,
	config appconfig.Config,
	opts ...CoordinatorOption,
) *AgentCoordinator {
	if len(config.StopSequences) > 0 {
		config.StopSequences = append([]string(nil), config.StopSequences...)
	}
	config.LLMProfile = config.DefaultLLMProfile()

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
		bgRegistry:   newBackgroundTaskRegistry(),
	}

	for _, opt := range opts {
		opt(coordinator)
	}

	// Create services only if not provided via options
	if coordinator.costDecorator == nil {
		coordinator.costDecorator = cost.NewCostTrackingDecorator(costTracker, coordinator.logger, coordinator.clock)
	}

	coordinator.prepService = preparation.NewExecutionPreparationService(preparation.ExecutionPreparationDeps{
		LLMFactory:            llmFactory,
		ToolRegistry:          toolRegistry,
		SessionStore:          sessionStore,
		ContextMgr:            contextMgr,
		HistoryMgr:            historyManager,
		Parser:                parser,
		Config:                config,
		Logger:                coordinator.logger,
		Clock:                 coordinator.clock,
		CostDecorator:         coordinator.costDecorator,
		CostTracker:           coordinator.costTracker,
		OKRContextProvider:    coordinator.okrContextProvider,
		KernelContextProvider: coordinator.kernelContextProvider,
		CredentialRefresher:   coordinator.credentialRefresher,
	})

	if coordinator.contextMgr != nil {
		if err := coordinator.contextMgr.Preload(context.Background()); err != nil {
			coordinator.logger.Warn("Context preload failed: %v", err)
		}
	}

	return coordinator
}

type planSessionTitleRecorder struct {
	mu      sync.RWMutex
	title   string
	sink    agent.EventListener
	onTitle func(string)
}

func (r *planSessionTitleRecorder) OnEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}

	if e, ok := event.(*domain.Event); ok && e.Kind == types.EventToolCompleted {
		if e.Data.Error == nil && strings.EqualFold(strings.TrimSpace(e.Data.ToolName), "plan") {
			if title := extractPlanSessionTitle(e.Data.Metadata); title != "" {
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
	r.mu.RLock()
	defer r.mu.RUnlock()
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
	eventListener := wrapWithWorkflowEnvelope(wrapWithSLAEnrichment(listener, c.toolSLACollector), nil)
	var planTitleRecorder *planSessionTitleRecorder
	var serializingListener *SerializingEventListener
	if eventListener != nil && !appcontext.IsSubagentContext(ctx) {
		planTitleRecorder = &planSessionTitleRecorder{
			sink: eventListener,
			onTitle: func(title string) {
				c.persistSessionTitle(ctx, sessionID, title)
			},
		}
		eventListener = planTitleRecorder
	}
	if eventListener != nil {
		serializingListener = NewSerializingEventListener(eventListener)
		eventListener = serializingListener
	}

	ctx = id.WithSessionID(ctx, sessionID)
	if c.timerManager != nil {
		ctx = shared.WithTimerManager(ctx, c.timerManager)
	}
	if c.schedulerService != nil {
		ctx = shared.WithScheduler(ctx, c.schedulerService)
	}
	ctx, ensuredRunID := id.EnsureRunID(ctx, id.NewRunID)
	if ensuredRunID == "" {
		ensuredRunID = id.RunIDFromContext(ctx)
	}
	parentRunID := id.ParentRunIDFromContext(ctx)
	if serializingListener != nil {
		defer func() {
			flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			serializingListener.Flush(flushCtx, ensuredRunID)
		}()
	}

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

	effectiveCfg := c.effectiveConfig(ctx)

	// Prepare execution environment with event listener support
	env, err := c.prepareExecutionWithListener(ctx, task, sessionID, eventListener, effectiveCfg)
	if err != nil {
		wf.fail(stagePrepare, err)
		return attachWorkflow(nil, env), err
	}
	if sessionID == "" {
		sessionID = env.Session.ID
	}
	// Propagate context user_id into session metadata so resolveUserID
	// can find it for proactive hooks.
	if ctxUserID := id.UserIDFromContext(ctx); ctxUserID != "" {
		if env.Session.Metadata == nil {
			env.Session.Metadata = make(map[string]string)
		}
		if env.Session.Metadata["user_id"] == "" {
			env.Session.Metadata["user_id"] = ctxUserID
		}
	}
	// Propagate channel into session metadata for debug/diagnostic visibility.
	if channel := appcontext.ChannelFromContext(ctx); channel != "" {
		if env.Session.Metadata == nil {
			env.Session.Metadata = make(map[string]string)
		}
		if env.Session.Metadata["channel"] == "" {
			env.Session.Metadata["channel"] = channel
		}
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

	// Run proactive hooks (pre-task OKR injections, etc.)
	if c.hookRegistry != nil && !appcontext.IsSubagentContext(ctx) {
		hookTask := hooks.TaskInfo{
			TaskInput: task,
			SessionID: env.Session.ID,
			RunID:     ensuredRunID,
			UserID:    c.resolveUserID(ctx, env.Session),
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
	completionDefaults := buildCompletionDefaultsFromConfig(effectiveCfg)
	idAdapter := infraruntime.IDsAdapter{}
	latencyReporter := infraruntime.LatencyReporter
	jsonCodec := infraruntime.JSONCodec
	goRunner := infraruntime.GoRunner
	workingDirResolver := infraruntime.WorkingDirResolver
	workspaceMgrFactory := infraruntime.WorkspaceManagerFactory

	backgroundExecutor := func(bgCtx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		bgCtx = appcontext.MarkSubagentContext(bgCtx)
		return c.ExecuteTask(bgCtx, prompt, sessionID, listener)
	}
	var bgManager *react.BackgroundTaskManager
	if c.bgRegistry != nil && env != nil && env.Session != nil {
		bgManager = c.bgRegistry.Get(env.Session.ID, func() *react.BackgroundTaskManager {
			return react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
				RunContext:          ctx,
				Logger:              logger,
				Clock:               c.clock,
				IDGenerator:         idAdapter,
				IDContextReader:     idAdapter,
				GoRunner:            goRunner,
				WorkingDirResolver:  workingDirResolver,
				WorkspaceMgrFactory: workspaceMgrFactory,
				ExecuteTask:         backgroundExecutor,
				ExternalExecutor:    c.externalExecutor,
				SessionID:           env.Session.ID,
			})
		})
	}

	reactEngine := react.NewReactEngine(react.ReactEngineConfig{
		MaxIterations:       effectiveCfg.MaxIterations,
		Logger:              logger,
		Clock:               c.clock,
		IDGenerator:         idAdapter,
		IDContextReader:     idAdapter,
		LatencyReporter:     latencyReporter,
		JSONCodec:           jsonCodec,
		GoRunner:            goRunner,
		WorkingDirResolver:  workingDirResolver,
		WorkspaceMgrFactory: workspaceMgrFactory,
		CompletionDefaults:  completionDefaults,
		FinalAnswerReview: react.FinalAnswerReviewConfig{
			Enabled:            effectiveCfg.Proactive.FinalAnswerReview.Enabled,
			MaxExtraIterations: effectiveCfg.Proactive.FinalAnswerReview.MaxExtraIterations,
		},
		AttachmentMigrator:  c.attachmentMigrator,
		AttachmentPersister: c.attachmentPersister,
		CheckpointStore:     c.checkpointStore,
		Workflow:            wf,
		IterationHook:       c.iterationHook,
		SessionPersister: func(ctx context.Context, _ *storage.Session, state *agent.TaskState) {
			// Async persist after each iteration for diagnostics visibility.
			// Ignore the nil session param; we capture env.Session from closure.
			c.asyncSaveSession(ctx, env.Session)
		},
		BackgroundExecutor: backgroundExecutor,
		BackgroundManager:  bgManager,
		ExternalExecutor:   c.externalExecutor,
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

	// Run proactive hooks (post-task processing, etc.)
	if c.hookRegistry != nil && !appcontext.IsSubagentContext(ctx) && executionErr == nil {
		hookResult := hooks.TaskResultInfo{
			TaskInput:  task,
			Answer:     result.Answer,
			SessionID:  env.Session.ID,
			RunID:      ensuredRunID,
			UserID:     c.resolveUserID(ctx, env.Session),
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
	return c.prepareExecutionWithListener(ctx, task, sessionID, nil, c.effectiveConfig(ctx))
}

// prepareExecutionWithListener prepares execution with event emission support
func (c *AgentCoordinator) prepareExecutionWithListener(ctx context.Context, task string, sessionID string, listener agent.EventListener, cfg appconfig.Config) (*agent.ExecutionEnvironment, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	if listener == nil {
		if _, ok := c.prepService.(*preparation.ExecutionPreparationService); !ok && c.prepService != nil {
			return c.prepService.Prepare(ctx, task, sessionID)
		}
	}
	logger := c.loggerFor(ctx)
	prepService := preparation.NewExecutionPreparationService(preparation.ExecutionPreparationDeps{
		LLMFactory:            c.llmFactory,
		ToolRegistry:          c.toolRegistry,
		SessionStore:          c.sessionStore,
		ContextMgr:            c.contextMgr,
		HistoryMgr:            c.historyMgr,
		Parser:                c.parser,
		Config:                cfg,
		Logger:                logger,
		Clock:                 c.clock,
		CostDecorator:         c.costDecorator,
		EventEmitter:          listener,
		CostTracker:           c.costTracker,
		OKRContextProvider:    c.okrContextProvider,
		KernelContextProvider: c.kernelContextProvider,
		CredentialRefresher:   c.credentialRefresher,
	})
	return prepService.Prepare(ctx, task, sessionID)
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

// resolveUserID extracts a user identifier from the session metadata.
func (c *AgentCoordinator) resolveUserID(ctx context.Context, session *storage.Session) string {
	if ctx != nil {
		if uid := id.UserIDFromContext(ctx); uid != "" {
			return uid
		}
	}
	if session == nil || session.Metadata == nil {
		return ""
	}
	if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
		return uid
	}
	// Fallback: use session ID prefix for Lark sessions
	if strings.HasPrefix(session.ID, "lark-") {
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
				Output:   textutil.TruncateWithEllipsis(tr.Content, 200),
			})
		}
	}
	return calls
}

// performTaskPreAnalysis performs quick task analysis using LLM
// executeWithToolDisplay wraps ReactEngine execution with tool call display
