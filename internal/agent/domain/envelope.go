package domain

import (
	"strings"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
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
	// WorkflowID references the workflow instance producing the event.
	WorkflowID string
	// RunID mirrors WorkflowID for compatibility with run-scoped consumers.
	RunID string
	// NodeID denotes the workflow node or tool call identifier.
	NodeID string
	// NodeKind classifies the node (step, plan, tool, subflow, result, diagnostic, etc.).
	NodeKind string
	// Subtask metadata allows clients to render delegated workstreams.
	IsSubtask      bool
	SubtaskIndex   int
	TotalSubtasks  int
	SubtaskPreview string
	MaxParallel    int
	// Payload holds event-specific data. It is sanitized at the streaming layer.
	Payload map[string]any
}

// EventType satisfies agent.AgentEvent with the semantic event name.
func (e *WorkflowEventEnvelope) EventType() string {
	return e.Event
}

// GetAttachments exposes attachments when the payload includes them.
func (e *WorkflowEventEnvelope) GetAttachments() map[string]ports.Attachment {
	if e == nil || len(e.Payload) == 0 {
		return nil
	}
	raw, ok := e.Payload["attachments"]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]ports.Attachment:
		return ports.CloneAttachmentMap(typed)
	case map[string]any:
		attachments := make(map[string]ports.Attachment)
		for key, value := range typed {
			att, ok := value.(ports.Attachment)
			if !ok {
				continue
			}
			name := strings.TrimSpace(key)
			if name == "" {
				name = strings.TrimSpace(att.Name)
			}
			if name == "" {
				continue
			}
			if att.Name == "" {
				att.Name = name
			}
			attachments[name] = att
		}
		if len(attachments) == 0 {
			return nil
		}
		return attachments
	default:
		return nil
	}
}

// NewWorkflowEnvelopeFromEvent copies base context from the originating event while
// assigning the semantic event_type.
func NewWorkflowEnvelopeFromEvent(event agent.AgentEvent, eventType string) *WorkflowEventEnvelope {
	if event == nil {
		return nil
	}
	ts := event.Timestamp()
	if ts.IsZero() {
		ts = time.Now()
	}
	env := &WorkflowEventEnvelope{
		BaseEvent: NewBaseEvent(event.GetAgentLevel(), event.GetSessionID(), event.GetTaskID(), event.GetParentTaskID(), ts),
		Event:     eventType,
		Version:   1,
	}
	if withLogID, ok := event.(interface{ GetLogID() string }); ok {
		env.SetLogID(withLogID.GetLogID())
	}
	return env
}
