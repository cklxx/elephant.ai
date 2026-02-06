package app

import (
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

func BenchmarkEventBroadcasterOnEventWithSubscriber(b *testing.B) {
	broadcaster := NewEventBroadcaster(WithMaxHistory(1))
	broadcaster.logger = logging.Nop()

	ch := make(chan agentports.AgentEvent, 1024)
	broadcaster.RegisterClient("bench-session", ch)
	defer broadcaster.UnregisterClient("bench-session", ch)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ch:
			}
		}
	}()
	defer close(done)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, "bench-session", "run-1", "", time.Now()),
		Version:   1,
		Event:     types.EventToolProgress,
		NodeID:    "tool:bench",
		NodeKind:  "tool",
		Payload: map[string]any{
			"call_id":   "call-1",
			"tool_name": "bash",
			"chunk":     "benchmark payload",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcaster.OnEvent(event)
	}
}

func BenchmarkEventBroadcasterOnEventNoSubscriber(b *testing.B) {
	broadcaster := NewEventBroadcaster(WithMaxHistory(1))
	broadcaster.logger = logging.Nop()

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, "bench-session", "run-1", "", time.Now()),
		Version:   1,
		Event:     types.EventToolProgress,
		NodeID:    "tool:bench",
		NodeKind:  "tool",
		Payload: map[string]any{
			"call_id":   "call-1",
			"tool_name": "bash",
			"chunk":     "benchmark payload",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcaster.OnEvent(event)
	}
}
