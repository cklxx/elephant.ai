package lark

import (
	"context"
	"testing"
	"time"

	"alex/internal/shared/testutil"
)

func TestPlanReviewPostgresStore_SaveGetClear(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := NewPlanReviewPostgresStore(pool, 30*time.Minute)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	pending := PlanReviewPending{
		UserID:        "ou_user_1",
		ChatID:        "oc_chat_1",
		RunID:         "run-1",
		OverallGoalUI: "goal",
		InternalPlan:  map[string]any{"steps": []any{"a", "b"}},
	}
	if err := store.SavePending(ctx, pending); err != nil {
		t.Fatalf("save pending: %v", err)
	}

	got, ok, err := store.GetPending(ctx, "ou_user_1", "oc_chat_1")
	if err != nil {
		t.Fatalf("get pending: %v", err)
	}
	if !ok {
		t.Fatal("expected pending record")
	}
	if got.RunID != "run-1" || got.OverallGoalUI != "goal" {
		t.Fatalf("unexpected record: %+v", got)
	}

	if err := store.ClearPending(ctx, "ou_user_1", "oc_chat_1"); err != nil {
		t.Fatalf("clear pending: %v", err)
	}
	_, ok, err = store.GetPending(ctx, "ou_user_1", "oc_chat_1")
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if ok {
		t.Fatal("expected pending cleared")
	}
}

func TestPlanReviewPostgresStore_Expired(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := NewPlanReviewPostgresStore(pool, time.Minute)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	pending := PlanReviewPending{
		UserID:        "ou_user_2",
		ChatID:        "oc_chat_2",
		RunID:         "run-2",
		OverallGoalUI: "goal-2",
		CreatedAt:     time.Now().Add(-2 * time.Hour),
		ExpiresAt:     time.Now().Add(-time.Minute),
	}
	if err := store.SavePending(ctx, pending); err != nil {
		t.Fatalf("save pending: %v", err)
	}

	_, ok, err := store.GetPending(ctx, "ou_user_2", "oc_chat_2")
	if err != nil {
		t.Fatalf("get pending: %v", err)
	}
	if ok {
		t.Fatal("expected expired pending to be treated as missing")
	}
}
