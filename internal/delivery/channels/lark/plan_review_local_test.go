package lark

import (
	"context"
	"testing"
	"time"
)

// --- planReviewKey ---

func TestPlanReviewKey_Basic(t *testing.T) {
	if got := planReviewKey("u1", "c1"); got != "u1::c1" {
		t.Fatalf("expected u1::c1, got %q", got)
	}
}

func TestPlanReviewKey_TrimsWhitespace(t *testing.T) {
	if got := planReviewKey("  u1  ", "  c1  "); got != "u1::c1" {
		t.Fatalf("expected u1::c1, got %q", got)
	}
}

func TestPlanReviewKey_Empty(t *testing.T) {
	if got := planReviewKey("", ""); got != "::" {
		t.Fatalf("expected ::, got %q", got)
	}
}

// --- SavePending ---

func TestSavePending_Valid(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	err := store.SavePending(context.Background(), PlanReviewPending{
		UserID: "u1", ChatID: "c1", RunID: "r1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok, _ := store.GetPending(context.Background(), "u1", "c1")
	if !ok {
		t.Fatal("expected pending found")
	}
	if got.RunID != "r1" {
		t.Fatalf("expected RunID r1, got %q", got.RunID)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt set")
	}
	if got.ExpiresAt.IsZero() {
		t.Fatal("expected ExpiresAt set")
	}
}

func TestSavePending_BlankUserID(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	err := store.SavePending(context.Background(), PlanReviewPending{
		UserID: "", ChatID: "c1",
	})
	if err == nil {
		t.Fatal("expected error for blank user_id")
	}
}

func TestSavePending_BlankChatID(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	err := store.SavePending(context.Background(), PlanReviewPending{
		UserID: "u1", ChatID: "",
	})
	if err == nil {
		t.Fatal("expected error for blank chat_id")
	}
}

func TestSavePending_PreservesExistingTimestamps(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC) // far future to avoid eviction

	// Pin clock so EvictByTTL won't expire the entry.
	store.SetNow(func() time.Time { return fixed })

	err := store.SavePending(context.Background(), PlanReviewPending{
		UserID:    "u1",
		ChatID:    "c1",
		CreatedAt: fixed,
		ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _, _ := store.GetPending(context.Background(), "u1", "c1")
	if !got.CreatedAt.Equal(fixed) {
		t.Fatal("expected CreatedAt preserved")
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Fatal("expected ExpiresAt preserved")
	}
}

// --- GetPending ---

func TestGetPending_NotFound(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	_, ok, err := store.GetPending(context.Background(), "u1", "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestGetPending_EmptyFields(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	_, ok, err := store.GetPending(context.Background(), "", "c1")
	if err != nil || ok {
		t.Fatalf("expected false for empty user_id, got ok=%v err=%v", ok, err)
	}
	_, ok, err = store.GetPending(context.Background(), "u1", "")
	if err != nil || ok {
		t.Fatalf("expected false for empty chat_id, got ok=%v err=%v", ok, err)
	}
}

func TestGetPending_Expired(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store.SetNow(func() time.Time { return fixedTime })

	_ = store.SavePending(context.Background(), PlanReviewPending{
		UserID: "u1", ChatID: "c1",
	})

	// Advance past TTL
	store.SetNow(func() time.Time { return fixedTime.Add(2 * time.Hour) })

	_, ok, _ := store.GetPending(context.Background(), "u1", "c1")
	if ok {
		t.Fatal("expected expired entry not returned")
	}
}

// --- ClearPending ---

func TestClearPending_Removes(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	_ = store.SavePending(context.Background(), PlanReviewPending{
		UserID: "u1", ChatID: "c1",
	})
	_ = store.ClearPending(context.Background(), "u1", "c1")

	_, ok, _ := store.GetPending(context.Background(), "u1", "c1")
	if ok {
		t.Fatal("expected cleared")
	}
}

func TestClearPending_NonExistent(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	err := store.ClearPending(context.Background(), "u1", "c1")
	if err != nil {
		t.Fatalf("expected no error for non-existent, got %v", err)
	}
}

func TestClearPending_EmptyFields(t *testing.T) {
	store := NewPlanReviewMemoryStore(time.Hour)
	if err := store.ClearPending(context.Background(), "", "c1"); err != nil {
		t.Fatalf("expected no error for empty user_id, got %v", err)
	}
}

// --- NewPlanReviewFileStore ---

func TestNewPlanReviewFileStore_EmptyDir(t *testing.T) {
	_, err := NewPlanReviewFileStore("", time.Hour)
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestNewPlanReviewFileStore_ValidDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewPlanReviewFileStore(dir, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

// --- DefaultTTL ---

func TestNewPlanReviewMemoryStore_DefaultTTL(t *testing.T) {
	store := NewPlanReviewMemoryStore(0) // 0 → should use defaultPlanReviewTTL
	if store.ttl != defaultPlanReviewTTL {
		t.Fatalf("expected default TTL %v, got %v", defaultPlanReviewTTL, store.ttl)
	}
}
