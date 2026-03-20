package builtin

import (
	"context"

	"alex/internal/core/hook"
)

// RenderOutbound implements hook.OutboundRenderer.
// Converts ModelOutput into Outbound messages.
// Basic implementation: creates a single text Outbound from ModelOutput.Text.
func (p *Plugin) RenderOutbound(_ context.Context, state *hook.TurnState, output *hook.ModelOutput) ([]hook.Outbound, error) {
	if output == nil || output.Text == "" {
		return nil, nil
	}

	return []hook.Outbound{
		{
			Channel:   state.Channel,
			SessionID: state.SessionID,
			Content:   output.Text,
		},
	}, nil
}

var _ hook.OutboundRenderer = (*Plugin)(nil)
