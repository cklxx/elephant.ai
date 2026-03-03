package kernel

import (
	"context"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// TestFileStore_RecoverStalePending_CancelsOrphanedPendingDispatches is the
// regression test for the stale-pending leak: dispatches enqueued but never
// claimed (e.g., executor crashed before ClaimDispatches ran) must be
// cancelled by RecoverStalePending after leaseDuration has elapsed.
func TestFileStore_RecoverStalePending_CancelsOrphanedPendingDispatches(t *testing.T) {
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	leaseDur := 30 * time.Minute
	dir := t.TempDir()
	store := NewFileStore(dir, leaseDur, 14*24*time.Hour)
	store.now = func() time.Time { return now }

	// Stale pending: created > leaseDuration ago, never claimed.
	stale := kerneldomain.Dispatch{
		DispatchID: "stale-p1",
		KernelID:   "k1",
		AgentID:    "audit-executor",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}
	// Fresh pending: created within leaseDuration — should be left alone.
	fresh := kerneldomain.Dispatch{
		DispatchID: "fresh-p2",
		KernelID:   "k1",
		AgentID:    "build-executor",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  now.Add(-10 * time.Minute),
		UpdatedAt:  now.Add(-10 * time.Minute),
	}
	// Different kernel: must not be touched.
	otherKernel := kerneldomain.Dispatch{
		DispatchID: "other-p3",
		KernelID:   "k2",
		AgentID:    "audit-executor",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  now.Add(-3 * time.Hour),
		UpdatedAt:  now.Add(-3 * time.Hour),
	}
	// Running dispatch — must not be touched by RecoverStalePending.
	running := kerneldomain.Dispatch{
		DispatchID: "running-d4",
		KernelID:   "k1",
		AgentID:    "build-executor",
		Status:     kerneldomain.DispatchRunning,
		CreatedAt:  now.Add(-1 * time.Hour),
		UpdatedAt:  now.Add(-1 * time.Hour),
	}

	store.dispatches[stale.DispatchID] = stale
	store.dispatches[fresh.DispatchID] = fresh
	store.dispatches[otherKernel.DispatchID] = otherKernel
	store.dispatches[running.DispatchID] = running

	recovered, err := store.RecoverStalePending(context.Background(), "k1")
	if err != nil {
		t.Fatalf("RecoverStalePending: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 stale pending cancelled, got %d", recovered)
	}

	// stale-p1 must be cancelled with an error message.
	d := store.dispatches["stale-p1"]
	if d.Status != kerneldomain.DispatchCancelled {
		t.Errorf("expected stale-p1 status=cancelled, got %v", d.Status)
	}
	if d.Error == "" {
		t.Error("expected stale-p1 to have an error message")
	}

	// fresh-p2 must remain pending (within lease window).
	if d := store.dispatches["fresh-p2"]; d.Status != kerneldomain.DispatchPending {
		t.Errorf("expected fresh-p2 to remain pending, got %v", d.Status)
	}

	// other-p3 (different kernel) must remain pending.
	if d := store.dispatches["other-p3"]; d.Status != kerneldomain.DispatchPending {
		t.Errorf("expected other-p3 (k2) to remain pending, got %v", d.Status)
	}

	// running-d4 must remain running (not touched by RecoverStalePending).
	if d := store.dispatches["running-d4"]; d.Status != kerneldomain.DispatchRunning {
		t.Errorf("expected running-d4 to remain running, got %v", d.Status)
	}
}

// TestFileStore_RecoverStalePending_ContextCancellation verifies ctx is respected.
func TestFileStore_RecoverStalePending_ContextCancellation(t *testing.T) {
	store := &FileStore{
		dispatches:    make(map[string]kerneldomain.Dispatch),
		leaseDuration: 30 * time.Minute,
		now:           time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.RecoverStalePending(ctx, "k1")
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestFileStore_RecoverStalePending_PersistAfterCancel verifies that cancelled
// dispatches are immediately persisted to disk (K-03 pattern): the cancel is
// durable even before the next retention prune cycle.
func TestFileStore_RecoverStalePending_PersistAfterCancel(t *testing.T) {
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	leaseDur := 30 * time.Minute
	// Use a very long retention so that the newly-cancelled dispatch is NOT
	// pruned immediately (UpdatedAt is reset to now, so it won't age out for
	// retentionDur). We only need to verify the CANCEL status is persisted.
	retentionDur := 14 * 24 * time.Hour
	dir := t.TempDir()
	store := NewFileStore(dir, leaseDur, retentionDur)
	store.now = func() time.Time { return now }

	stale := kerneldomain.Dispatch{
		DispatchID: "stale-gc",
		KernelID:   "k1",
		AgentID:    "audit-executor",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}
	store.dispatches[stale.DispatchID] = stale

	recovered, err := store.RecoverStalePending(context.Background(), "k1")
	if err != nil {
		t.Fatalf("RecoverStalePending: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 cancelled, got %d", recovered)
	}

	// In-memory: the dispatch should now be CANCELLED, not PENDING.
	if d := store.dispatches["stale-gc"]; d.Status != kerneldomain.DispatchCancelled {
		t.Errorf("expected in-memory status=cancelled, got %v", d.Status)
	}

	// Reload from disk and confirm the cancel was persisted (K-03: persist=true).
	fresh := NewFileStore(dir, leaseDur, retentionDur)
	fresh.now = func() time.Time { return now }
	if err := fresh.load(); err != nil {
		t.Fatalf("fresh load: %v", err)
	}
	d, exists := fresh.dispatches["stale-gc"]
	if !exists {
		t.Fatal("stale-gc not found on disk after cancel — it should be persisted with status=cancelled")
	}
	if d.Status != kerneldomain.DispatchCancelled {
		t.Errorf("expected persisted status=cancelled, got %v", d.Status)
	}
}

func TestFileStore_RecoverStalePending_PrunesTerminalAndPersists(t *testing.T) {
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	leaseDur := 30 * time.Minute
	retentionDur := 7 * 24 * time.Hour
	dir := t.TempDir()
	store := NewFileStore(dir, leaseDur, retentionDur)
	store.now = func() time.Time { return now }

	// Stale pending dispatch should be cancelled.
	stale := kerneldomain.Dispatch{
		DispatchID: "stale-p4",
		KernelID:   "k1",
		AgentID:    "audit-executor",
		Status:     kerneldomain.DispatchPending,
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}
	// Expired terminal record should be pruned during the same recover+persist path.
	oldTerminal := kerneldomain.Dispatch{
		DispatchID: "old-done",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(-15 * 24 * time.Hour),
		UpdatedAt:  now.Add(-15 * 24 * time.Hour),
	}
	store.dispatches[stale.DispatchID] = stale
	store.dispatches[oldTerminal.DispatchID] = oldTerminal

	if recovered, err := store.RecoverStalePending(context.Background(), "k1"); err != nil {
		t.Fatalf("RecoverStalePending: %v", err)
	} else if recovered != 1 {
		t.Fatalf("expected 1 recovered dispatch, got %d", recovered)
	}

	if got, ok := store.dispatches["stale-p4"]; !ok {
		t.Fatal("stale-p4 should remain in-memory after recovery")
	} else if got.Status != kerneldomain.DispatchCancelled {
		t.Fatalf("expected stale-p4 status=cancelled, got %v", got.Status)
	}
	if _, exists := store.dispatches["old-done"]; exists {
		t.Fatal("old-done should be pruned during RecoverStalePending")
	}

	fresh := NewFileStore(dir, leaseDur, retentionDur)
	fresh.now = func() time.Time { return now }
	if err := fresh.load(); err != nil {
		t.Fatalf("fresh load: %v", err)
	}

	if got, ok := fresh.dispatches["stale-p4"]; !ok {
		t.Fatal("stale-p4 should be persisted after RecoverStalePending")
	} else if got.Status != kerneldomain.DispatchCancelled {
		t.Fatalf("expected persisted stale-p4 status=cancelled, got %v", got.Status)
	}
	if _, exists := fresh.dispatches["old-done"]; exists {
		t.Fatal("reloaded state should not contain pruned old-done")
	}
}
