package coordinator

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/hooks"
	corehook "alex/internal/core/hook"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	react "alex/internal/domain/agent/react"
	infraadapters "alex/internal/infra/adapters"
	infraruntime "alex/internal/infra/runtime"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

// bridgePlugin wraps AgentCoordinator's dependencies to implement the new hook
// interfaces. This is a transitional adapter: it makes existing code work
// through the Framework without rewriting all business logic.
type bridgePlugin struct {
	coordinator *AgentCoordinator

	// lastResult is populated during RunModel so the caller can read it back
	// after ProcessInbound returns.
	lastResult *agent.TaskResult

	// lastEnv is populated during RunModel for use by SaveState and PostTask.
	lastEnv *agent.ExecutionEnvironment

	// lastExecErr captures any execution error from RunModel for PostTask.
	lastExecErr error

	// wf is the agent workflow tracking prepare/execute/summarize/persist stages.
	// Created by the caller (ExecuteTask) and passed in.
	wf *agentWorkflow

	// outCtx is the output context for workflow event emission.
	outCtx *agent.OutputContext

	// sessionID is resolved during execution.
	sessionID string

	// parentRunID is set by the caller before ProcessInbound.
	parentRunID string

	// ensuredRunID is set by the caller before ProcessInbound.
	ensuredRunID string

	// dispatcher is set by the caller for title extraction.
	dispatcher EventDispatcher

	// eventListener is set by the caller for ReAct engine.
	eventListener agent.EventListener
}

func newBridgePlugin(c *AgentCoordinator) *bridgePlugin {
	return &bridgePlugin{coordinator: c}
}

func (p *bridgePlugin) Name() string  { return "coordinator-bridge" }
func (p *bridgePlugin) Priority() int { return 100 }

// ResolveSession resolves session IDs from envelope metadata into TurnState.
func (p *bridgePlugin) ResolveSession(_ context.Context, state *corehook.TurnState) error {
	if sid, ok := state.Metadata["session_id"]; ok {
		if s, ok := sid.(string); ok {
			state.SessionID = s
			p.sessionID = s
		}
	}
	if rid, ok := state.Metadata["run_id"]; ok {
		if s, ok := rid.(string); ok {
			state.RunID = s
			p.ensuredRunID = s
		}
	}
	if prid, ok := state.Metadata["parent_run_id"]; ok {
		if s, ok := prid.(string); ok {
			state.ParentRunID = s
			p.parentRunID = s
		}
	}
	return nil
}

