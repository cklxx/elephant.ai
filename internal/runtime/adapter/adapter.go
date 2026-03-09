// Package adapter provides the MemberAdapter abstraction that launches and
// controls individual member CLIs (claude_code, codex, …) inside Kaku panes.
// Each adapter is responsible for:
//   - Creating a Kaku pane and starting the CLI inside it.
//   - Injecting the session goal and subsequent input.
//   - Detecting completion and reporting it through HookSink.
package adapter

import "context"

// StartMode controls how the adapter obtains its Kaku pane.
type StartMode int

const (
	// ModeSplit creates a new pane by splitting from an existing parent pane.
	ModeSplit StartMode = iota
	// ModeDirectPane reuses an existing pane (typically from the pane pool).
	ModeDirectPane
)

// StartOpts bundles the parameters for Adapter.Start.
type StartOpts struct {
	SessionID string
	Goal      string
	WorkDir   string
	PaneID    int       // ModeSplit: parent pane to split from; ModeDirectPane: pane to reuse
	Mode      StartMode // how to obtain the pane
}

// Adapter controls one member CLI instance for a runtime session.
type Adapter interface {
	// Start launches the member CLI and injects the session goal.
	// It returns as soon as the goal has been submitted; it does not wait for completion.
	Start(ctx context.Context, opts StartOpts) error

	// Inject sends additional text into the running CLI (e.g. to unblock a stalled session).
	Inject(ctx context.Context, sessionID, text string) error

	// Stop terminates the CLI. If poolPane is true, the pane is kept alive
	// (only the CLI process is exited); otherwise the pane is killed.
	Stop(ctx context.Context, sessionID string, poolPane bool) error
}

// HookSink receives structured lifecycle callbacks from an adapter.
// It is implemented by runtime.Runtime, which translates them into state
// machine transitions and event bus publications.
type HookSink interface {
	OnHeartbeat(sessionID string)
	OnCompleted(sessionID, answer string)
	OnFailed(sessionID, errMsg string)
	OnNeedsInput(sessionID, prompt string)
}
