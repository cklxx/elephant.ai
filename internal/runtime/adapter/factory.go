package adapter

import (
	"fmt"

	"alex/internal/runtime/panel"
	"alex/internal/runtime/session"
)

// Factory creates Adapters for different member types.
type Factory struct {
	pm       panel.ManagerIface
	sink     HookSink
	hooksURL string
	codex    CodexExecutor // optional; nil disables Codex support
}

// NewFactory creates an AdapterFactory.
// codex may be nil if Codex is not configured.
func NewFactory(pm panel.ManagerIface, sink HookSink, hooksURL string, codex CodexExecutor) *Factory {
	return &Factory{pm: pm, sink: sink, hooksURL: hooksURL, codex: codex}
}

// New returns an Adapter for the given member type.
func (f *Factory) New(member session.MemberType) (Adapter, error) {
	switch member {
	case session.MemberClaudeCode:
		return NewClaudeCodeAdapter(f.pm, f.sink, f.hooksURL), nil
	case session.MemberCodex:
		if f.codex == nil {
			return nil, fmt.Errorf("adapter factory: codex executor not configured")
		}
		return NewCodexAdapter(f.pm, f.codex, f.sink), nil
	default:
		return nil, fmt.Errorf("adapter factory: unsupported member type %q", member)
	}
}
