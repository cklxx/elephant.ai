package app

import (
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
)

func TestGetActiveRunIDReturnsRegisteredRun(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	// No run registered → empty string
	if got := broadcaster.GetActiveRunID("session-1"); got != "" {
		t.Fatalf("expected empty string for unregistered session, got %q", got)
	}

	// Register run → returns run ID
	broadcaster.RegisterRunSession("session-1", "run-abc")
	if got := broadcaster.GetActiveRunID("session-1"); got != "run-abc" {
		t.Fatalf("expected run-abc, got %q", got)
	}

	// Unregister → returns empty string again
	broadcaster.UnregisterRunSession("session-1")
	if got := broadcaster.GetActiveRunID("session-1"); got != "" {
		t.Fatalf("expected empty string after unregister, got %q", got)
	}
}

func TestEventBroadcasterBroadcastsToRegisteredClients(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	ch := make(chan agent.AgentEvent, 1)
	broadcaster.RegisterClient("session-1", ch)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", "task-1", "", time.Now()),
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
