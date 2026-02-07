package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/delivery/server/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
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
	logger    logging.Logger

	persistencePath string

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

// WithTaskPersistenceFile enables task store persistence in the specified file.
func WithTaskPersistenceFile(path string) TaskStoreOption {
	return func(s *InMemoryTaskStore) { s.persistencePath = strings.TrimSpace(path) }
}

// NewInMemoryTaskStore creates a new in-memory task store with optional TTL
// eviction. Call Close() to stop the background eviction goroutine.
func NewInMemoryTaskStore(opts ...TaskStoreOption) *InMemoryTaskStore {
	s := &InMemoryTaskStore{
		tasks:     make(map[string]*ports.Task),
		retention: defaultTaskRetention,
		maxSize:   defaultMaxTasks,
		logger:    logging.NewComponentLogger("InMemoryTaskStore"),
		stopCh:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.loadFromDisk()
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

	changed := false
	for taskID, task := range s.tasks {
		if !isTerminalStatus(task.Status) {
			continue
		}
		if task.CompletedAt != nil && now.Sub(*task.CompletedAt) > s.retention {
			delete(s.tasks, taskID)
			changed = true
		}
	}

	// If still over maxSize, evict oldest terminal tasks.
	if len(s.tasks) <= s.maxSize {
		if changed {
			s.persistLocked()
		}
		return
	}
	s.evictOldestTerminalLocked()
	s.persistLocked()
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
	s.persistLocked()
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

	s.persistLocked()
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

	s.persistLocked()
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

	s.persistLocked()
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

	s.persistLocked()
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
	data, err := os.ReadFile(s.persistencePath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.logger.Warn("failed to load task persistence file %s: %v", s.persistencePath, err)
		}
		return
	}

	var persisted persistedTaskStore
	if err := json.Unmarshal(data, &persisted); err != nil {
		s.logger.Warn("failed to parse task persistence file %s: %v", s.persistencePath, err)
		return
	}

	loaded := make(map[string]*ports.Task, len(persisted.Tasks))
	for _, task := range persisted.Tasks {
		if task == nil || strings.TrimSpace(task.ID) == "" {
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

	if err := os.MkdirAll(filepath.Dir(s.persistencePath), 0o755); err != nil {
		s.logger.Warn("failed to create task persistence directory for %s: %v", s.persistencePath, err)
		return
	}

	tmpPath := fmt.Sprintf("%s.tmp-%d", s.persistencePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		s.logger.Warn("failed to write task persistence temp file %s: %v", tmpPath, err)
		return
	}
	if err := os.Rename(tmpPath, s.persistencePath); err != nil {
		_ = os.Remove(tmpPath)
		s.logger.Warn("failed to atomically persist task store to %s: %v", s.persistencePath, err)
	}
}
