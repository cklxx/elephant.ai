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

func TestFileStore_MarkDispatchDone_ClearsHeavyweightFields(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, 30*time.Minute, 24*time.Hour)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	dispatches, err := store.EnqueueDispatches(ctx, "k1", "c1", []kerneldomain.DispatchSpec{
		{AgentID: "agent-a", Prompt: "large prompt content here", Priority: 10,
			Team: &kerneldomain.TeamDispatchSpec{Template: "t", Goal: "g"}},
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}
	dID := dispatches[0].DispatchID

	if err := store.MarkDispatchRunning(ctx, dID); err != nil {
		t.Fatalf("MarkDispatchRunning: %v", err)
	}
	if err := store.MarkDispatchDone(ctx, dID, "task-1"); err != nil {
		t.Fatalf("MarkDispatchDone: %v", err)
	}

	store.mu.RLock()
	d := store.dispatches[dID]
	store.mu.RUnlock()

	if d.Prompt != "" {
		t.Errorf("expected Prompt to be cleared after Done, got %q", d.Prompt)
	}
	if d.Team != nil {
		t.Errorf("expected Team to be nil after Done, got %+v", d.Team)
	}
}

func TestFileStore_MarkDispatchFailed_ClearsHeavyweightFields(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, 30*time.Minute, 24*time.Hour)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	dispatches, err := store.EnqueueDispatches(ctx, "k1", "c1", []kerneldomain.DispatchSpec{
		{AgentID: "agent-a", Prompt: "another large prompt", Priority: 10,
			Team: &kerneldomain.TeamDispatchSpec{Template: "t", Goal: "g"}},
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}
	dID := dispatches[0].DispatchID

	if err := store.MarkDispatchRunning(ctx, dID); err != nil {
		t.Fatalf("MarkDispatchRunning: %v", err)
	}
	if err := store.MarkDispatchFailed(ctx, dID, "timeout"); err != nil {
		t.Fatalf("MarkDispatchFailed: %v", err)
	}

	store.mu.RLock()
	d := store.dispatches[dID]
	store.mu.RUnlock()

	if d.Prompt != "" {
		t.Errorf("expected Prompt to be cleared after Failed, got %q", d.Prompt)
	}
	if d.Team != nil {
		t.Errorf("expected Team to be nil after Failed, got %+v", d.Team)
	}
}

func TestFileStore_PruneExpired_RemovesOldTerminalDispatches(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	store := &FileStore{
		dispatches:        make(map[string]kerneldomain.Dispatch),
		filePath:          t.TempDir() + "/dispatches.json",
		retentionDuration: 24 * time.Hour,
		now:               func() time.Time { return now },
	}

	// Old terminal dispatch — should be pruned.
	store.dispatches["old-done"] = kerneldomain.Dispatch{
		DispatchID: "old-done",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(-48 * time.Hour),
		UpdatedAt:  now.Add(-48 * time.Hour),
	}
	// Recent terminal dispatch — should remain.
	store.dispatches["new-done"] = kerneldomain.Dispatch{
		DispatchID: "new-done",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(-1 * time.Hour),
		UpdatedAt:  now.Add(-1 * time.Hour),
	}
	// Active dispatch — should remain.
	store.dispatches["running"] = kerneldomain.Dispatch{
		DispatchID: "running",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchRunning,
		CreatedAt:  now.Add(-48 * time.Hour),
		UpdatedAt:  now.Add(-48 * time.Hour),
	}

	pruned, err := store.PruneExpired(context.Background())
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", pruned)
	}
	if _, ok := store.dispatches["old-done"]; ok {
		t.Fatal("expected old-done to be pruned")
	}
	if _, ok := store.dispatches["new-done"]; !ok {
		t.Fatal("expected new-done to remain")
	}
	if _, ok := store.dispatches["running"]; !ok {
		t.Fatal("expected running to remain")
	}
}

// TestFileStore_PruneLocked_RemovesExpiredTerminalDispatches verifies that
// terminal dispatches older than the retention window are pruned from memory.
