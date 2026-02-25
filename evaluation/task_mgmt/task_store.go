package task_mgmt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// TaskStore provides file-based persistence for eval task definitions and batch runs.
type TaskStore struct {
	dir string
	mu  sync.RWMutex
}

// NewTaskStore creates a TaskStore at the given directory.
func NewTaskStore(dir string) (*TaskStore, error) {
	tasksDir := filepath.Join(dir, "tasks")
	runsDir := filepath.Join(dir, "runs")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tasks dir: %w", err)
	}
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create runs dir: %w", err)
	}
	return &TaskStore{dir: dir}, nil
}

// SaveTask persists a task definition.
func (s *TaskStore) SaveTask(task *EvalTaskDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, "tasks", task.ID+".json")
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// GetTask loads a task by ID.
func (s *TaskStore) GetTask(id string) (*EvalTaskDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, "tasks", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return nil, fmt.Errorf("read task: %w", err)
	}
	var task EvalTaskDefinition
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("decode task: %w", err)
	}
	return &task, nil
}

// ListTasks returns all task definitions sorted by creation time (newest first).
func (s *TaskStore) ListTasks() ([]*EvalTaskDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.dir, "tasks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}

	tasks := make([]*EvalTaskDefinition, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var task EvalTaskDefinition
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}
		tasks = append(tasks, &task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return tasks, nil
}

// DeleteTask removes a task definition.
func (s *TaskStore) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, "tasks", id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task %s not found", id)
		}
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// SaveRun persists a batch run record.
func (s *TaskStore) SaveRun(run *BatchRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, "runs", run.ID+".json")
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ListRunsForTask returns batch runs for a task, newest first.
func (s *TaskStore) ListRunsForTask(taskID string) ([]*BatchRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.dir, "runs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read runs dir: %w", err)
	}

	var runs []*BatchRun
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var run BatchRun
		if err := json.Unmarshal(data, &run); err != nil {
			continue
		}
		if run.TaskID == taskID {
			runs = append(runs, &run)
		}
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs, nil
}
