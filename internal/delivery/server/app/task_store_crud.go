package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"alex/internal/delivery/server/ports"
	"alex/internal/infra/filestore"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

// Create creates a new task with optional presets
func (s *InMemoryTaskStore) Create(ctx context.Context, sessionID string, description string, agentPreset string, toolPreset string) (*ports.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := id.RunIDFromContext(ctx)
	if taskID == "" {
		taskID = id.NewRunID()
	}
	now := time.Now()

	task := &ports.Task{
		ID:           taskID,
		SessionID:    sessionID,
		ParentTaskID: id.ParentRunIDFromContext(ctx),
		Status:       ports.TaskStatusPending,
		Description:  description,
		CreatedAt:    now,
		Metadata:     make(map[string]string),
		AgentPreset:  agentPreset,
		ToolPreset:   toolPreset,
	}

	s.tasks[taskID] = task
	delete(s.owners, taskID)
	delete(s.leases, taskID)
	s.persistLocked()

	// Return a copy to prevent callers from sharing references with the store.
	taskCopy := *task
	return &taskCopy, nil
}

// Get retrieves a task by ID
func (s *InMemoryTaskStore) Get(ctx context.Context, taskID string) (*ports.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	// Return a copy to prevent concurrent access issues
	taskCopy := *task
	return &taskCopy, nil
}

// Update updates task state
func (s *InMemoryTaskStore) Update(ctx context.Context, task *ports.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return NotFoundError(fmt.Sprintf("task %s", task.ID))
	}

	s.tasks[task.ID] = task
	s.persistLocked()
	return nil
}

// List returns tasks with pagination
func (s *InMemoryTaskStore) List(ctx context.Context, limit int, offset int) ([]*ports.Task, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert map to slice and sort by created_at (newest first)
	tasks := make([]*ports.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		// Create a copy to prevent concurrent access issues
		taskCopy := *task
		tasks = append(tasks, &taskCopy)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	total := len(tasks)

	// Apply pagination
	if offset >= total {
		return []*ports.Task{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return tasks[offset:end], total, nil
}

// ListBySession returns tasks for a specific session
func (s *InMemoryTaskStore) ListBySession(ctx context.Context, sessionID string) ([]*ports.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ports.Task, 0)
	for _, task := range s.tasks {
		if task.SessionID == sessionID {
			// Create a copy to prevent concurrent access issues
			taskCopy := *task
			tasks = append(tasks, &taskCopy)
		}
	}

	// Sort by created_at (newest first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// SummarizeSessionTasks returns task_count/last_task for each requested
// session ID in one pass over the in-memory task map.
func (s *InMemoryTaskStore) SummarizeSessionTasks(ctx context.Context, sessionIDs []string) (map[string]SessionTaskSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make(map[string]SessionTaskSummary, len(sessionIDs))
	targetSessions := make(map[string]struct{}, len(sessionIDs))
	lastCreatedAt := make(map[string]time.Time, len(sessionIDs))
	lastTaskID := make(map[string]string, len(sessionIDs))

	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}
		if _, exists := targetSessions[sessionID]; exists {
			continue
		}
		targetSessions[sessionID] = struct{}{}
		summaries[sessionID] = SessionTaskSummary{}
	}

	if len(targetSessions) == 0 {
		return summaries, nil
	}

	for _, task := range s.tasks {
		if _, interested := targetSessions[task.SessionID]; !interested {
			continue
		}

		summary := summaries[task.SessionID]
		summary.TaskCount++

		lastAt, hasLast := lastCreatedAt[task.SessionID]
		shouldUpdateLast := !hasLast || task.CreatedAt.After(lastAt)
		if !shouldUpdateLast && task.CreatedAt.Equal(lastAt) {
			shouldUpdateLast = task.ID > lastTaskID[task.SessionID]
		}
		if shouldUpdateLast {
			summary.LastTask = task.Description
			lastCreatedAt[task.SessionID] = task.CreatedAt
			lastTaskID[task.SessionID] = task.ID
		}

		summaries[task.SessionID] = summary
	}

	return summaries, nil
}

// ListByStatus returns tasks for the provided status filters.
func (s *InMemoryTaskStore) ListByStatus(ctx context.Context, statuses ...ports.TaskStatus) ([]*ports.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(statuses) == 0 {
		return []*ports.Task{}, nil
	}

	statusSet := make(map[ports.TaskStatus]struct{}, len(statuses))
	for _, status := range statuses {
		statusSet[status] = struct{}{}
	}

	tasks := make([]*ports.Task, 0)
	for _, task := range s.tasks {
		if _, ok := statusSet[task.Status]; !ok {
			continue
		}
		taskCopy := *task
		tasks = append(tasks, &taskCopy)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// Delete removes a task
func (s *InMemoryTaskStore) Delete(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	delete(s.tasks, taskID)
	delete(s.owners, taskID)
	delete(s.leases, taskID)
	s.persistLocked()
	return nil
}

type persistedTaskStore struct {
	Version int           `json:"version"`
	Tasks   []*ports.Task `json:"tasks"`
}

func (s *InMemoryTaskStore) loadFromDisk() {
	if s.persistencePath == "" {
		return
	}
	data, err := filestore.ReadFileOrEmpty(s.persistencePath)
	if err != nil {
		s.logger.Warn("failed to load task persistence file %s: %v", s.persistencePath, err)
		return
	}
	if len(data) == 0 {
		return
	}

	var persisted persistedTaskStore
	if err := json.Unmarshal(data, &persisted); err != nil {
		s.logger.Warn("failed to parse task persistence file %s: %v", s.persistencePath, err)
		return
	}

	loaded := make(map[string]*ports.Task, len(persisted.Tasks))
	for _, task := range persisted.Tasks {
		if task == nil || utils.IsBlank(task.ID) {
			continue
		}
		taskCopy := *task
		loaded[task.ID] = &taskCopy
	}
	s.tasks = loaded
}

func (s *InMemoryTaskStore) persistLocked() {
	if s.persistencePath == "" {
		return
	}

	snapshot := make([]*ports.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		taskCopy := *task
		snapshot = append(snapshot, &taskCopy)
	}

	payload := persistedTaskStore{
		Version: 1,
		Tasks:   snapshot,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("failed to encode task persistence payload: %v", err)
		return
	}

	if err := filestore.AtomicWrite(s.persistencePath, data, 0o600); err != nil {
		s.logger.Warn("failed to persist task store to %s: %v", s.persistencePath, err)
	}
}
