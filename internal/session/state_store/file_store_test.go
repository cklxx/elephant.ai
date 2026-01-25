package state_store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
)

func TestFileStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	sessionID := "sess-test"

	snapshot := Snapshot{
		SessionID:  sessionID,
		TurnID:     1,
		LLMTurnSeq: 1,
		CreatedAt:  time.Now().UTC().Round(time.Second),
		Summary:    "observed user question",
		Plans: []agent.PlanNode{{
			ID:     "root",
			Title:  "Plan",
			Status: "in_progress",
		}},
		Beliefs: []agent.Belief{{
			Statement:  "user prefers concise answers",
			Confidence: 0.8,
		}},
	}

	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	latest, err := store.LatestSnapshot(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("LatestSnapshot failed: %v", err)
	}
	if latest.Summary != snapshot.Summary {
		t.Fatalf("expected summary %q, got %q", snapshot.Summary, latest.Summary)
	}

	items, cursor, err := store.ListSnapshots(context.Background(), sessionID, "", 10)
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(items) != 1 || cursor != "" {
		t.Fatalf("unexpected list result %+v cursor=%q", items, cursor)
	}

	retrieved, err := store.GetSnapshot(context.Background(), sessionID, 1)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if retrieved.TurnID != 1 {
		t.Fatalf("expected turn 1 got %d", retrieved.TurnID)
	}

	// Ensure files written to disk
	path := filepath.Join(dir, sessionID, "turn_000001.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected snapshot file to exist: %v", err)
	}
}
