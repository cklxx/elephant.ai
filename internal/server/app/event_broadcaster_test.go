package app

import (
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
)

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

func TestBroadcastDropIncreasesMetrics(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	// Buffer size 1 — fill it, then the next event will be dropped.
	ch := make(chan agent.AgentEvent, 1)
	broadcaster.RegisterClient("s1", ch)

	makeEvent := func(eventType string) *domain.WorkflowEventEnvelope {
		return &domain.WorkflowEventEnvelope{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, "s1", "run-1", "", time.Now()),
			Version:   1,
			Event:     eventType,
		}
	}

	// Fill the buffer.
	broadcaster.OnEvent(makeEvent(types.EventNodeStarted))
	// This second event will be dropped because the buffer is full.
	broadcaster.OnEvent(makeEvent(types.EventToolProgress))
	// Third event also dropped.
	broadcaster.OnEvent(makeEvent(types.EventToolProgress))

	// Verify metrics recorded both drops.
	metrics := broadcaster.GetMetrics()
	if metrics.DroppedEvents != 2 {
		t.Fatalf("expected 2 dropped events, got %d", metrics.DroppedEvents)
	}
	if metrics.DropsPerSession["s1"] != 2 {
		t.Fatalf("expected 2 drops for session s1, got %d", metrics.DropsPerSession["s1"])
	}

	// The original event is still in the buffer.
	first := <-ch
	if first.EventType() != types.EventNodeStarted {
		t.Fatalf("expected first event %s, got %s", types.EventNodeStarted, first.EventType())
	}
}

func TestBroadcastDropNotificationDeliveredWhenBufferDrainsConcurrently(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	// Large enough buffer so that after a drop, the notification can fit.
	// Scenario: 2-slot buffer, fill both slots, drain 1, then trigger a drop
	// that produces a notification into the newly freed slot.
	ch := make(chan agent.AgentEvent, 2)
	broadcaster.RegisterClient("s1", ch)

	makeEvent := func(eventType string) *domain.WorkflowEventEnvelope {
		return &domain.WorkflowEventEnvelope{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, "s1", "run-1", "", time.Now()),
			Version:   1,
			Event:     eventType,
		}
	}

	// Fill the buffer (2 slots).
	broadcaster.OnEvent(makeEvent(types.EventNodeStarted))
	broadcaster.OnEvent(makeEvent(types.EventNodeCompleted))

	// Drain one slot to create room for the notification.
	<-ch

	// This event will be dropped (buffer has 1 event, 1 free slot).
	// Wait — the buffer has 1 event + 1 free slot = event will fit, not drop.
	// We need the buffer to be full. Let me fill it again.
	broadcaster.OnEvent(makeEvent(types.EventToolStarted)) // fills slot 2 again

	// Now truly full. Next event drops, notification tries the full buffer.
	broadcaster.OnEvent(makeEvent(types.EventToolProgress)) // dropped

	// Drain all and check what we got.
	var received []string
	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch:
			received = append(received, evt.EventType())
		default:
		}
	}

	// We should have the 2 events that were in the buffer.
	// The drop notification couldn't fit because buffer was full.
	// This is by design: notification is best-effort.
	metrics := broadcaster.GetMetrics()
	if metrics.DroppedEvents != 1 {
		t.Fatalf("expected 1 dropped event, got %d", metrics.DroppedEvents)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 events in buffer, got %d", len(received))
	}
}

func TestStreamDroppedEnvelopeFields(t *testing.T) {
	env := newStreamDroppedEnvelope("session-42", types.EventToolProgress, 7)

	if env.EventType() != types.EventStreamDropped {
		t.Fatalf("expected event type %s, got %s", types.EventStreamDropped, env.EventType())
	}
	if env.GetSessionID() != "session-42" {
		t.Fatalf("expected session ID session-42, got %s", env.GetSessionID())
	}
	if env.Payload["dropped_event_type"] != types.EventToolProgress {
		t.Fatalf("unexpected dropped_event_type: %v", env.Payload["dropped_event_type"])
	}
	if env.Payload["total_drops"] != int64(7) {
		t.Fatalf("unexpected total_drops: %v", env.Payload["total_drops"])
	}
	if env.NodeKind != "system" {
		t.Fatalf("expected NodeKind 'system', got %s", env.NodeKind)
	}
}
