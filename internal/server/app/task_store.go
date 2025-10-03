package app

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	agentPorts "alex/internal/agent/ports"
	"alex/internal/server/ports"
	"github.com/google/uuid"
)

// InMemoryTaskStore implements TaskStore with in-memory storage
type InMemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*ports.Task
}

// NewInMemoryTaskStore creates a new in-memory task store
func NewInMemoryTaskStore() ports.TaskStore {
	return &InMemoryTaskStore{
		tasks: make(map[string]*ports.Task),
	}
}

// Create creates a new task with optional presets
func (s *InMemoryTaskStore) Create(ctx context.Context, sessionID string, description string, agentPreset string, toolPreset string) (*ports.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := fmt.Sprintf("task-%s", uuid.New().String())
	now := time.Now()

	task := &ports.Task{
		ID:          taskID,
		SessionID:   sessionID,
		Status:      ports.TaskStatusPending,
		Description: description,
		CreatedAt:   now,
		Metadata:    make(map[string]string),
		AgentPreset: agentPreset,
		ToolPreset:  toolPreset,
	}

	s.tasks[taskID] = task
	return task, nil
}

// Get retrieves a task by ID
func (s *InMemoryTaskStore) Get(ctx context.Context, taskID string) (*ports.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// Update updates task state
func (s *InMemoryTaskStore) Update(ctx context.Context, task *ports.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}

// List returns tasks with pagination
func (s *InMemoryTaskStore) List(ctx context.Context, limit int, offset int) ([]*ports.Task, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert map to slice and sort by created_at (newest first)
	tasks := make([]*ports.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
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
			tasks = append(tasks, task)
		}
	}

	// Sort by created_at (newest first)
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
		return fmt.Errorf("task not found: %s", taskID)
	}

	delete(s.tasks, taskID)
	return nil
}

// SetStatus updates task status
func (s *InMemoryTaskStore) SetStatus(ctx context.Context, taskID string, status ports.TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = status

	// Update timestamps based on status
	now := time.Now()
	switch status {
	case ports.TaskStatusRunning:
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
	case ports.TaskStatusCompleted, ports.TaskStatusFailed, ports.TaskStatusCancelled:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
	}

	return nil
}

// SetError records task failure
func (s *InMemoryTaskStore) SetError(ctx context.Context, taskID string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Error = err.Error()
	task.Status = ports.TaskStatusFailed
	now := time.Now()
	task.CompletedAt = &now

	return nil
}

// SetResult stores task completion result
func (s *InMemoryTaskStore) SetResult(ctx context.Context, taskID string, result *agentPorts.TaskResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Result = result
	task.Status = ports.TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.TotalIterations = result.Iterations
	task.TokensUsed = result.TokensUsed
	task.TotalTokens = result.TokensUsed // Total tokens = final tokens used

	// Update session ID from result (in case task was created without one)
	// NOTE: With the fix in ExecuteTaskAsync, this should no longer be needed
	// but kept for backward compatibility
	if result.SessionID != "" {
		task.SessionID = result.SessionID
	}

	return nil
}

// UpdateProgress updates task execution progress
func (s *InMemoryTaskStore) UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.CurrentIteration = iteration
	task.TokensUsed = tokensUsed

	return nil
}
