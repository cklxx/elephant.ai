package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
)

type stubSubtaskWrapper struct {
	inner agent.AgentEvent
	meta  agent.SubtaskMetadata
	level agent.AgentLevel
}

func (w *stubSubtaskWrapper) EventType() string                     { return w.inner.EventType() }
func (w *stubSubtaskWrapper) Timestamp() time.Time                  { return w.inner.Timestamp() }
func (w *stubSubtaskWrapper) GetSessionID() string                  { return w.inner.GetSessionID() }
func (w *stubSubtaskWrapper) GetRunID() string                      { return w.inner.GetRunID() }
func (w *stubSubtaskWrapper) GetParentRunID() string                { return w.inner.GetParentRunID() }
func (w *stubSubtaskWrapper) GetCorrelationID() string              { return w.inner.GetCorrelationID() }
func (w *stubSubtaskWrapper) GetCausationID() string                { return w.inner.GetCausationID() }
func (w *stubSubtaskWrapper) GetEventID() string                    { return w.inner.GetEventID() }
func (w *stubSubtaskWrapper) GetSeq() uint64                        { return w.inner.GetSeq() }
func (w *stubSubtaskWrapper) SubtaskDetails() agent.SubtaskMetadata { return w.meta }
func (w *stubSubtaskWrapper) WrappedEvent() agent.AgentEvent        { return w.inner }
func (w *stubSubtaskWrapper) GetAgentLevel() agent.AgentLevel {
	if w.level != "" {
		return w.level
	}
	return agent.LevelSubagent
}

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

func TestEventBroadcasterEvictsOldSessions(t *testing.T) {
	broadcaster := NewEventBroadcaster(WithMaxSessions(2))

	broadcaster.OnEvent(makeToolEnvelope("session-a", time.Now()))
	time.Sleep(10 * time.Millisecond)
	broadcaster.OnEvent(makeToolEnvelope("session-b", time.Now()))
	time.Sleep(10 * time.Millisecond)
	broadcaster.OnEvent(makeToolEnvelope("session-c", time.Now()))

	if got := broadcaster.GetEventHistory("session-a"); got != nil {
		t.Fatalf("expected oldest session history to be evicted")
	}
	if got := broadcaster.GetEventHistory("session-b"); got == nil {
		t.Fatalf("expected session-b history to be retained")
	}
	if got := broadcaster.GetEventHistory("session-c"); got == nil {
		t.Fatalf("expected session-c history to be retained")
	}
}

func TestEventBroadcasterExpiresSessionsByTTL(t *testing.T) {
	broadcaster := NewEventBroadcaster(WithSessionTTL(15 * time.Millisecond))

	broadcaster.OnEvent(makeToolEnvelope("session-ttl", time.Now()))
	time.Sleep(30 * time.Millisecond)

	if got := broadcaster.GetEventHistory("session-ttl"); got != nil {
		t.Fatalf("expected session history to expire by TTL")
	}
}

func makeToolEnvelope(sessionID string, ts time.Time) *domain.WorkflowEventEnvelope {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-"+sessionID, "", ts),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:1",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}
}

func TestEventBroadcasterSanitizesHugeWorkflowPayloadForHistory(t *testing.T) {
	store := &capturingHistoryStore{}
	broadcaster := NewEventBroadcaster(WithEventHistoryStore(store))

	hugeOutput := strings.Repeat("x", historyMaxStringBytes*4)
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess-large", "run-large", "", time.Now()),
		Version:   1,
		Event:     "workflow.lifecycle.updated",
		NodeKind:  "node",
		Payload: map[string]any{
			"workflow": map[string]any{
				"id": "run-large",
				"nodes": []any{
					map[string]any{
						"id":     "tool:1",
						"status": "succeeded",
						"output": hugeOutput,
					},
				},
			},
			"node": map[string]any{
				"id":     "tool:1",
				"status": "succeeded",
				"output": hugeOutput,
			},
		},
	}

	broadcaster.OnEvent(envelope)

	got := store.lastEvent()
	if got == nil {
		t.Fatalf("expected sanitized event to be stored")
	}
	storedEnvelope, ok := got.(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected WorkflowEventEnvelope, got %T", got)
	}

	workflow, ok := storedEnvelope.Payload["workflow"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow payload map, got %T", storedEnvelope.Payload["workflow"])
	}
	if gotCount, ok := workflow["nodes_count"].(int); !ok || gotCount != 1 {
		t.Fatalf("expected workflow.nodes_count=1, got %v", workflow["nodes_count"])
	}
	nodes, ok := workflow["nodes"].([]any)
	if !ok || len(nodes) != 1 {
		t.Fatalf("expected one sanitized workflow node, got %T len=%d", workflow["nodes"], len(nodes))
	}

	node, ok := nodes[0].(map[string]any)
	if !ok {
		t.Fatalf("expected sanitized node map, got %T", nodes[0])
	}
	preview, ok := node["output_preview"].(string)
	if !ok || preview == "" {
		t.Fatalf("expected output_preview on sanitized node, got %v", node["output_preview"])
	}
	if len(preview) > historyNodeOutputPreviewBytes+64 {
		t.Fatalf("expected bounded node output preview, got len=%d", len(preview))
	}

	payloadNode, ok := storedEnvelope.Payload["node"].(map[string]any)
	if !ok {
		t.Fatalf("expected sanitized payload.node map, got %T", storedEnvelope.Payload["node"])
	}
	payloadPreview, ok := payloadNode["output_preview"].(string)
	if !ok || payloadPreview == "" {
		t.Fatalf("expected output_preview in payload.node, got %v", payloadNode["output_preview"])
	}
	if len(payloadPreview) > historyNodeOutputPreviewBytes+64 {
		t.Fatalf("expected bounded payload.node preview, got len=%d", len(payloadPreview))
	}
}
