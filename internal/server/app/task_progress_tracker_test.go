package app

import (
	"context"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	serverPorts "alex/internal/server/ports"
)

func newTestTracker(t *testing.T) (*TaskProgressTracker, serverPorts.TaskStore) {
	t.Helper()
	store := NewInMemoryTaskStore()
	return NewTaskProgressTracker(store), store
}

func TestTrackerUpdatesOnNodeStarted(t *testing.T) {
	tracker, store := newTestTracker(t)

	ctx := context.Background()
	task, err := store.Create(ctx, "session-1", "test task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	tracker.RegisterRunSession("session-1", task.ID)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", task.ID, "", time.Now()),
		Event:     types.EventNodeStarted,
		Payload:   map[string]any{"iteration": 2},
	}
	tracker.OnEvent(event)

	got, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentIteration != 2 {
		t.Fatalf("expected iteration=2, got %d", got.CurrentIteration)
	}
}

func TestTrackerUpdatesOnNodeCompleted(t *testing.T) {
	tracker, store := newTestTracker(t)

	ctx := context.Background()
	task, err := store.Create(ctx, "session-1", "test task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	tracker.RegisterRunSession("session-1", task.ID)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", task.ID, "", time.Now()),
		Event:     types.EventNodeCompleted,
		Payload:   map[string]any{"iteration": 3, "tokens_used": 500},
	}
	tracker.OnEvent(event)

	got, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TokensUsed != 500 {
		t.Fatalf("expected tokens_used=500, got %d", got.TokensUsed)
	}
}

func TestTrackerUpdatesOnResultFinal(t *testing.T) {
	tracker, store := newTestTracker(t)

	ctx := context.Background()
	task, err := store.Create(ctx, "session-1", "test task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	tracker.RegisterRunSession("session-1", task.ID)

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", task.ID, "", time.Now()),
		Event:     types.EventResultFinal,
		Payload:   map[string]any{"total_iterations": 5, "total_tokens": 1200},
	}
	tracker.OnEvent(event)

	got, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentIteration != 5 {
		t.Fatalf("expected iteration=5, got %d", got.CurrentIteration)
	}
	if got.TokensUsed != 1200 {
		t.Fatalf("expected tokens=1200, got %d", got.TokensUsed)
	}
}

func TestTrackerIgnoresUnregisteredSessions(t *testing.T) {
	tracker, store := newTestTracker(t)

	ctx := context.Background()
	task, err := store.Create(ctx, "session-1", "test task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// Do NOT register session

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", task.ID, "", time.Now()),
		Event:     types.EventNodeCompleted,
		Payload:   map[string]any{"iteration": 3, "tokens_used": 500},
	}
	tracker.OnEvent(event)

	got, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TokensUsed != 0 {
		t.Fatalf("expected tokens_used=0 for unregistered session, got %d", got.TokensUsed)
	}
}

func TestTrackerUnregisterStopsTracking(t *testing.T) {
	tracker, store := newTestTracker(t)

	ctx := context.Background()
	task, err := store.Create(ctx, "session-1", "test task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	tracker.RegisterRunSession("session-1", task.ID)
	tracker.UnregisterRunSession("session-1")

	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", task.ID, "", time.Now()),
		Event:     types.EventNodeCompleted,
		Payload:   map[string]any{"iteration": 3, "tokens_used": 500},
	}
	tracker.OnEvent(event)

	got, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TokensUsed != 0 {
		t.Fatalf("expected tokens_used=0 after unregister, got %d", got.TokensUsed)
	}
}

func TestGetActiveRunID(t *testing.T) {
	tracker, _ := newTestTracker(t)

	// No run registered → empty string
	if got := tracker.GetActiveRunID("session-1"); got != "" {
		t.Fatalf("expected empty string for unregistered session, got %q", got)
	}

	// Register run → returns run ID
	tracker.RegisterRunSession("session-1", "run-abc")
	if got := tracker.GetActiveRunID("session-1"); got != "run-abc" {
		t.Fatalf("expected run-abc, got %q", got)
	}

	// Unregister → returns empty string again
	tracker.UnregisterRunSession("session-1")
	if got := tracker.GetActiveRunID("session-1"); got != "" {
		t.Fatalf("expected empty string after unregister, got %q", got)
	}
}
