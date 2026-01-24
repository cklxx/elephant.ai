package app

import (
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

func TestEventBroadcasterBroadcastsToRegisteredClients(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	ch := make(chan ports.AgentEvent, 1)
	broadcaster.RegisterClient("session-1", ch)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "session-1", "task-1", "", time.Now()),
		Version:   1,
		Event:     "workflow.node.started",
		NodeKind:  "plan",
		NodeID:    "plan-1",
	}

	broadcaster.OnEvent(event)

	select {
	case got := <-ch:
		if got != event {
			t.Fatalf("expected event to be delivered, got %T", got)
		}
	default:
		t.Fatalf("expected event to be delivered to registered client")
	}
}
