package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/domain/agent/react"
)

func sampleCheckpoint(sessionID string) *react.Checkpoint {
	result := "tool output"
	return &react.Checkpoint{
		ID:            "cp-1",
		SessionID:     sessionID,
		Iteration:     2,
		MaxIterations: 10,
		Messages: []react.MessageState{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
		PendingTools: []react.ToolCallState{
			{
				ID:        "tc-1",
				Name:      "web_search",
				Arguments: map[string]any{"query": "go testing"},
				Status:    "completed",
				Result:    &result,
			},
		},
		CreatedAt: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Version:   1,
	}
}

func sampleArchive(sessionID string, seq int) *react.CheckpointArchive {
	return &react.CheckpointArchive{
		SessionID:  sessionID,
		Seq:        seq,
		PhaseLabel: "phase-1",
		Messages: []react.MessageState{
			{Role: "user", Content: "archived message"},
		},
		TokenCount: 42,
		CreatedAt:  time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
	}
}

// --- Save + Load round-trip ---

func TestSaveAndLoad(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	ctx := context.Background()
	cp := sampleCheckpoint("sess-1")

	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, "sess-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.ID != cp.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, cp.ID)
	}
	if loaded.SessionID != cp.SessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, cp.SessionID)
	}
	if loaded.Iteration != cp.Iteration {
		t.Errorf("Iteration = %d, want %d", loaded.Iteration, cp.Iteration)
	}
	if len(loaded.Messages) != len(cp.Messages) {
		t.Errorf("Messages count = %d, want %d", len(loaded.Messages), len(cp.Messages))
	}
	if len(loaded.PendingTools) != 1 || loaded.PendingTools[0].Name != "web_search" {
		t.Errorf("PendingTools not preserved")
	}
	if loaded.PendingTools[0].Result == nil || *loaded.PendingTools[0].Result != "tool output" {
		t.Error("ToolCallState.Result not preserved")
	}
}

// --- Save validates nil ---

func TestSave_Nil(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	if err := store.Save(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil checkpoint")
	}
}

// --- Save validates empty session ID ---

func TestSave_EmptySessionID(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	cp := sampleCheckpoint("")
	if err := store.Save(context.Background(), cp); err == nil {
		t.Fatal("expected error for empty session_id")
	}
}

// --- Load missing file returns (nil, nil) ---

func TestLoad_NotFound(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	cp, err := store.Load(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cp != nil {
		t.Fatal("expected nil checkpoint for missing file")
	}
}

// --- Load empty session ID ---

func TestLoad_EmptySessionID(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	_, err := store.Load(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty session_id")
	}
}

// --- Load corrupt JSON ---

func TestLoad_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := store.Load(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

// --- Delete ---

func TestDelete(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	ctx := context.Background()

	if err := store.Save(ctx, sampleCheckpoint("sess-del")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "sess-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Confirm gone.
	cp, err := store.Load(ctx, "sess-del")
	if err != nil {
		t.Fatal(err)
	}
	if cp != nil {
		t.Fatal("checkpoint should be deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	if err := store.Delete(context.Background(), "nope"); err != nil {
		t.Fatalf("Delete non-existent should not error: %v", err)
	}
}

func TestDelete_EmptySessionID(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	if err := store.Delete(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty session_id")
	}
}

// --- SaveArchive ---

func TestSaveArchive(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)
	ctx := context.Background()

	archive := sampleArchive("sess-arc", 3)
	if err := store.SaveArchive(ctx, archive); err != nil {
		t.Fatalf("SaveArchive: %v", err)
	}

	// Verify file exists at expected path.
	path := filepath.Join(dir, "sess-arc", "archive", "3.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("archive file not found: %v", err)
	}

	var loaded react.CheckpointArchive
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal archive: %v", err)
	}
	if loaded.Seq != 3 {
		t.Errorf("Seq = %d, want 3", loaded.Seq)
	}
	if loaded.PhaseLabel != "phase-1" {
		t.Errorf("PhaseLabel = %q, want phase-1", loaded.PhaseLabel)
	}
	if loaded.TokenCount != 42 {
		t.Errorf("TokenCount = %d, want 42", loaded.TokenCount)
	}
}

func TestSaveArchive_Nil(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	if err := store.SaveArchive(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil archive")
	}
}

func TestSaveArchive_EmptySessionID(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	archive := sampleArchive("", 1)
	if err := store.SaveArchive(context.Background(), archive); err == nil {
		t.Fatal("expected error for empty session_id")
	}
}

// --- BaseDir ---

func TestBaseDir(t *testing.T) {
	store := NewFileCheckpointStore("/custom/path")
	if got := store.BaseDir(); got != "/custom/path" {
		t.Errorf("BaseDir() = %q, want /custom/path", got)
	}
}

// --- Save overwrites existing ---

func TestSave_Overwrite(t *testing.T) {
	store := NewFileCheckpointStore(t.TempDir())
	ctx := context.Background()

	cp1 := sampleCheckpoint("sess-ow")
	cp1.Iteration = 1
	if err := store.Save(ctx, cp1); err != nil {
		t.Fatal(err)
	}

	cp2 := sampleCheckpoint("sess-ow")
	cp2.Iteration = 5
	if err := store.Save(ctx, cp2); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(ctx, "sess-ow")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Iteration != 5 {
		t.Errorf("Iteration = %d, want 5 (overwritten)", loaded.Iteration)
	}
}

// --- JSON formatting ---

func TestSave_IndentedJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)
	if err := store.Save(context.Background(), sampleCheckpoint("sess-fmt")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sess-fmt.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Indented JSON should contain newlines and spaces.
	if len(data) < 50 {
		t.Fatal("JSON output looks too short for indented format")
	}
	if data[0] != '{' {
		t.Errorf("expected JSON to start with '{', got %q", data[0])
	}
}

// --- Multiple archives for same session ---

func TestSaveArchive_MultipleSeqs(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)
	ctx := context.Background()

	for seq := 0; seq < 3; seq++ {
		if err := store.SaveArchive(ctx, sampleArchive("sess-multi", seq)); err != nil {
			t.Fatalf("SaveArchive seq %d: %v", seq, err)
		}
	}

	for seq := 0; seq < 3; seq++ {
		path := filepath.Join(dir, "sess-multi", "archive", fmt.Sprintf("%d.json", seq))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("archive %d not found: %v", seq, err)
		}
	}
}
