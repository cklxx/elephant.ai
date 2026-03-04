package app

import (
	"context"
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func TestEventCausalityRoundTrip(t *testing.T) {
	dir := t.TempDir()

	store := NewFileEventHistoryStore(dir)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// Create an event with specific correlationID and causationID.
	now := time.Now().Truncate(time.Millisecond) // truncate for JSON round-trip fidelity
	base := domain.NewBaseEventFull(
		agent.LevelCore,
		"sess-causality",
		"run-001",
		"parent-run-001",
		"corr-123",
		"cause-456",
		42,
		now,
	)
	event := domain.NewEvent(types.EventNodeStarted, base)
	event.Data.Content = "test content"

	// Append the event to the store.
	if err := store.Append(ctx, event); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Create a NEW store instance from the same directory (simulates restart).
	store2 := NewFileEventHistoryStore(dir)

	// Replay events and verify causality fields survived the round-trip.
	var replayed []agent.AgentEvent
	err := store2.Stream(ctx, EventHistoryFilter{SessionID: "sess-causality"}, func(e agent.AgentEvent) error {
		replayed = append(replayed, e)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	if len(replayed) != 1 {
		t.Fatalf("expected 1 replayed event, got %d", len(replayed))
	}

	got := replayed[0]
	if got.GetCorrelationID() != "corr-123" {
		t.Errorf("correlationID: got %q, want %q", got.GetCorrelationID(), "corr-123")
	}
	if got.GetCausationID() != "cause-456" {
		t.Errorf("causationID: got %q, want %q", got.GetCausationID(), "cause-456")
	}
	if got.GetSessionID() != "sess-causality" {
		t.Errorf("sessionID: got %q, want %q", got.GetSessionID(), "sess-causality")
	}
	if got.GetRunID() != "run-001" {
		t.Errorf("runID: got %q, want %q", got.GetRunID(), "run-001")
	}
	if got.GetParentRunID() != "parent-run-001" {
		t.Errorf("parentRunID: got %q, want %q", got.GetParentRunID(), "parent-run-001")
	}
	if got.GetSeq() != 42 {
		t.Errorf("seq: got %d, want %d", got.GetSeq(), 42)
	}
}

func TestEnvelopeCausalityRoundTrip(t *testing.T) {
	dir := t.TempDir()

	store := NewFileEventHistoryStore(dir)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	now := time.Now().Truncate(time.Millisecond)
	base := domain.NewBaseEventFull(
		agent.LevelSubagent,
		"sess-env-causality",
		"run-env-001",
		"parent-env-001",
		"corr-env-789",
		"cause-env-012",
		7,
		now,
	)

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent:  base,
		Version:    1,
		Event:      "workflow.tool.completed",
		WorkflowID: "wf-1",
		RunID:      "run-env-001",
		NodeID:     "bash:1",
		NodeKind:   "tool",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}

	if err := store.Append(ctx, envelope); err != nil {
		t.Fatalf("Append envelope: %v", err)
	}

	// Read back from a fresh store instance.
	store2 := NewFileEventHistoryStore(dir)
	var replayed []agent.AgentEvent
	err := store2.Stream(ctx, EventHistoryFilter{SessionID: "sess-env-causality"}, func(e agent.AgentEvent) error {
		replayed = append(replayed, e)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(replayed) != 1 {
		t.Fatalf("expected 1 replayed event, got %d", len(replayed))
	}

	got := replayed[0]
	if got.GetCorrelationID() != "corr-env-789" {
		t.Errorf("envelope correlationID: got %q, want %q", got.GetCorrelationID(), "corr-env-789")
	}
	if got.GetCausationID() != "cause-env-012" {
		t.Errorf("envelope causationID: got %q, want %q", got.GetCausationID(), "cause-env-012")
	}
	if got.GetAgentLevel() != agent.LevelSubagent {
		t.Errorf("envelope agentLevel: got %q, want %q", got.GetAgentLevel(), agent.LevelSubagent)
	}
}
