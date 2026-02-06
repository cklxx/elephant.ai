package app

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"alex/internal/delivery/server/ports"
	agent "alex/internal/domain/agent/ports/agent"
	id "alex/internal/shared/utils/id"
)

const (
	defaultTaskRetention = 24 * time.Hour
	defaultMaxTasks      = 10000
	defaultEvictInterval = 5 * time.Minute
)

// InMemoryTaskStore implements TaskStore with in-memory storage and TTL-based
// eviction for terminal tasks (completed, failed, cancelled).
type InMemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*ports.Task

	retention time.Duration // how long terminal tasks are kept
	maxSize   int           // hard cap on total tasks

	stopOnce sync.Once
	stopCh   chan struct{}
}

// TaskStoreOption configures an InMemoryTaskStore.
type TaskStoreOption func(*InMemoryTaskStore)

// WithTaskRetention sets how long terminal tasks are retained before eviction.
func WithTaskRetention(d time.Duration) TaskStoreOption {
	return func(s *InMemoryTaskStore) { s.retention = d }
}

// WithMaxTasks sets the hard cap on total stored tasks.
func WithMaxTasks(n int) TaskStoreOption {
	return func(s *InMemoryTaskStore) { s.maxSize = n }
}

// NewInMemoryTaskStore creates a new in-memory task store with optional TTL
// eviction. Call Close() to stop the background eviction goroutine.
func NewInMemoryTaskStore(opts ...TaskStoreOption) *InMemoryTaskStore {
	s := &InMemoryTaskStore{
		tasks:     make(map[string]*ports.Task),
		retention: defaultTaskRetention,
		maxSize:   defaultMaxTasks,
		stopCh:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	go s.evictLoop()
	return s
}

// Close stops the background eviction goroutine.
func (s *InMemoryTaskStore) Close() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// evictLoop periodically removes expired terminal tasks.
func (s *InMemoryTaskStore) evictLoop() {
	ticker := time.NewTicker(defaultEvictInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.evictExpired()
		}
	}
}

// evictExpired removes terminal tasks older than retention, respecting maxSize.
func (s *InMemoryTaskStore) evictExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for taskID, task := range s.tasks {
		if !isTerminalStatus(task.Status) {
			continue
		}
		if task.CompletedAt != nil && now.Sub(*task.CompletedAt) > s.retention {
			delete(s.tasks, taskID)
		}
	}

	// If still over maxSize, evict oldest terminal tasks.
	if len(s.tasks) <= s.maxSize {
		return
	}
	s.evictOldestTerminalLocked()
}

// evictOldestTerminalLocked removes the oldest terminal tasks to bring the
// store back under maxSize. Caller must hold s.mu.
func (s *InMemoryTaskStore) evictOldestTerminalLocked() {
	type candidate struct {
		id          string
		completedAt time.Time
	}
	var candidates []candidate
	for taskID, task := range s.tasks {
		if isTerminalStatus(task.Status) && task.CompletedAt != nil {
			candidates = append(candidates, candidate{id: taskID, completedAt: *task.CompletedAt})
		}
	}
	// Sort oldest first.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].completedAt.Before(candidates[j].completedAt)
	})

	toRemove := len(s.tasks) - s.maxSize
	for i := 0; i < toRemove && i < len(candidates); i++ {
		delete(s.tasks, candidates[i].id)
	}
}

func isTerminalStatus(status ports.TaskStatus) bool {
	switch status {
	case ports.TaskStatusCompleted, ports.TaskStatusFailed, ports.TaskStatusCancelled:
		return true
	default:
		return false
	}
}

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

// Delete removes a task
func (s *InMemoryTaskStore) Delete(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
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
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.Status = status

	// Update timestamps and termination reason based on status
	now := time.Now()
	switch status {
	case ports.TaskStatusRunning:
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
	case ports.TaskStatusCompleted:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		if task.TerminationReason == ports.TerminationReasonNone {
			task.TerminationReason = ports.TerminationReasonCompleted
		}
	case ports.TaskStatusCancelled:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		if task.TerminationReason == ports.TerminationReasonNone {
			task.TerminationReason = ports.TerminationReasonCancelled
		}
	case ports.TaskStatusFailed:
		if task.CompletedAt == nil {
			task.CompletedAt = &now
		}
		if task.TerminationReason == ports.TerminationReasonNone {
			task.TerminationReason = ports.TerminationReasonError
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
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.Error = err.Error()
	task.Status = ports.TaskStatusFailed
	task.TerminationReason = ports.TerminationReasonError
	now := time.Now()
	task.CompletedAt = &now

	return nil
}

// SetResult stores task completion result
func (s *InMemoryTaskStore) SetResult(ctx context.Context, taskID string, result *agent.TaskResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.Result = result
	task.Status = ports.TaskStatusCompleted
	task.TerminationReason = ports.TerminationReasonCompleted
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

	if result.ParentRunID != "" {
		task.ParentTaskID = result.ParentRunID
	}

	return nil
}

// UpdateProgress updates task execution progress
func (s *InMemoryTaskStore) UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.CurrentIteration = iteration
	task.TokensUsed = tokensUsed

	return nil
}

// SetTerminationReason sets the termination reason for a task
func (s *InMemoryTaskStore) SetTerminationReason(ctx context.Context, taskID string, reason ports.TerminationReason) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NotFoundError(fmt.Sprintf("task %s", taskID))
	}

	task.TerminationReason = reason

	return nil
}
