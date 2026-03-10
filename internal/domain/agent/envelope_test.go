package domain

import (
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

func TestWorkflowEventEnvelope_EventType(t *testing.T) {
	env := &WorkflowEventEnvelope{Event: "workflow.test"}
	if env.EventType() != "workflow.test" {
		t.Errorf("expected workflow.test, got %s", env.EventType())
	}
}

func TestWorkflowEventEnvelope_GetAttachments_Nil(t *testing.T) {
	if (&WorkflowEventEnvelope{}).GetAttachments() != nil {
		t.Error("expected nil for empty payload")
	}
	var nilEnv *WorkflowEventEnvelope
	if nilEnv.GetAttachments() != nil {
		t.Error("expected nil for nil envelope")
	}
}

func TestWorkflowEventEnvelope_GetAttachments_TypedMap(t *testing.T) {
	atts := map[string]ports.Attachment{
		"file.txt": {Name: "file.txt", MediaType: "text/plain"},
	}
	env := &WorkflowEventEnvelope{
		Payload: map[string]any{"attachments": atts},
	}
	got := env.GetAttachments()
	if got == nil || len(got) != 1 {
		t.Fatal("expected 1 attachment")
	}
	if got["file.txt"].Name != "file.txt" {
		t.Errorf("expected file.txt, got %s", got["file.txt"].Name)
	}
}

func TestWorkflowEventEnvelope_GetAttachments_AnyMap(t *testing.T) {
	env := &WorkflowEventEnvelope{
		Payload: map[string]any{
			"attachments": map[string]any{
				"doc.pdf": ports.Attachment{Name: "doc.pdf", MediaType: "application/pdf"},
				"empty":   "not-an-attachment", // should be skipped
			},
		},
	}
	got := env.GetAttachments()
	if got == nil || len(got) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got))
	}
}

func TestWorkflowEventEnvelope_GetAttachments_AnyMap_EmptyKey(t *testing.T) {
	env := &WorkflowEventEnvelope{
		Payload: map[string]any{
			"attachments": map[string]any{
				"": ports.Attachment{Name: "fallback.png"},
			},
		},
	}
	got := env.GetAttachments()
	if got == nil || len(got) != 1 {
		t.Fatalf("expected 1 attachment from fallback name, got %d", len(got))
	}
	if _, ok := got["fallback.png"]; !ok {
		t.Error("expected key from att.Name fallback")
	}
}

func TestWorkflowEventEnvelope_GetAttachments_AnyMap_BothEmpty(t *testing.T) {
	env := &WorkflowEventEnvelope{
		Payload: map[string]any{
			"attachments": map[string]any{
				"  ": ports.Attachment{Name: "  "},
			},
		},
	}
	got := env.GetAttachments()
	if got != nil {
		t.Error("expected nil when both key and name are blank")
	}
}

func TestWorkflowEventEnvelope_GetAttachments_UnsupportedType(t *testing.T) {
	env := &WorkflowEventEnvelope{
		Payload: map[string]any{"attachments": 42},
	}
	if env.GetAttachments() != nil {
		t.Error("expected nil for unsupported attachment type")
	}
}

func TestNewWorkflowEnvelopeFromEvent_Nil(t *testing.T) {
	if NewWorkflowEnvelopeFromEvent(nil, "test") != nil {
		t.Error("expected nil for nil event")
	}
}

func TestNewWorkflowEnvelopeFromEvent_ValidEvent(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := NewBaseEvent(agent.LevelCore, "sess", "run", "parent", ts)
	base.SetCorrelationID("corr")
	base.SetCausationID("cause")
	base.SetSeq(42)

	e := NewEvent("original.type", base)
	env := NewWorkflowEnvelopeFromEvent(e, "workflow.wrapped")

	if env == nil {
		t.Fatal("expected non-nil envelope")
	}
	if env.Event != "workflow.wrapped" {
		t.Errorf("expected workflow.wrapped, got %s", env.Event)
	}
	if env.Version != 1 {
		t.Errorf("expected version 1, got %d", env.Version)
	}
	if env.GetSessionID() != "sess" {
		t.Errorf("expected sess, got %s", env.GetSessionID())
	}
	if env.GetCorrelationID() != "corr" {
		t.Errorf("expected corr, got %s", env.GetCorrelationID())
	}
	if env.GetSeq() != 42 {
		t.Errorf("expected 42, got %d", env.GetSeq())
	}
}

type logIDEvent struct {
	*Event
}

func (e *logIDEvent) GetLogID() string { return "log-id-123" }

func TestNewWorkflowEnvelopeFromEvent_WithLogID(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "", "", "", time.Now())
	inner := NewEvent("test", base)
	e := &logIDEvent{Event: inner}

	env := NewWorkflowEnvelopeFromEvent(e, "wrapped")
	if env.GetLogID() != "log-id-123" {
		t.Errorf("expected log-id-123, got %s", env.GetLogID())
	}
}

func TestNewWorkflowEnvelopeFromEvent_ZeroTimestamp(t *testing.T) {
	base := BaseEvent{agentLevel: agent.LevelCore}
	e := NewEvent("test", base)

	env := NewWorkflowEnvelopeFromEvent(e, "wrapped")
	if env.Timestamp().IsZero() {
		t.Error("expected non-zero timestamp when source is zero")
	}
}
