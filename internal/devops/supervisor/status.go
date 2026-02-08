package supervisor

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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

// Read reads the status from disk. It handles both the nested format
// (written by the Go supervisor) and the flat format (written by
// scripts/lark/supervisor.sh).
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

	// If Components is empty, try to parse the flat format from supervisor.sh
	if len(status.Components) == 0 {
		parseFlatStatus(data, &status)
	}

	return status, nil
}

// parseFlatStatus extracts component data from the flat JSON format
// written by scripts/lark/supervisor.sh (e.g. "main_pid", "main_health").
func parseFlatStatus(data []byte, status *Status) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	status.Components = make(map[string]ComponentStatus)

	type compDef struct {
		name      string
		pidKey    string
		healthKey string
		shaKey    string
		aliveKey  string // for components using bool alive instead of health string
	}
	defs := []compDef{
		{"main", "main_pid", "main_health", "main_deployed_sha", ""},
		{"test", "test_pid", "test_health", "test_deployed_sha", ""},
		{"loop", "loop_pid", "", "", "loop_alive"},
	}

	for _, d := range defs {
		pid := jsonInt(raw, d.pidKey)
		if pid == 0 && d.aliveKey == "" {
			continue // component not present
		}

		health := jsonString(raw, d.healthKey)
		if health == "" && d.aliveKey != "" {
			if jsonBool(raw, d.aliveKey) {
				health = "alive"
			} else {
				health = "down"
			}
		}

		sha := jsonString(raw, d.shaKey)

		status.Components[d.name] = ComponentStatus{
			PID:         pid,
			Health:      health,
			DeployedSHA: sha,
		}
	}

	// Flat autofix fields
	if state := jsonString(raw, "autofix_state"); state != "" {
		status.Autofix.State = state
	}
	if v := jsonString(raw, "autofix_incident_id"); v != "" {
		status.Autofix.IncidentID = v
	}
	if v := jsonString(raw, "autofix_last_reason"); v != "" {
		status.Autofix.LastReason = v
	}
	if v := jsonString(raw, "autofix_last_started_at"); v != "" {
		status.Autofix.LastStarted = v
	}
	if v := jsonString(raw, "autofix_last_finished_at"); v != "" {
		status.Autofix.LastFinished = v
	}
	if v := jsonString(raw, "autofix_last_commit"); v != "" {
		status.Autofix.LastCommit = v
	}
	if v, ok := raw["autofix_runs_window"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			status.Autofix.RunsWindow = n
		}
	}
}

func jsonString(m map[string]json.RawMessage, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return ""
}

func jsonInt(m map[string]json.RawMessage, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	var n int
	if json.Unmarshal(v, &n) == nil {
		return n
	}
	// supervisor.sh writes PID as string
	var s string
	if json.Unmarshal(v, &s) == nil {
		n, _ = strconv.Atoi(s)
		return n
	}
	return 0
}

func jsonBool(m map[string]json.RawMessage, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	var b bool
	if json.Unmarshal(v, &b) == nil {
		return b
	}
	return false
}
