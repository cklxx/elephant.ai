package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

type capturingHistoryStore struct {
	mu   sync.Mutex
	last ports.AgentEvent
}

func (s *capturingHistoryStore) Append(_ context.Context, event ports.AgentEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.last = event
	return nil
}

func (s *capturingHistoryStore) Stream(_ context.Context, _ EventHistoryFilter, _ func(ports.AgentEvent) error) error {
	return nil
}

func (s *capturingHistoryStore) DeleteSession(_ context.Context, _ string) error { return nil }

func (s *capturingHistoryStore) HasSessionEvents(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (s *capturingHistoryStore) lastEvent() ports.AgentEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.last
}

func TestEventBroadcasterPersistsSubtaskWrappers(t *testing.T) {
	store := &capturingHistoryStore{}
	broadcaster := NewEventBroadcaster(WithEventHistoryStore(store))

	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", now),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:1",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}

	wrapper := &stubSubtaskWrapper{
		inner: envelope,
		level: ports.LevelSubagent,
		meta: ports.SubtaskMetadata{
			Index:       0,
			Total:       1,
			Preview:     "Delegated work",
			MaxParallel: 1,
		},
	}

	broadcaster.OnEvent(wrapper)

	got := store.lastEvent()
	if got == nil {
		t.Fatalf("expected event to be appended to history store")
	}
	if got != wrapper {
		t.Fatalf("expected wrapper event to be persisted, got %T", got)
	}
	if got.GetAgentLevel() != ports.LevelSubagent {
		t.Fatalf("expected persisted agent level %q, got %q", ports.LevelSubagent, got.GetAgentLevel())
	}
}

