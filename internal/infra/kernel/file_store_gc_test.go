package kernel

import (
	"context"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// TestFileStore_PruneLocked_RemovesExpiredTerminalDispatches verifies that terminal
// dispatches older than retentionDuration are removed from the in-memory map.
func TestFileStore_PruneLocked_RemovesExpiredTerminalDispatches(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	retention := 7 * 24 * time.Hour
	store := &FileStore{
		dispatches:        make(map[string]kerneldomain.Dispatch),
		retentionDuration: retention,
		now:               func() time.Time { return now },
	}

	// Old terminal dispatch (should be pruned)
	oldDone := kerneldomain.Dispatch{
		DispatchID: "old-done",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(-8 * 24 * time.Hour),
		UpdatedAt:  now.Add(-8 * 24 * time.Hour),
	}
	// Recent terminal dispatch (should survive)
	recentFailed := kerneldomain.Dispatch{
		DispatchID: "recent-failed",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchFailed,
		CreatedAt:  now.Add(-2 * 24 * time.Hour),
		UpdatedAt:  now.Add(-2 * 24 * time.Hour),
	}
	// Active dispatch (should never be pruned regardless of age)
	activeRunning := kerneldomain.Dispatch{
		DispatchID: "active-running",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchRunning,
		CreatedAt:  now.Add(-10 * 24 * time.Hour),
		UpdatedAt:  now.Add(-10 * 24 * time.Hour),
	}

	store.dispatches[oldDone.DispatchID] = oldDone
	store.dispatches[recentFailed.DispatchID] = recentFailed
	store.dispatches[activeRunning.DispatchID] = activeRunning

	removed, err := store.pruneLocked(context.Background(), now, false)
	if err != nil {
		t.Fatalf("pruneLocked: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 dispatch pruned, got %d", removed)
	}
	if _, exists := store.dispatches["old-done"]; exists {
		t.Error("expected old-done to be pruned but it still exists")
	}
	if _, exists := store.dispatches["recent-failed"]; !exists {
		t.Error("expected recent-failed to survive pruning")
	}
	if _, exists := store.dispatches["active-running"]; !exists {
		t.Error("expected active-running to survive pruning (non-terminal)")
	}
}

// TestFileStore_PruneLocked_ZeroRetentionSkips verifies that zero/negative
// retentionDuration disables pruning entirely.
func TestFileStore_PruneLocked_ZeroRetentionSkips(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	store := &FileStore{
		dispatches:        make(map[string]kerneldomain.Dispatch),
		retentionDuration: 0, // disabled
		now:               func() time.Time { return now },
	}
	store.dispatches["old"] = kerneldomain.Dispatch{
		DispatchID: "old",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(-365 * 24 * time.Hour),
		UpdatedAt:  now.Add(-365 * 24 * time.Hour),
	}

	removed, err := store.pruneLocked(context.Background(), now, false)
	if err != nil {
		t.Fatalf("pruneLocked: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 pruned when retention=0, got %d", removed)
	}
	if _, exists := store.dispatches["old"]; !exists {
		t.Error("expected old dispatch to remain when retention is disabled")
	}
}

// TestFileStore_PruneLocked_UsesMostRecentTimestamp verifies that pruneDeadline
// uses UpdatedAt when it is newer than CreatedAt.
func TestFileStore_PruneLocked_UsesMostRecentTimestamp(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	retention := 7 * 24 * time.Hour
	store := &FileStore{
		dispatches:        make(map[string]kerneldomain.Dispatch),
		retentionDuration: retention,
		now:               func() time.Time { return now },
	}

	// CreatedAt is old but UpdatedAt is recent — should NOT be pruned.
	d := kerneldomain.Dispatch{
		DispatchID: "recently-updated",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchDone,
		CreatedAt:  now.Add(-10 * 24 * time.Hour), // old
		UpdatedAt:  now.Add(-1 * time.Hour),        // very recent
	}
	store.dispatches[d.DispatchID] = d

	removed, err := store.pruneLocked(context.Background(), now, false)
	if err != nil {
		t.Fatalf("pruneLocked: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 pruned (UpdatedAt is recent), got %d", removed)
	}
	if _, exists := store.dispatches["recently-updated"]; !exists {
		t.Error("recently-updated should survive because UpdatedAt is within retention window")
	}
}

// TestFileStore_RecoverStaleRunning_MarksStuckDispatchesAsFailed verifies that
// dispatches stuck in "running" beyond leaseDuration are transitioned to failed.
func TestFileStore_RecoverStaleRunning_MarksStuckDispatchesAsFailed(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	leaseDur := 30 * time.Minute
	// Use a real temp dir so persistLocked can write the file.
	dir := t.TempDir()
	store := NewFileStore(dir, leaseDur, 14*24*time.Hour)
	store.now = func() time.Time { return now }

	// Stale running dispatch (last updated > leaseDuration ago).
	stale := kerneldomain.Dispatch{
		DispatchID: "stale-d1",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchRunning,
		UpdatedAt:  now.Add(-45 * time.Minute),
	}
	// Fresh running dispatch (last updated within leaseDuration).
	fresh := kerneldomain.Dispatch{
		DispatchID: "fresh-d2",
		KernelID:   "k1",
		Status:     kerneldomain.DispatchRunning,
		UpdatedAt:  now.Add(-5 * time.Minute),
	}
	// Already-done dispatch for another kernel (should be untouched).
	otherKernel := kerneldomain.Dispatch{
		DispatchID: "other-d3",
		KernelID:   "k2",
		Status:     kerneldomain.DispatchRunning,
		UpdatedAt:  now.Add(-2 * time.Hour),
	}

	store.dispatches[stale.DispatchID] = stale
	store.dispatches[fresh.DispatchID] = fresh
	store.dispatches[otherKernel.DispatchID] = otherKernel

	recovered, err := store.RecoverStaleRunning(context.Background(), "k1")
	if err != nil {
		t.Fatalf("RecoverStaleRunning: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 stale dispatch recovered, got %d", recovered)
	}

	// stale-d1 should be failed.
	if d := store.dispatches["stale-d1"]; d.Status != kerneldomain.DispatchFailed {
		t.Errorf("expected stale-d1 status=failed, got %v", d.Status)
	}
	if d := store.dispatches["stale-d1"]; d.Error == "" {
		t.Error("expected stale-d1 to have an error message")
	}
	// fresh-d2 should remain running.
	if d := store.dispatches["fresh-d2"]; d.Status != kerneldomain.DispatchRunning {
		t.Errorf("expected fresh-d2 to remain running, got %v", d.Status)
	}
	// other-d3 (different kernel) should remain running.
	if d := store.dispatches["other-d3"]; d.Status != kerneldomain.DispatchRunning {
		t.Errorf("expected other-d3 (k2) to remain running, got %v", d.Status)
	}
}

// TestFileStore_RecoverStaleRunning_ContextCancellation verifies context is respected.
func TestFileStore_RecoverStaleRunning_ContextCancellation(t *testing.T) {
	store := &FileStore{
		dispatches:    make(map[string]kerneldomain.Dispatch),
		leaseDuration: 30 * time.Minute,
		now:           time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.RecoverStaleRunning(ctx, "k1")
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestFileStore_NewFileStore_Defaults verifies that NewFileStore applies safe defaults.
func TestFileStore_NewFileStore_Defaults(t *testing.T) {
	store := NewFileStore(t.TempDir(), 0, 0)
	if store.leaseDuration != 30*time.Minute {
		t.Errorf("expected leaseDuration default=30m, got %v", store.leaseDuration)
	}
	if store.retentionDuration != 14*24*time.Hour {
		t.Errorf("expected retentionDuration default=14d, got %v", store.retentionDuration)
	}
}
