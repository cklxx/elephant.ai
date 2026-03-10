package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

func TestSaveLoadDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	s := session.New("abc123", session.MemberClaudeCode, "test goal", "/tmp")
	_ = s.Transition(session.StateStarting)

	if err := st.Save(s); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := st.Load("abc123")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ID != "abc123" || loaded.State != session.StateStarting {
		t.Fatalf("unexpected loaded session: %+v", loaded)
	}

	if err := st.Delete("abc123"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.Load("abc123"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"s1", "s2", "s3"} {
		s := session.New(id, session.MemberCodex, "goal", "/tmp")
		if err := st.Save(s); err != nil {
			t.Fatal(err)
		}
	}

	all, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(all))
	}
}

func TestLoad_notFound(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)
	if _, err := st.Load("nonexistent"); err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestAppendEvent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Append two events.
	st.AppendEvent("sess-1", "heartbeat", map[string]any{"iteration": 1})
	st.AppendEvent("sess-1", "completed", map[string]any{"answer": "done"})

	// Read the events file and verify it has 2 lines of valid JSON.
	data, err := os.ReadFile(filepath.Join(dir, "sess-1.events.jsonl"))
	if err != nil {
		t.Fatalf("read events file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 event lines, got %d", len(lines))
	}

	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
		if entry["session_id"] != "sess-1" {
			t.Errorf("line %d: session_id = %v, want sess-1", i, entry["session_id"])
		}
		if _, ok := entry["at"]; !ok {
			t.Errorf("line %d: missing 'at' timestamp", i)
		}
	}

	// First event should have type=heartbeat, iteration=1.
	var first map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &first)
	if first["type"] != "heartbeat" {
		t.Errorf("first event type = %v, want heartbeat", first["type"])
	}
}

func TestAppendEvent_EmptySessionID(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)

	// Should be a no-op — no file created.
	st.AppendEvent("", "test", nil)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".events.jsonl") {
			t.Fatal("no events file should be created for empty session ID")
		}
	}
}

func TestDelete_NonExistent(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)
	// Deleting a non-existent session should return nil (not error).
	if err := st.Delete("does-not-exist"); err != nil {
		t.Fatalf("Delete non-existent should return nil, got: %v", err)
	}
}

func TestLoadAll_SkipsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)

	// Save a valid session.
	s := session.New("valid", session.MemberClaudeCode, "goal", "/tmp")
	if err := st.Save(s); err != nil {
		t.Fatal(err)
	}

	// Write a corrupt JSON file.
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a non-JSON file (should be skipped).
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a session"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory (should be skipped).
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	all, err := st.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 valid session, got %d", len(all))
	}
	if all[0].ID != "valid" {
		t.Errorf("expected session ID 'valid', got %q", all[0].ID)
	}
}

func TestSave_Overwrite(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)

	s := session.New("overwrite-test", session.MemberCodex, "goal v1", "/tmp")
	if err := st.Save(s); err != nil {
		t.Fatal(err)
	}

	// Transition and re-save.
	_ = s.Transition(session.StateStarting)
	if err := st.Save(s); err != nil {
		t.Fatal(err)
	}

	loaded, err := st.Load("overwrite-test")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != session.StateStarting {
		t.Errorf("state = %s, want starting", loaded.State)
	}
}
