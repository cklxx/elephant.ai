package domain

import (
	"time"

	"alex/internal/agent/ports"
)

// WorkflowEventEnvelope standardizes the streaming contract across backends and frontends.
// It wraps existing agent events with semantic event_type identifiers while preserving
// legacy types for transitionary consumers.
type WorkflowEventEnvelope struct {
	BaseEvent
	// Version allows future evolution of the envelope contract.
	Version int
	// Event contains the semantic event_type (e.g., workflow.node.started).
	Event string
	// LegacyType carries the original domain EventType for dual-publish transitions.
	LegacyType string
	// WorkflowID references the workflow instance producing the event.
	WorkflowID string
	// RunID mirrors WorkflowID for compatibility with run-scoped consumers.
	RunID string
	// NodeID denotes the workflow node or tool call identifier.
	NodeID string
	// NodeKind classifies the node (step, plan, tool, subflow, result, diagnostic, etc.).
	NodeKind string
	// Payload holds event-specific data. It is sanitized at the streaming layer.
	Payload map[string]any
}

// EventType satisfies ports.AgentEvent with the semantic event name.
func (e *WorkflowEventEnvelope) EventType() string {
	return e.Event
}

// NewWorkflowEnvelopeFromEvent copies base context from the originating event while
// assigning the semantic event_type.
func NewWorkflowEnvelopeFromEvent(event ports.AgentEvent, eventType string) *WorkflowEventEnvelope {
	if event == nil {
		return nil
	}
	ts := event.Timestamp()
	if ts.IsZero() {
		ts = time.Now()
	}
	return &WorkflowEventEnvelope{
		BaseEvent:  NewBaseEvent(event.GetAgentLevel(), event.GetSessionID(), event.GetTaskID(), event.GetParentTaskID(), ts),
		Event:      eventType,
		LegacyType: event.EventType(),
		Version:    1,
	}
}
