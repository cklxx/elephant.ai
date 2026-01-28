package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
)

type capturingHistoryStore struct {
	mu   sync.Mutex
	last agent.AgentEvent
}

func (s *capturingHistoryStore) Append(_ context.Context, event agent.AgentEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.last = event
	return nil
}

func (s *capturingHistoryStore) Stream(_ context.Context, _ EventHistoryFilter, _ func(agent.AgentEvent) error) error {
	return nil
}

func (s *capturingHistoryStore) DeleteSession(_ context.Context, _ string) error { return nil }

func (s *capturingHistoryStore) HasSessionEvents(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (s *capturingHistoryStore) lastEvent() agent.AgentEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.last
}

func TestEventBroadcasterPersistsSubtaskWrappers(t *testing.T) {
	store := &capturingHistoryStore{}
	broadcaster := NewEventBroadcaster(WithEventHistoryStore(store))

	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "parent", now),
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
		level: agent.LevelSubagent,
		meta: agent.SubtaskMetadata{
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
	if got.GetAgentLevel() != agent.LevelSubagent {
		t.Fatalf("expected persisted agent level %q, got %q", agent.LevelSubagent, got.GetAgentLevel())
	}
}

func TestEventBroadcasterSkipsExecutorUpdates(t *testing.T) {
	store := &capturingHistoryStore{}
	broadcaster := NewEventBroadcaster(WithEventHistoryStore(store))

	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "parent", now),
		Version:   1,
		Event:     "workflow.executor.update",
		NodeKind:  "diagnostic",
		NodeID:    "executor-update-1",
		Payload: map[string]any{
			"update_type": "tool_call",
		},
	}

	broadcaster.OnEvent(envelope)

	if got := store.lastEvent(); got != nil {
		t.Fatalf("expected executor update to be skipped in history, got %T", got)
	}
}

func TestEventBroadcasterSkipsHighVolumeHistoryEvents(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name   string
		event  agent.AgentEvent
		expect bool
	}{
		{
			name: "skip output delta",
			event: &domain.WorkflowEventEnvelope{
				BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", now),
				Version:   1,
				Event:     "workflow.node.output.delta",
				NodeKind:  "generation",
				Payload: map[string]any{
					"delta": "stream",
				},
			},
			expect: false,
		},
		{
			name: "skip tool progress",
			event: &domain.WorkflowEventEnvelope{
				BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", now),
				Version:   1,
				Event:     "workflow.tool.progress",
				NodeKind:  "tool",
				Payload: map[string]any{
					"chunk": "partial",
				},
			},
			expect: false,
		},
		{
			name: "skip streaming final chunk",
			event: &domain.WorkflowEventEnvelope{
				BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", now),
				Version:   1,
				Event:     "workflow.result.final",
				NodeKind:  "result",
				Payload: map[string]any{
					"final_answer":    "partial",
					"is_streaming":    true,
					"stream_finished": false,
				},
			},
			expect: false,
		},
		{
			name: "keep terminal final",
			event: &domain.WorkflowEventEnvelope{
				BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", now),
				Version:   1,
				Event:     "workflow.result.final",
				NodeKind:  "result",
				Payload: map[string]any{
					"final_answer":    "complete",
					"is_streaming":    false,
					"stream_finished": true,
				},
			},
			expect: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &capturingHistoryStore{}
			broadcaster := NewEventBroadcaster(WithEventHistoryStore(store))

			broadcaster.OnEvent(tc.event)

			got := store.lastEvent()
			if tc.expect {
				if got == nil {
					t.Fatalf("expected event to be persisted")
				}
			} else if got != nil {
				t.Fatalf("expected event to be skipped, got %T", got)
			}
		})
	}
}
