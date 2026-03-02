package kernel

import (
	"context"
	"reflect"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

func TestFileStore_ListRecentByAgent_PrefersUpdatedAtOverCreatedAt(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	store := &FileStore{
		dispatches: make(map[string]kerneldomain.Dispatch),
		now:        func() time.Time { return now },
	}

	store.dispatches["old"] = kerneldomain.Dispatch{
		DispatchID: "old",
		KernelID:   "k1",
		AgentID:    "build-executor",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(2 * time.Minute),
		UpdatedAt:  now.Add(2 * time.Minute),
	}
	store.dispatches["new"] = kerneldomain.Dispatch{
		DispatchID: "new",
		KernelID:   "k1",
		AgentID:    "build-executor",
		Status:     kerneldomain.DispatchRunning,
		CreatedAt:  now,
		UpdatedAt:  now.Add(3 * time.Minute),
	}

	recent, err := store.ListRecentByAgent(context.Background(), "k1")
	if err != nil {
		t.Fatalf("ListRecentByAgent: %v", err)
	}

	got, ok := recent["build-executor"]
	if !ok {
		t.Fatal("expected recent dispatch for build-executor")
	}
	if got.DispatchID != "new" {
		t.Fatalf("expected dispatch 'new' (newer updated_at), got %q", got.DispatchID)
	}
}

func TestFileStore_ListRecentByAgent_TieBreaksByCreatedAtThenDispatchID(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	store := &FileStore{
		dispatches: make(map[string]kerneldomain.Dispatch),
		now:        func() time.Time { return now },
	}

	store.dispatches["a"] = kerneldomain.Dispatch{
		DispatchID: "a",
		KernelID:   "k1",
		AgentID:    "research-executor",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	store.dispatches["b"] = kerneldomain.Dispatch{
		DispatchID: "b",
		KernelID:   "k1",
		AgentID:    "research-executor",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	recent, err := store.ListRecentByAgent(context.Background(), "k1")
	if err != nil {
		t.Fatalf("ListRecentByAgent: %v", err)
	}

	got, ok := recent["research-executor"]
	if !ok {
		t.Fatal("expected recent dispatch for research-executor")
	}
	if got.DispatchID != "b" {
		t.Fatalf("expected deterministic final tie-break to select 'b', got %q", got.DispatchID)
	}
}

func TestFileStore_ListActiveDispatches_DeterministicTieBreakByDispatchID(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	store := &FileStore{
		dispatches: make(map[string]kerneldomain.Dispatch),
		now:        func() time.Time { return now },
	}

	createdAt := now.Add(-time.Minute)
	store.dispatches["z"] = kerneldomain.Dispatch{
		DispatchID: "z",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
	store.dispatches["a"] = kerneldomain.Dispatch{
		DispatchID: "a",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
	store.dispatches["done"] = kerneldomain.Dispatch{
		DispatchID: "done",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}

	out, err := store.ListActiveDispatches(context.Background(), "k1")
	if err != nil {
		t.Fatalf("ListActiveDispatches: %v", err)
	}

	gotIDs := make([]string, 0, len(out))
	for _, d := range out {
		gotIDs = append(gotIDs, d.DispatchID)
	}
	wantIDs := []string{"a", "z"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("unexpected active dispatch ordering: got %v want %v", gotIDs, wantIDs)
	}
}

func TestFileStore_ListActiveDispatches_ContextCanceled(t *testing.T) {
	store := &FileStore{dispatches: make(map[string]kerneldomain.Dispatch)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.ListActiveDispatches(ctx, "k1")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestFileStore_PruneLocked_RemovesExpiredTerminalDispatches verifies that
// terminal dispatches older than the retention window are pruned from memory.
