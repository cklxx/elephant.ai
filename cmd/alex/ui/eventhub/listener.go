package eventhub

import "alex/internal/agent/ports"

// Listener bridges coordinator events into the UI hub.
type Listener struct {
	hub *Hub
}

// NewListener returns a listener that forwards events to the provided hub.
func NewListener(hub *Hub) *Listener {
	if hub == nil {
		return &Listener{}
	}
	return &Listener{hub: hub}
}

// OnEvent implements ports.EventListener.
func (l *Listener) OnEvent(event ports.AgentEvent) {
	if l == nil || l.hub == nil {
		return
	}
	l.hub.PublishAgentEvent(event)
}
