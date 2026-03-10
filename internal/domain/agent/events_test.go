package domain

import (
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func TestEvent_EventType(t *testing.T) {
	e := NewEvent("test.kind", BaseEvent{})
	if e.EventType() != "test.kind" {
		t.Errorf("expected test.kind, got %s", e.EventType())
	}
}

func TestEvent_GetAttachments_NilAndEmpty(t *testing.T) {
	e := &Event{}
	if e.GetAttachments() != nil {
		t.Error("expected nil for zero-value event")
	}

	e.Data.Attachments = map[string]ports.Attachment{}
	if e.GetAttachments() != nil {
		t.Error("expected nil for empty attachments")
	}
}

func TestEvent_GetAttachments_DeepCopy(t *testing.T) {
	original := map[string]ports.Attachment{
		"img.png": {Name: "img.png", MediaType: "image/png"},
	}
	e := &Event{Data: EventData{Attachments: original}}
	cloned := e.GetAttachments()

	if cloned == nil || len(cloned) != 1 {
		t.Fatal("expected 1 attachment")
	}
	// Mutation should not affect original
	cloned["img.png"] = ports.Attachment{Name: "modified"}
	if original["img.png"].Name != "img.png" {
		t.Error("original was mutated")
	}
}

func TestNewEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewEvent(types.EventNodeStarted, base)
	if e.Kind != types.EventNodeStarted {
		t.Errorf("expected %s, got %s", types.EventNodeStarted, e.Kind)
	}
}

func TestPercentageOf(t *testing.T) {
	tests := []struct {
		value, total int
		want         float64
	}{
		{50, 100, 50.0},
		{0, 100, 0.0},
		{100, 100, 100.0},
		{1, 0, 0.0},
		{1, -1, 0.0},
	}
	for _, tt := range tests {
		got := percentageOf(tt.value, tt.total)
		if got != tt.want {
			t.Errorf("percentageOf(%d, %d) = %f, want %f", tt.value, tt.total, got, tt.want)
		}
	}
}

func TestEventListenerFunc(t *testing.T) {
	var called bool
	fn := EventListenerFunc(func(e AgentEvent) {
		called = true
	})
	base := NewBaseEvent(agent.LevelCore, "", "", "", time.Now())
	e := NewEvent("test", base)
	fn.OnEvent(e)
	if !called {
		t.Error("expected listener to be called")
	}
}
