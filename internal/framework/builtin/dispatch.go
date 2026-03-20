package builtin

import (
	"context"

	"alex/internal/core/hook"
)

// DispatchOutbound implements hook.OutboundDispatcher.
// Stub: no-op for now. Will be wired to channel.Manager.Send later.
func (p *Plugin) DispatchOutbound(_ context.Context, _ []hook.Outbound) error {
	return nil
}

var _ hook.OutboundDispatcher = (*Plugin)(nil)
