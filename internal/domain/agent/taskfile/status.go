package taskfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
	"gopkg.in/yaml.v3"
)

// statusFile is the YAML structure written as a sidecar to a TaskFile.
type statusFile struct {
	PlanID    string       `yaml:"plan_id"`
	UpdatedAt time.Time    `yaml:"updated_at"`
	Tasks     []taskStatus `yaml:"tasks"`
}

// taskStatus captures the current state of a single task.
type taskStatus struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"`
	AgentType   string `yaml:"agent_type,omitempty"`
	Error       string `yaml:"error,omitempty"`
	Elapsed     string `yaml:"elapsed,omitempty"`
	Stale       bool   `yaml:"stale,omitempty"`
}

// ReadStatusFile reads and deserializes a status sidecar YAML file.
func ReadStatusFile(path string) (*statusFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read status file: %w", err)
	}
	var sf statusFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse status file: %w", err)
	}
	return &sf, nil
}

// statusWriter manages atomic writes of task status to a YAML sidecar file
// and optionally polls the dispatcher for live updates.
type statusWriter struct {
	mu       sync.Mutex
	path     string
	file     statusFile
	stopCh   chan struct{}
	stopOnce sync.Once
	logger   logging.Logger
}

// newStatusWriter creates a statusWriter that writes to the given path.
func newStatusWriter(path string, logger logging.Logger) *statusWriter {
	return &statusWriter{
		path:   path,
		stopCh: make(chan struct{}),
		logger: logging.OrNop(logger),
	}
}

// RehydrateFrom restores in-memory state from a previously persisted statusFile.
func (sw *statusWriter) RehydrateFrom(sf *statusFile) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.file = *sf
}

// InitFromTaskFile initializes status entries from a TaskFile. Tasks with
// dependencies start as "blocked"; others start as "pending".
func (sw *statusWriter) InitFromTaskFile(tf *TaskFile) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	statuses := make([]taskStatus, len(tf.Tasks))
	for i, t := range tf.Tasks {
		status := "pending"
		if len(t.DependsOn) > 0 {
			status = "blocked"
		}
		statuses[i] = taskStatus{
			ID:          t.ID,
			Description: t.Description,
			Status:      status,
			AgentType:   t.AgentType,
		}
	}

	sw.file = statusFile{
		PlanID:    tf.PlanID,
		UpdatedAt: time.Now(),
		Tasks:     statuses,
	}
	return sw.writeUnsafe()
}

// StartPolling begins periodic status syncing from the dispatcher.
func (sw *statusWriter) StartPolling(dispatcher agent.BackgroundTaskDispatcher, taskIDs []string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-sw.stopCh:
				return
			case <-ticker.C:
				if sw.syncFromDispatcher(dispatcher, taskIDs) {
					sw.Stop()
					return
				}
			}
		}
	}()
}

// Stop terminates the polling goroutine.
func (sw *statusWriter) Stop() {
	sw.stopOnce.Do(func() {
		close(sw.stopCh)
	})
}

// SyncOnce performs a single status sync and writes to disk.
func (sw *statusWriter) SyncOnce(dispatcher agent.BackgroundTaskDispatcher, taskIDs []string) {
	sw.syncFromDispatcher(dispatcher, taskIDs)
}

// Path returns the status file path.
func (sw *statusWriter) Path() string {
	return sw.path
}

func (sw *statusWriter) syncFromDispatcher(dispatcher agent.BackgroundTaskDispatcher, taskIDs []string) bool {
	summaries := dispatcher.Status(taskIDs)
	if len(summaries) == 0 {
		return false
	}

	byID := make(map[string]agent.BackgroundTaskSummary, len(summaries))
	for _, s := range summaries {
		byID[s.ID] = s
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	changed := false
	allTerminal := true
	for i := range sw.file.Tasks {
		ts := &sw.file.Tasks[i]
		s, ok := byID[ts.ID]
		if !ok {
			allTerminal = false
			continue
		}
		newStatus := string(s.Status)
		if ts.Status != newStatus || ts.Error != s.Error || ts.Stale != s.Stale {
			ts.Status = newStatus
			ts.Error = s.Error
			ts.Stale = s.Stale
			if s.Elapsed > 0 {
				ts.Elapsed = s.Elapsed.Round(time.Second).String()
			}
			changed = true
		}
		switch s.Status {
		case agent.BackgroundTaskStatusCompleted, agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
			// terminal
		default:
			allTerminal = false
		}
	}

	if changed {
		sw.file.UpdatedAt = time.Now()
		if err := sw.writeUnsafe(); err != nil {
			sw.logger.Warn("status write failed: %v", err)
		}
	}
	return allTerminal
}

func (sw *statusWriter) writeUnsafe() error {
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
