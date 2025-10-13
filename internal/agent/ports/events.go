package ports

import "time"

// AgentEvent represents a domain event emitted during execution
// It mirrors the contract implemented by the domain layer events.
type AgentEvent interface {
	EventType() string
	Timestamp() time.Time
	GetAgentLevel() AgentLevel
	GetSessionID() string
}

// EventListener consumes agent events (used by TUI/streaming layers)
type EventListener interface {
	OnEvent(event AgentEvent)
}

// NoopEventListener is an EventListener implementation that discards all events.
type NoopEventListener struct{}

// OnEvent discards the event without processing.
func (NoopEventListener) OnEvent(event AgentEvent) {}
