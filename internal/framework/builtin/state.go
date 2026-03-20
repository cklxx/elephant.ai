package builtin

import (
	"context"

	"alex/internal/core/hook"
)

// LoadState implements hook.StateLoader.
// Stub: no-op for now. Will be wired to load session messages later.
func (p *Plugin) LoadState(_ context.Context, _ *hook.TurnState) error {
	return nil
}

// SaveState implements hook.StateSaver.
// Stub: no-op for now. Will wrap existing SaveSessionAfterExecution later.
func (p *Plugin) SaveState(_ context.Context, _ *hook.TurnState) error {
	return nil
}

var _ hook.StateLoader = (*Plugin)(nil)
var _ hook.StateSaver = (*Plugin)(nil)
