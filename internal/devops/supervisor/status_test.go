package supervisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusFileWriteRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	sf := NewStatusFile(path)

	status := Status{
		Timestamp:          "2026-02-08T12:00:00Z",
		Mode:               "healthy",
		Components:         map[string]ComponentStatus{
			"main": {PID: 1234, Health: "healthy", DeployedSHA: "abc123"},
			"test": {PID: 5678, Health: "healthy", DeployedSHA: "def456"},
		},
		RestartCountWindow: 2,
		Autofix: AutofixStatus{
			State:      "idle",
			RunsWindow: 0,
		},
	}

	if err := sf.Write(status); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("status file not created: %v", err)
	}

	// Read back
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if got.Mode != "healthy" {
		t.Errorf("Mode = %q, want healthy", got.Mode)
	}
	if got.RestartCountWindow != 2 {
		t.Errorf("RestartCountWindow = %d, want 2", got.RestartCountWindow)
	}
	if comp, ok := got.Components["main"]; !ok {
		t.Error("missing main component")
	} else {
		if comp.PID != 1234 {
			t.Errorf("main PID = %d, want 1234", comp.PID)
		}
		if comp.Health != "healthy" {
			t.Errorf("main health = %q, want healthy", comp.Health)
		}
	}
}

func TestStatusFileAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	sf := NewStatusFile(path)

	// Write initial
	sf.Write(Status{Mode: "healthy"})

	// Write update
	sf.Write(Status{Mode: "degraded"})

	// No tmp file should remain
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after atomic write")
	}

	// Read should return latest
	got, _ := sf.Read()
	if got.Mode != "degraded" {
		t.Errorf("Mode = %q, want degraded", got.Mode)
	}
}
