package adapter

import (
	"context"
	"fmt"

	"alex/internal/runtime/panel"
)

// CodexExecutor is the minimal interface for executing a Codex task.
// It is satisfied by bridge.Executor.Execute (injected at wiring time so
// the adapter package stays free of infra/external dependencies).
type CodexExecutor interface {
	// Execute runs a Codex task and returns its answer, or an error.
	Execute(ctx context.Context, prompt, workDir string, onProgress func()) (string, error)
}

// CodexAdapter launches a Codex task via a bridge executor goroutine.
// A Kaku pane is created for visual status only; the actual execution runs
// through the executor, not through a pty session.
type CodexAdapter struct {
	pm       panel.ManagerIface
	executor CodexExecutor
	sink     HookSink
}

// NewCodexAdapter creates an adapter for Codex sessions.
func NewCodexAdapter(pm panel.ManagerIface, executor CodexExecutor, sink HookSink) *CodexAdapter {
	return &CodexAdapter{pm: pm, executor: executor, sink: sink}
}

// Start creates a visual pane and launches the Codex executor goroutine.
func (a *CodexAdapter) Start(ctx context.Context, opts StartOpts) error {
	// Create a visual pane so the operator can see status.
	pane, err := a.pm.Split(ctx, panel.SplitOpts{
		ParentPaneID: opts.PaneID,
		Direction:    "bottom",
		Percent:      40,
		WorkDir:      opts.WorkDir,
	})
	if err != nil {
		return fmt.Errorf("codex adapter: split pane: %w", err)
	}
	_ = pane.Activate(ctx)

	// Show a status header in the pane.
	statusLine := fmt.Sprintf("echo '[Codex] session %s — running…'", opts.SessionID)
	_ = pane.Send(ctx, statusLine)

	// Run the executor in a goroutine. Heartbeats are fired on every progress callback.
	go func() {
		answer, execErr := a.executor.Execute(ctx, opts.Goal, opts.WorkDir, func() {
			a.sink.OnHeartbeat(opts.SessionID)
		})
		if execErr != nil {
			_ = pane.Send(ctx, fmt.Sprintf("echo '[Codex] FAILED: %s'", execErr.Error()))
			a.sink.OnFailed(opts.SessionID, execErr.Error())
			return
		}
		_ = pane.Send(ctx, "echo '[Codex] completed.'")
		a.sink.OnCompleted(opts.SessionID, answer)
	}()

	return nil
}

// Inject is a no-op for Codex: the executor runs to completion autonomously.
func (a *CodexAdapter) Inject(_ context.Context, sessionID, _ string) error {
	return fmt.Errorf("codex adapter: inject not supported for session %s", sessionID)
}

// Stop is a no-op: context cancellation (by the caller) propagates to the
// executor goroutine. There is no pane handle to kill because the pane is
// fire-and-forget.
func (a *CodexAdapter) Stop(_ context.Context, _ string, _ bool) error {
	return nil
}
