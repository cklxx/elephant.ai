package teamruntime

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestEventRecorder_Record_WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	rec := NewEventRecorder(path)

	if err := rec.Record("test_event", map[string]any{"key": "value"}); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if err := rec.Record("second_event", map[string]any{"n": 42}); err != nil {
		t.Fatalf("Record second failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open events file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []map[string]any
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid JSONL line: %v", err)
		}
		lines = append(lines, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}
	if lines[0]["type"] != "test_event" {
		t.Fatalf("first event type = %q, want test_event", lines[0]["type"])
	}
	if lines[0]["key"] != "value" {
		t.Fatalf("first event key = %v, want value", lines[0]["key"])
	}
	if lines[0]["timestamp"] == nil || lines[0]["timestamp"] == "" {
		t.Fatal("expected timestamp field in JSONL entry")
	}
	if lines[1]["type"] != "second_event" {
		t.Fatalf("second event type = %q, want second_event", lines[1]["type"])
	}
}

func TestEventRecorder_Record_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.jsonl")
	rec := NewEventRecorder(path)

	const numWriters = 20
	var wg sync.WaitGroup
	wg.Add(numWriters)
	for i := 0; i < numWriters; i++ {
		go func(idx int) {
			defer wg.Done()
			if err := rec.Record("concurrent", map[string]any{"idx": idx}); err != nil {
				t.Errorf("concurrent Record(%d) failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open events file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid JSONL on line %d: %v", count+1, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	if count != numWriters {
		t.Fatalf("expected %d JSONL lines, got %d", numWriters, count)
	}
}

func TestEventRecorder_NilSafe(t *testing.T) {
	var rec *EventRecorder

	// Path on nil recorder returns empty string.
	if got := rec.Path(); got != "" {
		t.Fatalf("nil recorder Path() = %q, want empty", got)
	}

	// Record on nil recorder returns nil without error.
	if err := rec.Record("anything", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("nil recorder Record() returned error: %v", err)
	}
}

func TestEventRecorder_EmptyPath(t *testing.T) {
	rec := NewEventRecorder("")

	// Record with empty path is a no-op.
	if err := rec.Record("test", map[string]any{}); err != nil {
		t.Fatalf("empty-path recorder Record() returned error: %v", err)
	}
}

func TestEventRecorder_Path(t *testing.T) {
	rec := NewEventRecorder("/tmp/some/path.jsonl")
	if got := rec.Path(); got != "/tmp/some/path.jsonl" {
		t.Fatalf("Path() = %q, want /tmp/some/path.jsonl", got)
	}
}

func TestEventRecorder_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "events.jsonl")
	rec := NewEventRecorder(nested)

	if err := rec.Record("mkdir_test", map[string]any{}); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("expected file %s to exist: %v", nested, err)
	}
}
