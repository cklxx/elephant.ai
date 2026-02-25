package taskfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"gopkg.in/yaml.v3"
)

// StatusFile is the YAML structure written as a sidecar to a TaskFile.
type StatusFile struct {
	PlanID    string       `yaml:"plan_id"`
	UpdatedAt time.Time    `yaml:"updated_at"`
	Tasks     []TaskStatus `yaml:"tasks"`
}

// TaskStatus captures the current state of a single task.
type TaskStatus struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"`
	AgentType   string `yaml:"agent_type,omitempty"`
	Error       string `yaml:"error,omitempty"`
	Elapsed     string `yaml:"elapsed,omitempty"`
}

// StatusWriter manages atomic writes of task status to a YAML sidecar file
// and optionally polls the dispatcher for live updates.
type StatusWriter struct {
	mu       sync.Mutex
	path     string
	file     StatusFile
	stopCh   chan struct{}
	stopped  bool
}

// NewStatusWriter creates a StatusWriter that writes to the given path.
func NewStatusWriter(path string) *StatusWriter {
	return &StatusWriter{
		path:   path,
		stopCh: make(chan struct{}),
	}
}

// InitFromTaskFile initializes status entries from a TaskFile. Tasks with
// dependencies start as "blocked"; others start as "pending".
func (sw *StatusWriter) InitFromTaskFile(tf *TaskFile) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	hasDeps := make(map[string]bool, len(tf.Tasks))
	for _, t := range tf.Tasks {
		hasDeps[t.ID] = len(t.DependsOn) > 0
	}

	statuses := make([]TaskStatus, len(tf.Tasks))
	for i, t := range tf.Tasks {
		status := "pending"
		if hasDeps[t.ID] {
			status = "blocked"
		}
		statuses[i] = TaskStatus{
			ID:          t.ID,
			Description: t.Description,
			Status:      status,
			AgentType:   t.AgentType,
		}
	}

	sw.file = StatusFile{
		PlanID:    tf.PlanID,
		UpdatedAt: time.Now(),
		Tasks:     statuses,
	}
	return sw.writeUnsafe()
}

// StartPolling begins periodic status syncing from the dispatcher.
func (sw *StatusWriter) StartPolling(dispatcher agent.BackgroundTaskDispatcher, taskIDs []string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-sw.stopCh:
				return
			case <-ticker.C:
				sw.syncFromDispatcher(dispatcher, taskIDs)
			}
		}
	}()
}

// Stop terminates the polling goroutine.
func (sw *StatusWriter) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if !sw.stopped {
		sw.stopped = true
		close(sw.stopCh)
	}
}

// SyncOnce performs a single status sync and writes to disk.
func (sw *StatusWriter) SyncOnce(dispatcher agent.BackgroundTaskDispatcher, taskIDs []string) {
	sw.syncFromDispatcher(dispatcher, taskIDs)
}

// Path returns the status file path.
func (sw *StatusWriter) Path() string {
	return sw.path
}

func (sw *StatusWriter) syncFromDispatcher(dispatcher agent.BackgroundTaskDispatcher, taskIDs []string) {
	summaries := dispatcher.Status(taskIDs)
	if len(summaries) == 0 {
		return
	}

	byID := make(map[string]agent.BackgroundTaskSummary, len(summaries))
	for _, s := range summaries {
		byID[s.ID] = s
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	changed := false
	for i := range sw.file.Tasks {
		ts := &sw.file.Tasks[i]
		s, ok := byID[ts.ID]
		if !ok {
			continue
		}
		newStatus := string(s.Status)
		if ts.Status != newStatus || ts.Error != s.Error {
			ts.Status = newStatus
			ts.Error = s.Error
			if s.Elapsed > 0 {
				ts.Elapsed = s.Elapsed.Round(time.Second).String()
			}
			changed = true
		}
	}

	if changed {
		sw.file.UpdatedAt = time.Now()
		_ = sw.writeUnsafe()
	}
}

func (sw *StatusWriter) writeUnsafe() error {
	data, err := yaml.Marshal(&sw.file)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	dir := filepath.Dir(sw.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create status dir: %w", err)
	}

	tmp := sw.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write status tmp: %w", err)
	}
	if err := os.Rename(tmp, sw.path); err != nil {
		return fmt.Errorf("rename status: %w", err)
	}
	return nil
}
