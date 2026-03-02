package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// newTestStore returns an in-memory FileStore wired to a temp dir with
// a fixed clock.  The caller must clean up the returned dir.
func newTestStore(t *testing.T, now time.Time) (*FileStore, string) {
	t.Helper()
	dir := t.TempDir()
	store := &FileStore{
		dispatches:        make(map[string]kerneldomain.Dispatch),
		filePath:          filepath.Join(dir, "dispatches.json"),
		leaseDuration:     5 * time.Minute,
		retentionDuration: 14 * 24 * time.Hour,
		now:               func() time.Time { return now },
	}
	return store, dir
}

func newSpec(agentID, prompt string) kerneldomain.DispatchSpec {
	return kerneldomain.DispatchSpec{
		AgentID:  agentID,
		Prompt:   prompt,
		Priority: 0,
		Kind:     kerneldomain.DispatchKindAgent,
	}
}

// ---------------------------------------------------------------------------
// EnsureSchema
// ---------------------------------------------------------------------------

func TestFileStore_EnsureSchema_CreatesFileAndLoads(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)

	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(store.filePath)); err != nil {
		t.Fatalf("expected dispatch dir to exist: %v", err)
	}
}

func TestFileStore_EnsureSchema_CancelledContext(t *testing.T) {
	now := time.Now()
	store, _ := newTestStore(t, now)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := store.EnsureSchema(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

// ---------------------------------------------------------------------------
// EnqueueDispatches
// ---------------------------------------------------------------------------

func TestFileStore_Enqueue_Basic(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	specs := []kerneldomain.DispatchSpec{
		newSpec("agent-a", "do thing A"),
		newSpec("agent-b", "do thing B"),
	}
	created, err := store.EnqueueDispatches(context.Background(), "k1", "c1", specs)
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 dispatches, got %d", len(created))
	}
	for _, d := range created {
		if d.Status != kerneldomain.DispatchPending {
			t.Errorf("expected pending status, got %q", d.Status)
		}
		if d.KernelID != "k1" {
			t.Errorf("expected kernelID k1, got %q", d.KernelID)
		}
	}
}

func TestFileStore_Enqueue_EmptySpecs(t *testing.T) {
	now := time.Now()
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	created, err := store.EnqueueDispatches(context.Background(), "k1", "c1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected 0 dispatches, got %d", len(created))
	}
}

