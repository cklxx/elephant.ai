package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appconfig "alex/internal/app/agent/config"
	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/hooks"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	react "alex/internal/domain/agent/react"
	infraadapters "alex/internal/infra/adapters"
	infraruntime "alex/internal/infra/runtime"
	"alex/internal/infra/tools/builtin/shared"
	utils "alex/internal/shared/utils"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

// ExecuteTask executes a task with optional event listener for streaming output
func (c *AgentCoordinator) ExecuteTask(
	ctx context.Context,
	task string,
	sessionID string,
	listener agent.EventListener,
) (*agent.TaskResult, error) {
	ctx, logger, dispatcher, eventListener, ensuredRunID, parentRunID, outCtx, cleanup :=
		c.setupExecutionContext(ctx, task, sessionID, listener)
	if cleanup != nil {
		defer cleanup()
	}

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
	prepareStarted := time.Now()

	// Prepare execution environment with event listener support
	env, err := c.prepareExecutionWithListener(ctx, task, sessionID, eventListener, effectiveCfg)
	if err != nil {
		wf.fail(stagePrepare, err)
		return attachWorkflow(nil, env), err
	}
	if sessionID == "" {
		sessionID = env.Session.ID
	}

	c.finishPrepareStage(ctx, task, env, wf, outCtx, ensuredRunID, parentRunID, prepareStarted)
	ctx = id.WithSessionID(ctx, env.Session.ID)

	c.runPreTaskHooks(ctx, task, env, ensuredRunID, logger)

	result, executionErr := c.buildAndRunReactEngine(ctx, task, env, wf, eventListener, effectiveCfg, ensuredRunID, parentRunID, logger)

	return c.finalizeExecution(ctx, task, env, wf, dispatcher, result, executionErr,
		ensuredRunID, parentRunID, logger, attachWorkflow)
}

// setupExecutionContext initialises IDs, dispatcher, output context and returns
// all the values needed by the orchestrator. It also returns a cleanup func for
// the deferred event flush.
func (c *AgentCoordinator) setupExecutionContext(
	ctx context.Context,
	task string,
	sessionID string,
	listener agent.EventListener,
) (
	_ context.Context,
	_ agent.Logger,
	_ EventDispatcher,
	eventListener agent.EventListener,
	ensuredRunID string,
	parentRunID string,
	outCtx *agent.OutputContext,
	cleanup func(),
) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := c.loggerFor(ctx)

	dispatcher := NewEventDispatcher(listener, c.toolSLACollector, EventDispatcherOptions{
		EnablePlanTitle: !appcontext.IsSubagentContext(ctx),
		OnPlanTitle: func(title string) {
			c.persistSessionTitle(ctx, sessionID, title)
		},
	})
	eventListener = dispatcher.Listener()

	ctx = id.WithSessionID(ctx, sessionID)
	if c.timerManager != nil {
		ctx = shared.WithTimerManager(ctx, c.timerManager)
	}
	if c.schedulerService != nil {
		ctx = shared.WithScheduler(ctx, c.schedulerService)
	}
	ctx, ensuredRunID = id.EnsureRunID(ctx, id.NewRunID)
	if ensuredRunID == "" {
		ensuredRunID = id.RunIDFromContext(ctx)
	}
	parentRunID = id.ParentRunIDFromContext(ctx)

	if eventListener != nil {
		cleanup = func() {
			flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			dispatcher.Flush(flushCtx, ensuredRunID)
		}
	}

	// Core run: set correlation_id = own run_id as root of the causal chain.
	if id.CorrelationIDFromContext(ctx) == "" && ensuredRunID != "" {
		ctx = id.WithCorrelationID(ctx, ensuredRunID)
	}
	outCtx = agent.GetOutputContext(ctx)
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
	logger.Info("ExecuteTask started: session=%s task_chars=%d", obfuscateSessionID(sessionID), len(strings.TrimSpace(task)))

	return ctx, logger, dispatcher, eventListener, ensuredRunID, parentRunID, outCtx, cleanup
}

