package supervisor

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestAutofixRunnerReadStateFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "autofix.state.json")

	runner := NewAutofixRunner(AutofixConfig{
		StateFile:   stateFile,
		HistoryFile: filepath.Join(dir, "history"),
	}, slog.Default())

	// Missing file returns zero-value, no error
	data, err := runner.ReadStateFile()
	if err != nil {
		t.Fatalf("ReadStateFile with missing file: %v", err)
	}
	if data.State != "" {
		t.Errorf("State = %q, want empty", data.State)
	}

	// Write a state file and read it back
	state := `{
  "autofix_state": "succeeded",
  "autofix_incident_id": "afx-20260208T134600Z-main-a1b2c3d4",
  "autofix_last_reason": "restart storm: main",
  "autofix_last_started_at": "2026-02-08T13:46:00Z",
  "autofix_last_finished_at": "2026-02-08T13:50:00Z",
  "autofix_last_commit": "abc12345",
  "autofix_restart_required": "true"
}`
	if err := os.WriteFile(stateFile, []byte(state), 0o644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	data, err = runner.ReadStateFile()
	if err != nil {
		t.Fatalf("ReadStateFile: %v", err)
	}
	if data.State != "succeeded" {
		t.Errorf("State = %q, want succeeded", data.State)
	}
	if data.IncidentID != "afx-20260208T134600Z-main-a1b2c3d4" {
		t.Errorf("IncidentID = %q, want afx-20260208T134600Z-main-a1b2c3d4", data.IncidentID)
	}
	if data.LastReason != "restart storm: main" {
		t.Errorf("LastReason = %q, want 'restart storm: main'", data.LastReason)
	}
	if data.LastStartedAt != "2026-02-08T13:46:00Z" {
		t.Errorf("LastStartedAt = %q, want 2026-02-08T13:46:00Z", data.LastStartedAt)
	}
	if data.LastFinishedAt != "2026-02-08T13:50:00Z" {
		t.Errorf("LastFinishedAt = %q, want 2026-02-08T13:50:00Z", data.LastFinishedAt)
	}
	if data.LastCommit != "abc12345" {
		t.Errorf("LastCommit = %q, want abc12345", data.LastCommit)
	}
	if data.RestartRequired != "true" {
		t.Errorf("RestartRequired = %q, want true", data.RestartRequired)
	}
}

func TestAutofixRunnerReadStateFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "autofix.state.json")

	runner := NewAutofixRunner(AutofixConfig{
		StateFile:   stateFile,
		HistoryFile: filepath.Join(dir, "history"),
	}, slog.Default())

	if err := os.WriteFile(stateFile, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write invalid state file: %v", err)
	}

	_, err := runner.ReadStateFile()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
