package app

import (
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
)

func TestMultiEventListenerFanOut(t *testing.T) {
	var countA, countB atomic.Int32

	listenerA := domain.EventListenerFunc(func(event agent.AgentEvent) { countA.Add(1) })
	listenerB := domain.EventListenerFunc(func(event agent.AgentEvent) { countB.Add(1) })

	multi := NewMultiEventListener(listenerA, listenerB)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "s1", "r1", "", time.Now()),
		Event:     "test.event",
	}

	multi.OnEvent(event)

	if got := countA.Load(); got != 1 {
		t.Fatalf("expected listener A to receive 1 event, got %d", got)
	}
	if got := countB.Load(); got != 1 {
		t.Fatalf("expected listener B to receive 1 event, got %d", got)
	}
}

func TestMultiEventListenerHandlesNilListener(t *testing.T) {
	var count atomic.Int32
	listener := domain.EventListenerFunc(func(event agent.AgentEvent) { count.Add(1) })

	multi := NewMultiEventListener(nil, listener)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "s1", "r1", "", time.Now()),
		Event:     "test.event",
	}

	// Should not panic
	multi.OnEvent(event)

	if got := count.Load(); got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}
}
