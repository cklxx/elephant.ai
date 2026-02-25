package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectOrphanedBridges_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	orphans := DetectOrphanedBridges(dir)
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestDetectOrphanedBridges_NoBaseDir(t *testing.T) {
	t.Parallel()
	orphans := DetectOrphanedBridges("/nonexistent/path")
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestDetectOrphanedBridges_CompletedTask(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	taskDir := filepath.Join(dir, ".elephant", "bridge", "task-1")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write output file, status file, and done sentinel.
	if err := os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusData, _ := json.Marshal(map[string]any{"pid": 99999})
	if err := os.WriteFile(filepath.Join(taskDir, "status.json"), statusData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, ".done"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	orphans := DetectOrphanedBridges(dir)
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", orphans[0].TaskID)
	}
	if !orphans[0].HasDone {
		t.Error("HasDone should be true")
	}
	if orphans[0].PID != 99999 {
		t.Errorf("PID = %d, want 99999", orphans[0].PID)
	}
	// PID 99999 almost certainly not running.
	if orphans[0].IsRunning {
		t.Error("IsRunning should be false for PID 99999")
	}
}

func TestDetectOrphanedBridges_IncompleteTask(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	taskDir := filepath.Join(dir, ".elephant", "bridge", "task-2")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Output file present, no done sentinel.
	if err := os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	orphans := DetectOrphanedBridges(dir)
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].HasDone {
		t.Error("HasDone should be false")
	}
	if orphans[0].IsRunning {
		t.Error("IsRunning should be false (no PID)")
	}
}

func TestDetectOrphanedBridges_SkipsDirWithoutOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create task dir without output file.
	taskDir := filepath.Join(dir, ".elephant", "bridge", "task-3")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	orphans := DetectOrphanedBridges(dir)
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans (no output file), got %d", len(orphans))
	}
}

func TestDetectOrphanedBridges_MultipleTasks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, id := range []string{"a", "b", "c"} {
		taskDir := filepath.Join(dir, ".elephant", "bridge", id)
		_ = os.MkdirAll(taskDir, 0o755)
		_ = os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte("{}"), 0o644)
	}
	// Mark "b" as done.
	_ = os.WriteFile(filepath.Join(dir, ".elephant", "bridge", "b", ".done"), nil, 0o644)

	orphans := DetectOrphanedBridges(dir)
	if len(orphans) != 3 {
		t.Fatalf("expected 3 orphans, got %d", len(orphans))
	}

	doneCount := 0
	for _, o := range orphans {
		if o.HasDone {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Errorf("expected 1 done, got %d", doneCount)
	}
}

func TestCleanupBridgeDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	taskDir := filepath.Join(dir, ".elephant", "bridge", "cleanup-test")
	_ = os.MkdirAll(taskDir, 0o755)
	_ = os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte("{}"), 0o644)
	_ = os.WriteFile(filepath.Join(taskDir, ".done"), nil, 0o644)

	if err := CleanupBridgeDir(dir, "cleanup-test"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Errorf("expected task dir removed, got err=%v", err)
	}
}

func TestIsProcessAlive_NonexistentPID(t *testing.T) {
	t.Parallel()
	// PID 99999 almost certainly doesn't exist.
	if isProcessAlive(99999) {
		t.Skip("PID 99999 unexpectedly exists, skipping")
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	t.Parallel()
	if !isProcessAlive(os.Getpid()) {
		t.Error("expected current process to be alive")
	}
}