func TestFileStore_Enqueue_CancelledContext(t *testing.T) {
	now := time.Now()
	store, _ := newTestStore(t, now)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := store.EnqueueDispatches(ctx, "k1", "c1", []kerneldomain.DispatchSpec{newSpec("x", "y")})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

func TestFileStore_Enqueue_PersistsAcrossReload(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, dir := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	_, err := store.EnqueueDispatches(context.Background(), "k1", "c1", []kerneldomain.DispatchSpec{
		newSpec("agent-persist", "persist me"),
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}

	// Reload from disk
	store2 := &FileStore{
		dispatches:        make(map[string]kerneldomain.Dispatch),
		filePath:          filepath.Join(dir, "dispatches.json"),
		leaseDuration:     5 * time.Minute,
		retentionDuration: 14 * 24 * time.Hour,
		now:               func() time.Time { return now },
	}
	if err := store2.load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(store2.dispatches) != 1 {
		t.Fatalf("expected 1 dispatch after reload, got %d", len(store2.dispatches))
	}
}

// ---------------------------------------------------------------------------
// ClaimDispatches
// ---------------------------------------------------------------------------

func TestFileStore_Claim_Basic(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	_, err := store.EnqueueDispatches(context.Background(), "k1", "c1", []kerneldomain.DispatchSpec{
		newSpec("worker", "task1"),
		newSpec("worker", "task2"),
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}

	claimed, err := store.ClaimDispatches(context.Background(), "k1", "w1", 10)
	if err != nil {
		t.Fatalf("ClaimDispatches: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed, got %d", len(claimed))
	}
	for _, d := range claimed {
		if d.LeaseOwner != "w1" {
			t.Errorf("expected lease owner w1, got %q", d.LeaseOwner)
		}
		if d.LeaseUntil == nil {
			t.Error("expected LeaseUntil to be set")
		}
	}
}

func TestFileStore_Claim_ZeroLimit(t *testing.T) {
	now := time.Now()
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	claimed, err := store.ClaimDispatches(context.Background(), "k1", "w1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("expected 0, got %d", len(claimed))
	}
}

func TestFileStore_Claim_DoesNotClaimOtherKernel(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	_, err := store.EnqueueDispatches(context.Background(), "other-kernel", "c1", []kerneldomain.DispatchSpec{
		newSpec("agent-x", "task"),
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}

	claimed, err := store.ClaimDispatches(context.Background(), "k1", "w1", 10)
	if err != nil {
		t.Fatalf("ClaimDispatches: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("expected 0 cross-kernel claims, got %d", len(claimed))
	}
}

func TestFileStore_Claim_RespectsPriorityOrder(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	specs := []kerneldomain.DispatchSpec{
		{AgentID: "a-low", Prompt: "low", Priority: 1, Kind: kerneldomain.DispatchKindAgent},
		{AgentID: "a-high", Prompt: "high", Priority: 10, Kind: kerneldomain.DispatchKindAgent},
	}
	_, err := store.EnqueueDispatches(context.Background(), "k1", "c1", specs)
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}

	claimed, err := store.ClaimDispatches(context.Background(), "k1", "w1", 1)
	if err != nil {
		t.Fatalf("ClaimDispatches: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected 1, got %d", len(claimed))
	}
	if claimed[0].AgentID != "a-high" {
		t.Errorf("expected high-priority agent first, got %q", claimed[0].AgentID)
	}
}

func TestFileStore_Claim_DoesNotReclaimActiveLease(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	_, err := store.EnqueueDispatches(context.Background(), "k1", "c1", []kerneldomain.DispatchSpec{
		newSpec("agent-x", "task"),
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}

	// First claim
	claimed1, err := store.ClaimDispatches(context.Background(), "k1", "w1", 10)
	if err != nil || len(claimed1) != 1 {
		t.Fatalf("first claim: err=%v n=%d", err, len(claimed1))
	}

	// Second claim with lease still active — should return nothing
	claimed2, err := store.ClaimDispatches(context.Background(), "k1", "w2", 10)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(claimed2) != 0 {
		t.Fatalf("expected 0 reclaims while lease active, got %d", len(claimed2))
	}
}

func TestFileStore_Claim_ReclaimsExpiredLease(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	_, err := store.EnqueueDispatches(context.Background(), "k1", "c1", []kerneldomain.DispatchSpec{
		newSpec("agent-x", "task"),
	})
	if err != nil {
		t.Fatalf("EnqueueDispatches: %v", err)
	}

	// Claim with w1
	_, err = store.ClaimDispatches(context.Background(), "k1", "w1", 10)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	// Advance clock past lease expiry
	store.now = func() time.Time { return now.Add(10 * time.Minute) }

	// Second claim — lease expired, should succeed
	claimed2, err := store.ClaimDispatches(context.Background(), "k1", "w2", 10)
	if err != nil {
		t.Fatalf("claim after expiry: %v", err)
	}
	if len(claimed2) != 1 {
		t.Fatalf("expected 1 reclaim after expiry, got %d", len(claimed2))
	}
}

// ---------------------------------------------------------------------------
// MarkDispatchRunning
// ---------------------------------------------------------------------------

func TestFileStore_MarkRunning_Basic(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	created, err := store.EnqueueDispatches(context.Background(), "k1", "c1", []kerneldomain.DispatchSpec{
		newSpec("agent-x", "task"),
	})
	if err != nil || len(created) != 1 {
		t.Fatalf("EnqueueDispatches: err=%v n=%d", err, len(created))
	}
	id := created[0].DispatchID

	if err := store.MarkDispatchRunning(context.Background(), id); err != nil {
		t.Fatalf("MarkDispatchRunning: %v", err)
	}

	store.mu.RLock()
	d := store.dispatches[id]
	store.mu.RUnlock()
	if d.Status != kerneldomain.DispatchRunning {
		t.Errorf("expected running status, got %q", d.Status)
	}
}

func TestFileStore_MarkRunning_NotFound(t *testing.T) {
	now := time.Now()
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	err := store.MarkDispatchRunning(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent dispatch")
	}
}

func TestFileStore_MarkRunning_CancelledContext(t *testing.T) {
	now := time.Now()
	store, _ := newTestStore(t, now)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := store.MarkDispatchRunning(ctx, "any-id")
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Full state-machine cycle: Enqueue → Claim → Running → Done
// ---------------------------------------------------------------------------

func TestFileStore_FullCycle(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	store, _ := newTestStore(t, now)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// Enqueue
	created, err := store.EnqueueDispatches(context.Background(), "k1", "c1", []kerneldomain.DispatchSpec{
		newSpec("agent-x", "full-cycle-task"),
	})
	if err != nil || len(created) != 1 {
		t.Fatalf("EnqueueDispatches: err=%v n=%d", err, len(created))
	}
	id := created[0].DispatchID

	// Claim
	claimed, err := store.ClaimDispatches(context.Background(), "k1", "w1", 10)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimDispatches: err=%v n=%d", err, len(claimed))
	}

	// Running
	if err := store.MarkDispatchRunning(context.Background(), id); err != nil {
		t.Fatalf("MarkDispatchRunning: %v", err)
	}

	// Done
	if err := store.MarkDispatchDone(context.Background(), id, "task-abc"); err != nil {
		t.Fatalf("MarkDispatchDone: %v", err)
	}

	store.mu.RLock()
	d := store.dispatches[id]
	store.mu.RUnlock()
	if d.Status != kerneldomain.DispatchDone {
		t.Errorf("expected done status, got %q", d.Status)
	}
	if d.TaskID != "task-abc" {
		t.Errorf("expected taskID task-abc, got %q", d.TaskID)
	}
}
