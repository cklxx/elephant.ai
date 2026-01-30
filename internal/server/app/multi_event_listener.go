package app

import agent "alex/internal/agent/ports/agent"

// MultiEventListener fans out events to multiple listeners.
type MultiEventListener struct {
	listeners []agent.EventListener
}

// NewMultiEventListener creates a listener that forwards events to all provided listeners.
func NewMultiEventListener(listeners ...agent.EventListener) *MultiEventListener {
	return &MultiEventListener{listeners: listeners}
}

// OnEvent implements agent.EventListener.
func (m *MultiEventListener) OnEvent(event agent.AgentEvent) {
	for _, l := range m.listeners {
		if l != nil {
			l.OnEvent(event)
		}
	}
}
