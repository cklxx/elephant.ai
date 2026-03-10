package taskfile

import (
	"os"
	"path/filepath"
	"testing"
)

// osTestIO is a minimal StatusFileIO backed by the real filesystem, used only in tests.
type osTestIO struct{}

func (osTestIO) ReadFile(path string) ([]byte, error)              { return os.ReadFile(path) }
func (osTestIO) WriteFileAtomic(path string, data []byte) error    { return os.WriteFile(path, data, 0o644) }

func TestReadStatusFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team-test.status.yaml")
	content := `plan_id: team-test
updated_at: 2026-02-27T10:00:00Z
tasks:
  - id: team-worker-a
    description: worker-a role for team test
    status: completed
    agent_type: kimi
    elapsed: 12s
  - id: team-worker-b
    description: worker-b role for team test
    status: failed
    agent_type: codex
    error: "timeout after 120s"
    elapsed: 120s
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sf, err := ReadStatusFile(path, osTestIO{})
	if err != nil {
		t.Fatalf("ReadStatusFile: %v", err)
	}
	if sf.PlanID != "team-test" {
		t.Errorf("expected plan_id 'team-test', got %q", sf.PlanID)
	}
	if len(sf.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(sf.Tasks))
	}
	if sf.Tasks[0].Status != "completed" {
		t.Errorf("task 0 status: expected 'completed', got %q", sf.Tasks[0].Status)
	}
	if sf.Tasks[1].Status != "failed" {
		t.Errorf("task 1 status: expected 'failed', got %q", sf.Tasks[1].Status)
	}
	if sf.Tasks[1].Error != "timeout after 120s" {
		t.Errorf("task 1 error: expected 'timeout after 120s', got %q", sf.Tasks[1].Error)
	}
}

func TestReadStatusFile_FileNotFound(t *testing.T) {
	_, err := ReadStatusFile("/nonexistent/status.yaml", osTestIO{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadStatusFile_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.status.yaml")
	if err := os.WriteFile(path, []byte("tasks:\n  - [broken: {nesting"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ReadStatusFile(path, osTestIO{})
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}
