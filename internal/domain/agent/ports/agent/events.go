package agent

import "time"

// AgentEvent represents a domain event emitted during execution
// It mirrors the contract implemented by the domain layer events.
type AgentEvent interface {
	EventType() string
	Timestamp() time.Time
	GetAgentLevel() AgentLevel
	GetSessionID() string
	GetRunID() string
	GetParentRunID() string
	GetCorrelationID() string
	GetCausationID() string
	GetEventID() string
	GetSeq() uint64
}

// SubtaskMetadata describes contextual fields emitted alongside delegated subflows.
type SubtaskMetadata struct {
	Index       int
	Total       int
	Preview     string
	MaxParallel int
}

// SubtaskWrapper identifies events that wrap another AgentEvent with subtask context.
type SubtaskWrapper interface {
	AgentEvent
	SubtaskDetails() SubtaskMetadata
	WrappedEvent() AgentEvent
}

// EventListener consumes agent events (used by TUI/streaming layers)
type EventListener interface {
	OnEvent(event AgentEvent)
}

// NoopEventListener is an EventListener implementation that discards all events.
type NoopEventListener struct{}

// OnEvent discards the event without processing.
func (NoopEventListener) OnEvent(event AgentEvent) {}
