package kernel

import (
	"context"
	"fmt"
	"testing"
	"time"

	domain "alex/internal/domain/kernel"
)

func TestFileStore_SaveAndGet(t *testing.T) {
	store := newTestStore(t)

	d := domain.Dispatch{
		DispatchID: "d1",
		KernelID:   "k1",
		AgentName:  "agent-a",
		Status:     domain.DispatchPending,
	}
	if err := store.Save(context.Background(), d); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Get(context.Background(), "d1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AgentName != "agent-a" {
		t.Fatalf("got agent %q, want agent-a", got.AgentName)
	}
}

func TestFileStore_GetNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing dispatch")
	}
}

func TestFileStore_ListRecentByAgent(t *testing.T) {
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	store := newTestStoreWithTime(t, func() time.Time { return now })

	// Create 3 dispatches for agent-a, 1 for agent-b.
	for i := 0; i < 3; i++ {
		now = now.Add(time.Minute)
		store.now = func() time.Time { return now }
		_ = store.Save(context.Background(), domain.Dispatch{
			DispatchID: fmt.Sprintf("d-a-%d", i),
			KernelID:   "k1",
			AgentName:  "agent-a",
			Status:     domain.DispatchDone,
		})
	}
	_ = store.Save(context.Background(), domain.Dispatch{
		DispatchID: "d-b-0",
		KernelID:   "k1",
		AgentName:  "agent-b",
		Status:     domain.DispatchDone,
	})

	result, err := store.ListRecentByAgent(context.Background(), "k1", 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result["agent-a"]) != 2 {
		t.Fatalf("agent-a: got %d, want 2", len(result["agent-a"]))
	}
	if len(result["agent-b"]) != 1 {
		t.Fatalf("agent-b: got %d, want 1", len(result["agent-b"]))
	}
}

func TestFileStore_RecoverStaleRunning(t *testing.T) {
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	store := newTestStoreWithTime(t, func() time.Time { return now })
	store.leaseDuration = 10 * time.Minute

	// Running dispatch from 15 minutes ago — should be recovered.
	store.dispatches["stale"] = domain.Dispatch{
		DispatchID: "stale",
		KernelID:   "k1",
		Status:     domain.DispatchRunning,
		UpdatedAt:  now.Add(-15 * time.Minute),
	}
	// Running dispatch from 5 minutes ago — should NOT be recovered.
	store.dispatches["fresh"] = domain.Dispatch{
		DispatchID: "fresh",
		KernelID:   "k1",
		Status:     domain.DispatchRunning,
		UpdatedAt:  now.Add(-5 * time.Minute),
	}
	// Done dispatch from 15 minutes ago — should NOT be recovered.
	store.dispatches["done-old"] = domain.Dispatch{
		DispatchID: "done-old",
		KernelID:   "k1",
		Status:     domain.DispatchDone,
		UpdatedAt:  now.Add(-15 * time.Minute),
	}

	recovered, err := store.RecoverStaleRunning(context.Background(), "k1")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered %d, want 1", recovered)
	}
	if store.dispatches["stale"].Status != domain.DispatchFailed {
		t.Fatalf("stale dispatch status = %s, want failed", store.dispatches["stale"].Status)
	}
	if store.dispatches["fresh"].Status != domain.DispatchRunning {
		t.Fatal("fresh dispatch should still be running")
	}
}

