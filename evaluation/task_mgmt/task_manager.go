package task_mgmt

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var taskSeq atomic.Uint64

// TaskManager provides CRUD + run operations for eval task definitions.
type TaskManager struct {
	store *TaskStore
}

// NewTaskManager creates a new TaskManager.
func NewTaskManager(store *TaskStore) *TaskManager {
	return &TaskManager{store: store}
}

// Create creates a new eval task definition from a request.
func (m *TaskManager) Create(req CreateTaskRequest) (*EvalTaskDefinition, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("task name is required")
	}

	now := time.Now()
	task := &EvalTaskDefinition{
		ID:          generateTaskID(now),
		Name:        name,
		Description: req.Description,
		Status:      TaskStatusActive,
		DatasetPath: req.DatasetPath,
		DatasetType: req.DatasetType,
		Config:      req.Config,
		Tags:        req.Tags,
		Schedule:    req.Schedule,
		Metadata:    req.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := m.store.SaveTask(task); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}
	return task, nil
}

// Get returns a task by ID.
func (m *TaskManager) Get(id string) (*EvalTaskDefinition, error) {
	return m.store.GetTask(id)
}

// List returns all task definitions.
func (m *TaskManager) List() ([]*EvalTaskDefinition, error) {
	return m.store.ListTasks()
}

// Update applies partial updates to a task.
func (m *TaskManager) Update(id string, req UpdateTaskRequest) (*EvalTaskDefinition, error) {
	task, err := m.store.GetTask(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.DatasetPath != nil {
		task.DatasetPath = *req.DatasetPath
	}
	if req.Config != nil {
		task.Config = *req.Config
	}
	if req.Tags != nil {
		task.Tags = req.Tags
	}
	if req.Schedule != nil {
		task.Schedule = req.Schedule
	}
	if req.Metadata != nil {
		if task.Metadata == nil {
			task.Metadata = make(map[string]string)
		}
		for k, v := range req.Metadata {
			task.Metadata[k] = v
		}
	}
	task.UpdatedAt = time.Now()

	if err := m.store.SaveTask(task); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}
	return task, nil
}

// Delete removes a task definition.
func (m *TaskManager) Delete(id string) error {
	return m.store.DeleteTask(id)
}

// RecordRun creates a batch run record for a task.
func (m *TaskManager) RecordRun(taskID, evalJobID string) (*BatchRun, error) {
	_, err := m.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("task %s not found: %w", taskID, err)
	}

	now := time.Now()
	run := &BatchRun{
		ID:        fmt.Sprintf("run_%s_%d", taskID, now.UnixMilli()),
		TaskID:    taskID,
		EvalJobID: evalJobID,
		Status:    RunStatusPending,
		StartedAt: now,
	}

	if err := m.store.SaveRun(run); err != nil {
		return nil, fmt.Errorf("save run: %w", err)
	}
	return run, nil
}

// ListRuns returns batch runs for a task.
func (m *TaskManager) ListRuns(taskID string) ([]*BatchRun, error) {
	return m.store.ListRunsForTask(taskID)
}

func generateTaskID(t time.Time) string {
	seq := taskSeq.Add(1)
	return fmt.Sprintf("evtask_%d_%d", t.UnixMilli(), seq)
}
