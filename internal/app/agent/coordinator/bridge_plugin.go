package coordinator

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/hooks"
	"alex/internal/core/hook"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
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
func (p *bridgePlugin) ResolveSession(_ context.Context, state *hook.TurnState) error {
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
// "prepare" workflow stage.
func (p *bridgePlugin) LoadState(ctx context.Context, state *hook.TurnState) error {
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

	// Finish prepare stage (metadata, latency, workflow events)
	if p.wf != nil {
		c.finishPrepareStage(ctx, state.Input, env, p.wf, p.outCtx, p.ensuredRunID, p.parentRunID, prepareStarted)
	} else {
		// Even without workflow, ensure metadata is set
		ensureSessionMetadata(env.Session, "user_id", id.UserIDFromContext(ctx))
		ensureSessionMetadata(env.Session, "channel", appcontext.ChannelFromContext(ctx))
		clilatency.PrintfWithContext(ctx,
			"[latency] prepare_ms=%.2f session=%s\n",
			float64(time.Since(prepareStarted))/float64(time.Millisecond),
			env.Session.ID,
		)
	}

	return nil
}

// BuildPrompt returns a pass-through prompt; the ReAct engine builds the
// real prompt internally.
func (p *bridgePlugin) BuildPrompt(_ context.Context, state *hook.TurnState) (*hook.Prompt, error) {
	return &hook.Prompt{
		System:   "",
		Messages: state.Messages,
	}, nil
}

// PreTask runs proactive hooks (OKR injection, etc.) before model execution.
func (p *bridgePlugin) PreTask(ctx context.Context, state *hook.TurnState) error {
	if p.lastEnv == nil {
		return nil
	}
	c := p.coordinator
	logger := c.loggerFor(ctx)
	c.runPreTaskHooks(ctx, state.Input, p.lastEnv, p.ensuredRunID, logger)
	return nil
}

// RunModel is the key method: it wraps buildAndRunReactEngine.
// Preparation and pre-task hooks have already run in LoadState and PreTask.
func (p *bridgePlugin) RunModel(ctx context.Context, state *hook.TurnState, _ *hook.Prompt) (*hook.ModelOutput, error) {
	if p.lastEnv == nil {
		return nil, nil
	}

	c := p.coordinator
	effectiveCfg := c.effectiveConfig(ctx)
	logger := c.loggerFor(ctx)

	// Use the workflow created by the caller — buildAndRunReactEngine will
	// manage the stageExecute transitions on this workflow.
	wf := p.wf
	if wf == nil {
		wf = newAgentWorkflow(p.ensuredRunID, slog.Default(), p.eventListener, nil)
	}

	result, execErr := c.buildAndRunReactEngine(ctx, state.Input, p.lastEnv, wf, p.eventListener, effectiveCfg, logger)

	p.lastResult = result
	p.lastExecErr = execErr

	// Store references in TurnState for PostTask access
	state.Set("task_result", result)
	state.Set("execution_env", p.lastEnv)
	state.Set("execution_error", execErr)

	// Record summarize stage (mirrors old finalizeExecution)
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
		return &hook.ModelOutput{
			Text:       "",
			StopReason: "error",
		}, nil
	}

	return &hook.ModelOutput{
		Text:       result.Answer,
		StopReason: result.StopReason,
		Usage: hook.Usage{
			TotalTokens: result.TokensUsed,
		},
	}, nil
}

// SaveState persists the session after execution.
func (p *bridgePlugin) SaveState(ctx context.Context, _ *hook.TurnState) error {
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
func (p *bridgePlugin) RenderOutbound(_ context.Context, state *hook.TurnState, output *hook.ModelOutput) ([]hook.Outbound, error) {
	if output == nil || output.Text == "" {
		return nil, nil
	}
	return []hook.Outbound{{
		Channel:   state.Channel,
		SessionID: state.SessionID,
		Content:   output.Text,
	}}, nil
}

// DispatchOutbound is a no-op; outbound dispatch is handled by existing callers.
func (p *bridgePlugin) DispatchOutbound(_ context.Context, _ []hook.Outbound) error {
	return nil
}

// PostTask runs post-task hooks (memory capture, predictions, etc.) and logs costs.
func (p *bridgePlugin) PostTask(ctx context.Context, state *hook.TurnState, _ *hook.TurnResult) error {
	if p.lastEnv == nil || p.lastResult == nil {
		return nil
	}

	c := p.coordinator
	c.logSessionCost(ctx, p.sessionID, c.loggerFor(ctx))
	c.runPostTaskHooks(ctx, state.Input, p.lastEnv, p.lastResult, p.lastExecErr, p.ensuredRunID)

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
	_ hook.Plugin             = (*bridgePlugin)(nil)
	_ hook.SessionResolver    = (*bridgePlugin)(nil)
	_ hook.StateLoader        = (*bridgePlugin)(nil)
	_ hook.PromptBuilder      = (*bridgePlugin)(nil)
	_ hook.PreTaskHook        = (*bridgePlugin)(nil)
	_ hook.ModelRunner        = (*bridgePlugin)(nil)
	_ hook.StateSaver         = (*bridgePlugin)(nil)
	_ hook.OutboundRenderer   = (*bridgePlugin)(nil)
	_ hook.OutboundDispatcher = (*bridgePlugin)(nil)
	_ hook.PostTaskHook       = (*bridgePlugin)(nil)
)

// proactiveHookAdapter wraps a legacy hooks.ProactiveHook as a core/hook Plugin
// that implements PreTaskHook and PostTaskHook.
type proactiveHookAdapter struct {
	legacy hooks.ProactiveHook
}

func (a *proactiveHookAdapter) Name() string  { return a.legacy.Name() }
func (a *proactiveHookAdapter) Priority() int { return 50 }

func (a *proactiveHookAdapter) PreTask(ctx context.Context, state *hook.TurnState) error {
	task := hooks.TaskInfo{
		TaskInput: state.Input,
		SessionID: state.SessionID,
		RunID:     state.RunID,
		UserID:    state.UserID,
	}
	// Injections are returned but not applied here — the bridge plugin's
	// PreTask handles injection via the old registry path. This adapter
	// exists for future direct-registration scenarios.
	_ = a.legacy.OnTaskStart(ctx, task)
	return nil
}

func (a *proactiveHookAdapter) PostTask(ctx context.Context, state *hook.TurnState, result *hook.TurnResult) error {
	ri := hooks.TaskResultInfo{
		TaskInput: state.Input,
		SessionID: state.SessionID,
		RunID:     state.RunID,
		UserID:    state.UserID,
	}
	if result != nil && result.ModelOutput != nil {
		ri.Answer = result.ModelOutput.Text
	}
	return a.legacy.OnTaskCompleted(ctx, ri)
}

// AdaptProactiveHook wraps a legacy ProactiveHook as a core/hook Plugin.
func AdaptProactiveHook(h hooks.ProactiveHook) hook.Plugin {
	return &proactiveHookAdapter{legacy: h}
}
