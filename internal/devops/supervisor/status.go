package supervisor

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Status holds the full supervisor status snapshot.
type Status struct {
	Timestamp         string            `json:"ts_utc"`
	Mode              string            `json:"mode"`
	Components        map[string]ComponentStatus `json:"components"`
	RestartCountWindow int              `json:"restart_count_window"`
	Autofix           AutofixStatus     `json:"autofix"`
}

// ComponentStatus holds per-component status.
type ComponentStatus struct {
	PID         int    `json:"pid"`
	Health      string `json:"health"`
	DeployedSHA string `json:"deployed_sha,omitempty"`
}

// AutofixStatus holds autofix-related status.
type AutofixStatus struct {
	State        string `json:"state"`
	IncidentID   string `json:"incident_id,omitempty"`
	LastReason   string `json:"last_reason,omitempty"`
	LastStarted  string `json:"last_started_at,omitempty"`
	LastFinished string `json:"last_finished_at,omitempty"`
	LastCommit   string `json:"last_commit,omitempty"`
	RunsWindow   int    `json:"runs_window"`
}

// StatusFile provides atomic JSON status file operations.
type StatusFile struct {
	path string
	mu   sync.Mutex
}

// NewStatusFile creates a new status file manager.
func NewStatusFile(path string) *StatusFile {
	return &StatusFile{path: path}
}

// Write atomically writes the status to disk.
func (sf *StatusFile) Write(status Status) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if status.Timestamp == "" {
		status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	// Atomic write: tmp file + rename
	tmp := sf.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp status: %w", err)
	}
	return os.Rename(tmp, sf.path)
}

// Read reads the status from disk.
func (sf *StatusFile) Read() (Status, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	data, err := os.ReadFile(sf.path)
	if err != nil {
		return Status{}, err
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return Status{}, fmt.Errorf("parse status: %w", err)
	}
	return status, nil
}