// LoadState prepares the execution environment. This maps to the old
// "prepare" workflow stage. Inlined from the old finishPrepareStage method.
func (p *bridgePlugin) LoadState(ctx context.Context, state *corehook.TurnState) error {
	c := p.coordinator
	effectiveCfg := c.effectiveConfig(ctx)
	prepareStarted := time.Now()

	// stagePrepare was started by ExecuteTask before ProcessInbound.

	env, err := c.prepareExecutionWithListener(ctx, state.Input, p.sessionID, p.eventListener, effectiveCfg)
	if err != nil {
		if p.wf != nil {
			p.wf.fail(stagePrepare, err)
		}
		return err
	}
	p.lastEnv = env

	// Update session ID if it was empty (new session created)
	if p.sessionID == "" && env.Session != nil {
		p.sessionID = env.Session.ID
		state.SessionID = env.Session.ID
	}

	// Finish prepare stage — metadata, latency, workflow events.
	ensureSessionMetadata(env.Session, "user_id", id.UserIDFromContext(ctx))
	ensureSessionMetadata(env.Session, "channel", appcontext.ChannelFromContext(ctx))
	clilatency.PrintfWithContext(ctx,
		"[latency] prepare_ms=%.2f session=%s\n",
		float64(time.Since(prepareStarted))/float64(time.Millisecond),
		env.Session.ID,
	)

	if p.wf != nil {
		p.outCtx.SessionID = env.Session.ID
		p.outCtx.TaskID = p.ensuredRunID
		p.outCtx.ParentTaskID = p.parentRunID
		p.wf.setContext(p.outCtx)

		prepareOutput := map[string]any{
			"session": env.Session.ID,
			"task":    state.Input,
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
		p.wf.succeed(stagePrepare, prepareOutput)
	}

	return nil
}

// BuildPrompt returns a pass-through prompt; the ReAct engine builds the
// real prompt internally.
func (p *bridgePlugin) BuildPrompt(_ context.Context, state *corehook.TurnState) (*corehook.Prompt, error) {
	return &corehook.Prompt{
		System:   "",
		Messages: state.Messages,
	}, nil
}

// PreTask runs proactive hooks (OKR injection, etc.) before model execution.
// Inlined from the old runPreTaskHooks coordinator method.
func (p *bridgePlugin) PreTask(ctx context.Context, state *corehook.TurnState) error {
	if p.lastEnv == nil {
		return nil
	}
	c := p.coordinator
	logger := c.loggerFor(ctx)

	if c.hookRuntime == nil || appcontext.IsSubagentContext(ctx) {
		return nil
	}

	hookState := &corehook.TurnState{
		Input:     state.Input,
		SessionID: p.lastEnv.Session.ID,
		RunID:     p.ensuredRunID,
		UserID:    c.resolveUserID(ctx, p.lastEnv.Session),
	}

	corehook.CallMany[any](ctx, c.hookRuntime, func(pl corehook.Plugin) (any, bool, error) {
		if h, ok := pl.(corehook.PreTaskHook); ok {
			return nil, false, h.PreTask(ctx, hookState)
		}
		return nil, false, nil
	})

	// Collect legacy injections stored by adapted ProactiveHooks.
	if raw, ok := hookState.Get("legacy_injections"); ok {
		if injections, ok := raw.([]hooks.Injection); ok && len(injections) > 0 {
			proactiveContext := hooks.FormatInjectionsAsContext(injections)
			if proactiveContext != "" {
				p.lastEnv.State.Messages = append(p.lastEnv.State.Messages, ports.Message{
					Role:    "user",
					Content: proactiveContext,
					Source:  ports.MessageSourceProactive,
				})
				logger.Info("Injected %d proactive context items", len(injections))
			}
		}
	}
	return nil
}

// RunModel constructs the ReactEngine and runs SolveTask.
// Preparation and pre-task hooks have already run in LoadState and PreTask.
// Inlined from the old buildAndRunReactEngine coordinator method.
func (p *bridgePlugin) RunModel(ctx context.Context, state *corehook.TurnState, _ *corehook.Prompt) (*corehook.ModelOutput, error) {
	if p.lastEnv == nil {
		return nil, nil
	}

	c := p.coordinator
	effectiveCfg := c.effectiveConfig(ctx)
	logger := c.loggerFor(ctx)

	// Use the workflow created by the caller — the ReactEngine will
	// manage the stageExecute transitions on this workflow.
	wf := p.wf
	if wf == nil {
		wf = newAgentWorkflow(p.ensuredRunID, slog.Default(), p.eventListener, nil)
	}

	// Build and run the ReAct engine.
	env := p.lastEnv
	task := state.Input

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
			c.asyncSaveSession(env.Session)
		},
		BackgroundExecutor: backgroundExecutor,
		BackgroundManager:  bgManager,
		AtomicFileWriter:   infraadapters.NewOSAtomicWriter(),
	})

	if p.eventListener != nil {
		reactEngine.SetEventListener(p.eventListener)
	}

	wf.start(stageExecute)
	result, execErr := reactEngine.SolveTask(ctx, task, env.State, env.Services)
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

	p.lastResult = result
	p.lastExecErr = execErr

	// Store references in TurnState for PostTask access
	state.Set("task_result", result)
	state.Set("execution_env", p.lastEnv)
	state.Set("execution_error", execErr)

	// Record execute/summarize stages
	if wf != nil {
		if execErr != nil {
			wf.fail(stageExecute, execErr)
		} else if result != nil {
			wf.succeed(stageExecute, map[string]any{
				"iterations": result.Iterations,
				"stop":       result.StopReason,
			})
		}

		wf.start(stageSummarize)
		answerPreview := ""
		if result != nil && execErr == nil {
			answerPreview = strings.TrimSpace(result.Answer)
		}
		wf.succeed(stageSummarize, map[string]any{"answer_preview": answerPreview})
	}

	if execErr != nil {
		logger.Error("Task execution failed: %v", execErr)
		// Don't return error here — let SaveState and PostTask still run.
		// The error is captured in lastExecErr and will be returned via
		// the framework's TurnResult.
		return &corehook.ModelOutput{
			Text:       "",
			StopReason: "error",
		}, nil
	}

	return &corehook.ModelOutput{
		Text:       result.Answer,
		StopReason: result.StopReason,
		Usage: corehook.Usage{
			TotalTokens: result.TokensUsed,
		},
	}, nil
}

