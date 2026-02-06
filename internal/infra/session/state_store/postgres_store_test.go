package state_store

import (
	"context"
	"testing"
	"time"

	"alex/internal/shared/testutil"
)

func TestPostgresStore_CrossInstanceSnapshotAccess(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	storeA := NewPostgresStore(pool, SnapshotKindState)
	storeB := NewPostgresStore(pool, SnapshotKindState)

	if err := storeA.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	snapshot := Snapshot{
		SessionID:  "session-1",
		TurnID:     1,
		LLMTurnSeq: 1,
		Summary:    "first",
		CreatedAt:  time.Now(),
	}
	if err := storeA.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	loaded, err := storeB.GetSnapshot(ctx, "session-1", 1)
	if err != nil {
		t.Fatalf("get snapshot from other instance: %v", err)
	}
	if loaded.Summary != "first" {
		t.Fatalf("expected summary to match, got %q", loaded.Summary)
	}

	metas, _, err := storeB.ListSnapshots(ctx, "session-1", "", 10)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(metas) != 1 || metas[0].TurnID != 1 {
		t.Fatalf("expected snapshot metadata, got %#v", metas)
	}
}

func TestPostgresStore_KindIsolation(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	stateStore := NewPostgresStore(pool, SnapshotKindState)
	historyStore := NewPostgresStore(pool, SnapshotKindTurn)

	if err := stateStore.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	snapshot := Snapshot{
		SessionID:  "session-1",
		TurnID:     1,
		LLMTurnSeq: 1,
		Summary:    "state",
		CreatedAt:  time.Now(),
	}
	if err := stateStore.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	if _, err := historyStore.GetSnapshot(ctx, "session-1", 1); err != ErrSnapshotNotFound {
		t.Fatalf("expected history store to miss state snapshot, got %v", err)
	}
}

func TestPostgresStore_ClearSession(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresStore(pool, SnapshotKindState)

	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	snapshot := Snapshot{
		SessionID:  "session-clear",
		TurnID:     1,
		LLMTurnSeq: 1,
		Summary:    "state",
		CreatedAt:  time.Now(),
	}
	if err := store.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	if err := store.ClearSession(ctx, snapshot.SessionID); err != nil {
		t.Fatalf("clear session: %v", err)
	}

	if _, err := store.GetSnapshot(ctx, snapshot.SessionID, snapshot.TurnID); err != ErrSnapshotNotFound {
		t.Fatalf("expected snapshot to be cleared, got %v", err)
	}

	metas, _, err := store.ListSnapshots(ctx, snapshot.SessionID, "", 10)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(metas) != 0 {
		t.Fatalf("expected no snapshots after clear, got %#v", metas)
	}
}
