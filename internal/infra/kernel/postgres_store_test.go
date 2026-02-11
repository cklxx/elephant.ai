package kernel

import (
	"context"
	"os"
	"testing"

	kerneldomain "alex/internal/domain/kernel"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestStore(t *testing.T) *PostgresStore {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	store := NewPostgresStore(pool)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// Clean up test data after test.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM "+dispatchTable+" WHERE kernel_id LIKE 'test-%'")
	})

	return store
}

func TestPostgresStore_EnsureSchemaIdempotent(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	// Second call should succeed without error.
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("second EnsureSchema: %v", err)
	}
}

func TestPostgresStore_EnqueueAndListRecent(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	kernelID := "test-enqueue-list"

	specs := []kerneldomain.DispatchSpec{
		{AgentID: "agent-a", Prompt: "prompt-a", Priority: 10},
		{AgentID: "agent-b", Prompt: "prompt-b", Priority: 5, Metadata: map[string]string{"key": "val"}},
	}

	dispatches, err := store.EnqueueDispatches(ctx, kernelID, "cycle-1", specs)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if len(dispatches) != 2 {
		t.Fatalf("expected 2 dispatches, got %d", len(dispatches))
	}
	for _, d := range dispatches {
		if d.Status != kerneldomain.DispatchPending {
			t.Errorf("expected pending, got %s", d.Status)
		}
	}

	// ListRecentByAgent should return one per agent.
	recent, err := store.ListRecentByAgent(ctx, kernelID)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("expected 2 agents, got %d", len(recent))
	}
	if d, ok := recent["agent-b"]; ok {
		if d.Metadata["key"] != "val" {
			t.Errorf("metadata mismatch: %v", d.Metadata)
		}
	} else {
		t.Error("agent-b not found in recent")
	}
}

func TestPostgresStore_MarkDoneAndFailed(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	kernelID := "test-mark"

	dispatches, err := store.EnqueueDispatches(ctx, kernelID, "cycle-2", []kerneldomain.DispatchSpec{
		{AgentID: "ok-agent", Prompt: "ok"},
		{AgentID: "fail-agent", Prompt: "fail"},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := store.MarkDispatchRunning(ctx, dispatches[0].DispatchID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := store.MarkDispatchDone(ctx, dispatches[0].DispatchID, "task-123"); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if err := store.MarkDispatchFailed(ctx, dispatches[1].DispatchID, "timeout"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// Active dispatches should be empty (both are terminal).
	active, err := store.ListActiveDispatches(ctx, kernelID)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active, got %d", len(active))
	}
}

func TestPostgresStore_ClaimDispatches(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	kernelID := "test-claim"

	_, err := store.EnqueueDispatches(ctx, kernelID, "cycle-3", []kerneldomain.DispatchSpec{
		{AgentID: "c1", Prompt: "p1", Priority: 5},
		{AgentID: "c2", Prompt: "p2", Priority: 10},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Claim 1 — should get the higher-priority one first.
	claimed, err := store.ClaimDispatches(ctx, kernelID, "worker-1", 1)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected 1 claimed, got %d", len(claimed))
	}
	if claimed[0].AgentID != "c2" {
		t.Errorf("expected c2 (higher priority), got %s", claimed[0].AgentID)
	}

	// Second claim should get the remaining one.
	claimed2, err := store.ClaimDispatches(ctx, kernelID, "worker-1", 5)
	if err != nil {
		t.Fatalf("claim 2: %v", err)
	}
	if len(claimed2) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(claimed2))
	}
}