// SaveState persists the session after execution.
func (p *bridgePlugin) SaveState(ctx context.Context, _ *corehook.TurnState) error {
	if p.lastEnv == nil || p.lastResult == nil {
		return nil
	}

	c := p.coordinator
	logger := c.loggerFor(ctx)

	if p.wf != nil {
		p.wf.start(stagePersist)
	}

	if appcontext.IsSubagentContext(ctx) {
		logger.Debug("Skipping session persistence for subagent execution")
		if p.wf != nil {
			p.wf.succeed(stagePersist, "skipped (subagent context)")
		}
		return nil
	}

	// Set title from dispatcher if available
	if p.dispatcher != nil {
		if title := strings.TrimSpace(p.dispatcher.Title()); title != "" {
			ensureSessionMetadata(p.lastEnv.Session, "title", title)
		}
	}

	if err := c.SaveSessionAfterExecution(ctx, p.lastEnv.Session, p.lastResult); err != nil {
		if p.wf != nil {
			p.wf.fail(stagePersist, err)
		}
		return err
	}

	if p.wf != nil {
		p.wf.succeed(stagePersist, map[string]string{"session": p.lastEnv.Session.ID})
	}
	return nil
}

// RenderOutbound returns basic text rendering of the model output.
func (p *bridgePlugin) RenderOutbound(_ context.Context, state *corehook.TurnState, output *corehook.ModelOutput) ([]corehook.Outbound, error) {
	if output == nil || output.Text == "" {
		return nil, nil
	}
	return []corehook.Outbound{{
		Channel:   state.Channel,
		SessionID: state.SessionID,
		Content:   output.Text,
	}}, nil
}

// DispatchOutbound is a no-op; outbound dispatch is handled by existing callers.
func (p *bridgePlugin) DispatchOutbound(_ context.Context, _ []corehook.Outbound) error {
	return nil
}

// PostTask runs post-task hooks (memory capture, predictions, etc.) and logs costs.
// Inlined from the old logSessionCost and runPostTaskHooks coordinator methods.
func (p *bridgePlugin) PostTask(ctx context.Context, state *corehook.TurnState, _ *corehook.TurnResult) error {
	if p.lastEnv == nil || p.lastResult == nil {
		return nil
	}

	c := p.coordinator
	logger := c.loggerFor(ctx)

	// Log session cost — inlined from logSessionCost.
	if c.costTracker != nil {
		sessionStats, err := c.costTracker.GetSessionStats(ctx, p.sessionID)
		if err != nil {
			logger.Warn("Failed to get session stats: %v", err)
		} else {
			logger.Info("Session summary: requests=%d, total_tokens=%d (input=%d, output=%d), cost=$%.6f, duration=%v",
				sessionStats.RequestCount, sessionStats.TotalTokens,
				sessionStats.InputTokens, sessionStats.OutputTokens,
				sessionStats.TotalCost, sessionStats.Duration)
		}
	}

	// Run post-task hooks — inlined from runPostTaskHooks.
	if c.hookRuntime != nil && !appcontext.IsSubagentContext(ctx) && p.lastExecErr == nil {
		hookState := &corehook.TurnState{
			Input:     state.Input,
			SessionID: p.lastEnv.Session.ID,
			RunID:     p.ensuredRunID,
			UserID:    c.resolveUserID(ctx, p.lastEnv.Session),
		}

		turnResult := &corehook.TurnResult{
			SessionID: p.lastEnv.Session.ID,
			RunID:     p.ensuredRunID,
			Input:     state.Input,
		}
		if p.lastResult != nil {
			turnResult.ModelOutput = &corehook.ModelOutput{
				Text:       p.lastResult.Answer,
				StopReason: p.lastResult.StopReason,
			}
		}

		corehook.CallMany[any](ctx, c.hookRuntime, func(pl corehook.Plugin) (any, bool, error) {
			if h, ok := pl.(corehook.PostTaskHook); ok {
				return nil, false, h.PostTask(ctx, hookState, turnResult)
			}
			return nil, false, nil
		})
	}

	// Store error metadata in session if execution failed
	if p.lastExecErr != nil {
		metadata := storage.EnsureMetadata(p.lastEnv.Session)
		metadata["last_error"] = p.lastExecErr.Error()
		metadata["last_error_at"] = c.clock.Now().Format(time.RFC3339)
	}

	return nil
}

// Compile-time interface assertions.
var (
	_ corehook.Plugin             = (*bridgePlugin)(nil)
	_ corehook.SessionResolver    = (*bridgePlugin)(nil)
	_ corehook.StateLoader        = (*bridgePlugin)(nil)
	_ corehook.PromptBuilder      = (*bridgePlugin)(nil)
	_ corehook.PreTaskHook        = (*bridgePlugin)(nil)
	_ corehook.ModelRunner        = (*bridgePlugin)(nil)
	_ corehook.StateSaver         = (*bridgePlugin)(nil)
	_ corehook.OutboundRenderer   = (*bridgePlugin)(nil)
	_ corehook.OutboundDispatcher = (*bridgePlugin)(nil)
	_ corehook.PostTaskHook       = (*bridgePlugin)(nil)
)
