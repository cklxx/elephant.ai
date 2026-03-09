// Package adapter provides the MemberAdapter abstraction that launches and
// controls individual member CLIs (claude_code, codex, …) inside Kaku panes.
// Each adapter is responsible for:
//   - Creating a Kaku pane and starting the CLI inside it.
//   - Injecting the session goal and subsequent input.
//   - Detecting completion and reporting it through HookSink.
package adapter

import "context"

// Adapter controls one member CLI instance for a runtime session.
type Adapter interface {
	// Start creates a Kaku pane, launches the member CLI, and injects the session goal.
	// It returns as soon as the goal has been submitted; it does not wait for completion.
	Start(ctx context.Context, sessionID, goal, workDir string, parentPaneID int) error

	// Inject sends additional text into the running CLI (e.g. to unblock a stalled session).
	Inject(ctx context.Context, sessionID, text string) error

	// Stop terminates the CLI and kills its Kaku pane.
	Stop(ctx context.Context, sessionID string) error
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
