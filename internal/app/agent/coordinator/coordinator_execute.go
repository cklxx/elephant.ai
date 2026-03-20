package coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/core/envelope"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/framework"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

// ExecuteTask executes a task with optional event listener for streaming output.
// It delegates to Framework.ProcessInbound via the bridgePlugin, which
// encapsulates preparation, hook execution, ReAct engine invocation,
// session persistence, and post-task hooks into the 7-step turn lifecycle.
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

	timer := newStepTimer()

	// Create the workflow that tracks prepare/execute/summarize/persist stages.
	// Start prepare stage immediately — must be tracked even on early cancellation.
	wf := newAgentWorkflow(ensuredRunID, slog.Default(), eventListener, outCtx)
	wf.start(stagePrepare)

	// Create Framework with bridge plugin
	fw := framework.New(framework.Config{})
	bridge := newBridgePlugin(c)
	bridge.sessionID = sessionID
	bridge.ensuredRunID = ensuredRunID
	bridge.parentRunID = parentRunID
	bridge.dispatcher = dispatcher
	bridge.eventListener = eventListener
	bridge.wf = wf
	bridge.outCtx = outCtx
	fw.RegisterPlugin(bridge)

	// Build envelope from task input
	env := envelope.New(map[string]any{
		"content":       task,
		"session_id":    sessionID,
		"run_id":        ensuredRunID,
		"parent_run_id": parentRunID,
		"channel":       appcontext.ChannelFromContext(ctx),
	})

	// Process through framework — the bridge plugin drives all lifecycle stages
	processStart := time.Now()
	_, fwErr := fw.ProcessInbound(ctx, env)
	timer.track("framework_process", processStart)
	timer.logSummary(logger, ensuredRunID)

	// Handle context cancellation
	if ctx.Err() != nil && bridge.lastEnv != nil {
		logger.Debug("Task execution cancelled: %v", ctx.Err())
		c.persistSessionSnapshot(ctx, bridge.lastEnv, ensuredRunID, parentRunID, "cancelled")
	}

	// If preparation never completed (e.g., context cancelled before LoadState),
	// mark prepare stage as failed so the workflow snapshot reflects failure.
	if bridge.lastEnv == nil && fwErr != nil {
		wf.fail(stagePrepare, fwErr)
	}

	// Extract the TaskResult from the bridge plugin
	result := bridge.lastResult
	if result == nil {
		result = &agent.TaskResult{
			Answer:    "",
			SessionID: sessionID,
			RunID:     ensuredRunID,
		}
	}

	// Attach workflow snapshot
	resolvedSession := sessionID
	if bridge.lastEnv != nil && bridge.lastEnv.Session != nil && bridge.lastEnv.Session.ID != "" {
		resolvedSession = bridge.lastEnv.Session.ID
	}
	result = attachWorkflowSnapshot(result, wf, resolvedSession, ensuredRunID, parentRunID)

	result.SessionID = defaultString(result.SessionID, resolvedSession)
	result.RunID = defaultString(result.RunID, ensuredRunID)
	result.ParentRunID = defaultString(result.ParentRunID, parentRunID)

	if bridge.lastExecErr != nil {
		return result, fmt.Errorf("task execution failed: %w", bridge.lastExecErr)
	}
	if fwErr != nil {
		return result, fwErr
	}

	return result, nil
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