// finishPrepareStage completes the prepare workflow stage after the execution
// environment has been successfully created.
func (c *AgentCoordinator) finishPrepareStage(
	ctx context.Context,
	task string,
	env *agent.ExecutionEnvironment,
	wf *agentWorkflow,
	outCtx *agent.OutputContext,
	ensuredRunID, parentRunID string,
	prepareStarted time.Time,
) {
	ensureSessionMetadata(env.Session, "user_id", id.UserIDFromContext(ctx))
	ensureSessionMetadata(env.Session, "channel", appcontext.ChannelFromContext(ctx))
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
}

// runPreTaskHooks executes proactive hooks (pre-task OKR injections, etc.)
func (c *AgentCoordinator) runPreTaskHooks(
	ctx context.Context,
	task string,
	env *agent.ExecutionEnvironment,
	ensuredRunID string,
	logger agent.Logger,
) {
	if c.hookRegistry == nil || appcontext.IsSubagentContext(ctx) {
		return
	}
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

// buildAndRunReactEngine constructs the ReactEngine and runs SolveTask.
func (c *AgentCoordinator) buildAndRunReactEngine(
	ctx context.Context,
	task string,
	env *agent.ExecutionEnvironment,
	wf *agentWorkflow,
	eventListener agent.EventListener,
	effectiveCfg appconfig.Config,
	ensuredRunID, parentRunID string,
	logger agent.Logger,
) (*agent.TaskResult, error) {
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
				MaxConcurrentTasks:  effectiveCfg.MaxBackgroundTasks,
				ContextPropagators: []agent.ContextPropagatorFunc{
					appcontext.PropagateLLMSelection,
				},
				TmuxSender:    infraadapters.NewExecTmuxSender(),
				EventAppender: infraadapters.NewFileEventAppender(),
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
		AttachmentMigrator:  c.attachmentMigrator,
		AttachmentPersister: c.attachmentPersister,
		CheckpointStore:     c.checkpointStore,
		Workflow:            wf,
		IterationHook:       c.iterationHook,
		SessionPersister: func(ctx context.Context, _ *storage.Session, state *agent.TaskState) {
			c.asyncSaveSession(ctx, env.Session)
		},
		BackgroundExecutor: backgroundExecutor,
		BackgroundManager:  bgManager,
		ExternalExecutor:   c.externalExecutor,
		TeamDefinitions:    c.teamDefinitions,
		TeamRunRecorder:    c.teamRunRecorder,
		AtomicFileWriter:   infraadapters.NewOSAtomicWriter(),
	})

	if eventListener != nil {
		reactEngine.SetEventListener(eventListener)
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

	return result, executionErr
}

// finalizeExecution handles post-execution: cancellation, cost logging,
// post-task hooks, error stamping, and session persistence.
func (c *AgentCoordinator) finalizeExecution(
	ctx context.Context,
	task string,
	env *agent.ExecutionEnvironment,
	wf *agentWorkflow,
	dispatcher EventDispatcher,
	result *agent.TaskResult,
	executionErr error,
	ensuredRunID, parentRunID string,
	logger agent.Logger,
	attachWorkflow func(*agent.TaskResult, *agent.ExecutionEnvironment) *agent.TaskResult,
) (*agent.TaskResult, error) {
	if ctx.Err() != nil {
		logger.Debug("Task execution cancelled: %v", ctx.Err())
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

	// Stamp execution error into session metadata so it survives persistence.
	if executionErr != nil {
		metadata := storage.EnsureMetadata(env.Session)
		metadata["last_error"] = executionErr.Error()
		metadata["last_error_at"] = c.clock.Now().UTC().Format(time.RFC3339)
	}

	// Save session unless this is a delegated subagent run.
	wf.start(stagePersist)
	if appcontext.IsSubagentContext(ctx) {
		logger.Debug("Skipping session persistence for subagent execution")
		wf.succeed(stagePersist, "skipped (subagent context)")
	} else {
		if title := strings.TrimSpace(dispatcher.Title()); title != "" {
			if env.Session.Metadata == nil {
				env.Session.Metadata = make(map[string]string)
			}
			if utils.IsBlank(env.Session.Metadata["title"]) {
				env.Session.Metadata["title"] = title
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
