package builtin

import (
	"context"
	"errors"

	"alex/internal/core/hook"
)

// ErrNoModelRunner indicates no model runner has been configured.
var ErrNoModelRunner = errors.New("builtin: no model runner configured")

// RunModel implements hook.ModelRunner.
// Returns an error indicating no model runner is configured.
// Will be wired to existing buildAndRunReactEngine later.
func (p *Plugin) RunModel(_ context.Context, _ *hook.TurnState, _ *hook.Prompt) (*hook.ModelOutput, error) {
	return nil, ErrNoModelRunner
}

var _ hook.ModelRunner = (*Plugin)(nil)