func TestFileStore_PurgeTerminalDispatches(t *testing.T) {
	now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	store := newTestStoreWithTime(t, func() time.Time { return now })
	store.retentionPeriod = 24 * time.Hour

	// Old terminal — should be purged.
	store.dispatches["old-done"] = domain.Dispatch{
		DispatchID: "old-done",
		KernelID:   "k1",
		Status:     domain.DispatchDone,
		UpdatedAt:  now.Add(-48 * time.Hour),
	}
	// Recent terminal — should be kept.
	store.dispatches["recent-done"] = domain.Dispatch{
		DispatchID: "recent-done",
		KernelID:   "k1",
		Status:     domain.DispatchDone,
		UpdatedAt:  now.Add(-1 * time.Hour),
	}
	// Old but still running — should NOT be purged.
	store.dispatches["old-running"] = domain.Dispatch{
		DispatchID: "old-running",
		KernelID:   "k1",
		Status:     domain.DispatchRunning,
		UpdatedAt:  now.Add(-48 * time.Hour),
	}
	// Old terminal but different kernel — should NOT be purged.
	store.dispatches["other-kernel"] = domain.Dispatch{
		DispatchID: "other-kernel",
		KernelID:   "k2",
		Status:     domain.DispatchFailed,
		UpdatedAt:  now.Add(-48 * time.Hour),
	}

	purged, err := store.PurgeTerminalDispatches(context.Background(), "k1")
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged %d, want 1", purged)
	}
	if _, ok := store.dispatches["old-done"]; ok {
		t.Fatal("old-done should be purged")
	}
	if _, ok := store.dispatches["recent-done"]; !ok {
		t.Fatal("recent-done should be kept")
	}
	if _, ok := store.dispatches["old-running"]; !ok {
		t.Fatal("old-running should be kept")
	}
	if _, ok := store.dispatches["other-kernel"]; !ok {
		t.Fatal("other-kernel should be kept")
	}
}

func TestFileStore_PurgeNothingSkipsPersist(t *testing.T) {
	now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	store := newTestStoreWithTime(t, func() time.Time { return now })
	store.retentionPeriod = 24 * time.Hour

	store.dispatches["recent"] = domain.Dispatch{
		DispatchID: "recent",
		KernelID:   "k1",
		Status:     domain.DispatchDone,
		UpdatedAt:  now.Add(-1 * time.Hour),
	}

	purged, err := store.PurgeTerminalDispatches(context.Background(), "k1")
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 0 {
		t.Fatalf("purged %d, want 0", purged)
	}
}

func TestFileStore_DefaultLeaseDuration(t *testing.T) {
	store := NewFileStore(FileStoreConfig{Dir: t.TempDir()})
	if store.leaseDuration != defaultLeaseDuration {
		t.Fatalf("default lease = %v, want %v", store.leaseDuration, defaultLeaseDuration)
	}
	if store.leaseDuration != 30*time.Minute {
		t.Fatalf("default lease should be 30 minutes, got %v", store.leaseDuration)
	}
}

func TestFileStore_DefaultRetentionPeriod(t *testing.T) {
	store := NewFileStore(FileStoreConfig{Dir: t.TempDir()})
	if store.retentionPeriod != defaultRetentionPeriod {
		t.Fatalf("default retention = %v, want %v", store.retentionPeriod, defaultRetentionPeriod)
	}
}

func TestFileStore_LoadPersistRoundtrip(t *testing.T) {
	dir := t.TempDir()
	store1 := NewFileStore(FileStoreConfig{Dir: dir})

	d := domain.Dispatch{
		DispatchID: "d1",
		KernelID:   "k1",
		AgentName:  "agent-a",
		Status:     domain.DispatchDone,
		Summary:    "completed ok",
	}
	if err := store1.Save(context.Background(), d); err != nil {
		t.Fatalf("save: %v", err)
	}

	// New store loading from same directory.
	store2 := NewFileStore(FileStoreConfig{Dir: dir})
	if err := store2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	got, err := store2.Get(context.Background(), "d1")
	if err != nil {
		t.Fatalf("get from reloaded store: %v", err)
	}
	if got.Summary != "completed ok" {
		t.Fatalf("summary = %q, want %q", got.Summary, "completed ok")
	}
}

// --- helpers ---

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	return NewFileStore(FileStoreConfig{Dir: t.TempDir()})
}

func newTestStoreWithTime(t *testing.T, nowFn func() time.Time) *FileStore {
	t.Helper()
	s := NewFileStore(FileStoreConfig{Dir: t.TempDir()})
	s.now = nowFn
	return s
}
