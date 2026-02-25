package state_store

import (
	"context"
	"testing"
)

func TestInMemoryStoreListSnapshotsPaginationCursor(t *testing.T) {
	store := NewInMemoryStore()
	sessionID := "sess-1"

	for turn := 1; turn <= 5; turn++ {
		if err := store.SaveSnapshot(context.Background(), Snapshot{
			SessionID:  sessionID,
			TurnID:     turn,
			LLMTurnSeq: turn,
			Summary:    "snapshot",
		}); err != nil {
			t.Fatalf("SaveSnapshot turn %d: %v", turn, err)
		}
	}

	items, next, err := store.ListSnapshots(context.Background(), sessionID, "", 2)
	if err != nil {
		t.Fatalf("ListSnapshots page 1: %v", err)
	}
	assertTurnIDs(t, items, []int{1, 2})
	if next != "2" {
		t.Fatalf("expected next cursor 2, got %q", next)
	}

	items, next, err = store.ListSnapshots(context.Background(), sessionID, next, 2)
	if err != nil {
		t.Fatalf("ListSnapshots page 2: %v", err)
	}
	assertTurnIDs(t, items, []int{3, 4})
	if next != "4" {
		t.Fatalf("expected next cursor 4, got %q", next)
	}

	items, next, err = store.ListSnapshots(context.Background(), sessionID, next, 2)
	if err != nil {
		t.Fatalf("ListSnapshots page 3: %v", err)
	}
	assertTurnIDs(t, items, []int{5})
	if next != "" {
		t.Fatalf("expected empty next cursor, got %q", next)
	}
}

func TestInMemoryStoreListSnapshotsCursorBeyondRange(t *testing.T) {
	store := NewInMemoryStore()
	sessionID := "sess-2"

	if err := store.SaveSnapshot(context.Background(), Snapshot{SessionID: sessionID, TurnID: 1}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	items, next, err := store.ListSnapshots(context.Background(), sessionID, "999", 10)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %d", len(items))
	}
	if next != "" {
		t.Fatalf("expected empty next cursor, got %q", next)
	}
}

func assertTurnIDs(t *testing.T, items []SnapshotMetadata, want []int) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("expected %d items, got %d", len(want), len(items))
	}
	for i := range want {
		if items[i].TurnID != want[i] {
			t.Fatalf("item %d: expected turn %d, got %d", i, want[i], items[i].TurnID)
		}
	}
}
