package lark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	jsonx "alex/internal/shared/json"
)

const (
	defaultTaskStoreRetention       = 7 * 24 * time.Hour
	defaultTaskStoreMaxTasksPerChat = 200
)

type taskStoreDoc struct {
	Tasks []TaskRecord `json:"tasks"`
}

// TaskLocalStore is a local (memory/file) implementation of TaskStore.
// When filePath is empty the store is in-memory only.
type TaskLocalStore struct {
	mu              sync.RWMutex
	tasks           map[string]TaskRecord
	filePath        string
	retention       time.Duration
	maxTasksPerChat int
	now             func() time.Time
}

// NewTaskMemoryStore creates an in-memory task store.
func NewTaskMemoryStore(retention time.Duration, maxTasksPerChat int) *TaskLocalStore {
	return newTaskLocalStore("", retention, maxTasksPerChat)
}

// NewTaskFileStore creates a file-backed task store under dir/tasks.json.
func NewTaskFileStore(dir string, retention time.Duration, maxTasksPerChat int) (*TaskLocalStore, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("task file store dir is required")
	}
	if err := os.MkdirAll(trimmedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create task file store dir: %w", err)
	}
	store := newTaskLocalStore(filepath.Join(trimmedDir, "tasks.json"), retention, maxTasksPerChat)
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func newTaskLocalStore(filePath string, retention time.Duration, maxTasksPerChat int) *TaskLocalStore {
	if retention <= 0 {
		retention = defaultTaskStoreRetention
	}
	if maxTasksPerChat <= 0 {
		maxTasksPerChat = defaultTaskStoreMaxTasksPerChat
	}
	return &TaskLocalStore{
		tasks:           make(map[string]TaskRecord),
		filePath:        filePath,
		retention:       retention,
		maxTasksPerChat: maxTasksPerChat,
		now:             time.Now,
	}
}

// EnsureSchema validates file store readiness. Memory mode is no-op.
func (s *TaskLocalStore) EnsureSchema(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("task store not initialized")
	}
	if s.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("ensure task store directory: %w", err)
	}
	return nil
}

// SaveTask inserts or updates a task record.
func (s *TaskLocalStore) SaveTask(ctx context.Context, task TaskRecord) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("task store not initialized")
	}
	if strings.TrimSpace(task.TaskID) == "" || strings.TrimSpace(task.ChatID) == "" {
		return fmt.Errorf("task_id and chat_id required")
	}
	now := s.now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if strings.TrimSpace(task.Status) == "" {
		task.Status = "pending"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.TaskID] = task
	s.evictLocked(now)
	return s.persistLocked()
}

// UpdateStatus updates the task lifecycle status and optional fields.
func (s *TaskLocalStore) UpdateStatus(ctx context.Context, taskID, status string, opts ...TaskUpdateOption) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("task store not initialized")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task_id required")
	}

	values := ResolveTaskUpdateOptions(opts)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return nil
	}
	task.Status = strings.TrimSpace(status)
	task.UpdatedAt = now
	if isTerminalTaskStatus(task.Status) {
		task.CompletedAt = now
	}
	if values.AnswerPreview != nil {
		task.AnswerPreview = *values.AnswerPreview
	}
	if values.ErrorText != nil {
		task.Error = *values.ErrorText
	}
	if values.TokensUsed != nil {
		task.TokensUsed = *values.TokensUsed
	}
	if values.MergeStatus != nil {
		task.MergeStatus = *values.MergeStatus
	}
	s.tasks[taskID] = task
	s.evictLocked(now)
	return s.persistLocked()
}

// GetTask loads a task by id.
func (s *TaskLocalStore) GetTask(ctx context.Context, taskID string) (TaskRecord, bool, error) {
	if ctx != nil && ctx.Err() != nil {
		return TaskRecord{}, false, ctx.Err()
	}
	if s == nil {
		return TaskRecord{}, false, fmt.Errorf("task store not initialized")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TaskRecord{}, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[taskID]
	return task, ok, nil
}

// ListByChat returns tasks for a chat, newest first.
func (s *TaskLocalStore) ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]TaskRecord, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if s == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TaskRecord, 0, limit)
	for _, rec := range s.tasks {
		if rec.ChatID != chatID {
			continue
		}
		if activeOnly && isTerminalTaskStatus(rec.Status) {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// DeleteExpired removes tasks created before the cutoff.
func (s *TaskLocalStore) DeleteExpired(ctx context.Context, before time.Time) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("task store not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, rec := range s.tasks {
		if rec.CreatedAt.Before(before) {
			delete(s.tasks, id)
		}
	}
	return s.persistLocked()
}

// MarkStaleRunning marks non-terminal tasks as failed.
func (s *TaskLocalStore) MarkStaleRunning(ctx context.Context, reason string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("task store not initialized")
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, rec := range s.tasks {
		if rec.Status == "pending" || rec.Status == "running" || rec.Status == "waiting_input" {
			rec.Status = "failed"
			rec.Error = reason
			rec.UpdatedAt = now
			rec.CompletedAt = now
			s.tasks[id] = rec
		}
	}
	return s.persistLocked()
}

func (s *TaskLocalStore) evictLocked(now time.Time) {
	for id, rec := range s.tasks {
		if !isTerminalTaskStatus(rec.Status) {
			continue
		}
		completedAt := rec.CompletedAt
		if completedAt.IsZero() {
			completedAt = rec.UpdatedAt
		}
		if completedAt.IsZero() {
			completedAt = rec.CreatedAt
		}
		if now.Sub(completedAt) > s.retention {
			delete(s.tasks, id)
		}
	}

	if s.maxTasksPerChat <= 0 {
		return
	}
	type taskBuckets struct {
		active   []TaskRecord
		terminal []TaskRecord
	}
	byChat := make(map[string]*taskBuckets)
	for _, rec := range s.tasks {
		buckets, ok := byChat[rec.ChatID]
		if !ok {
			buckets = &taskBuckets{}
			byChat[rec.ChatID] = buckets
		}
		if isTerminalTaskStatus(rec.Status) {
			buckets.terminal = append(buckets.terminal, rec)
			continue
		}
		buckets.active = append(buckets.active, rec)
	}
	for chatID, buckets := range byChat {
		total := len(buckets.active) + len(buckets.terminal)
		if total <= s.maxTasksPerChat {
			continue
		}
		keep := make(map[string]struct{}, total)
		for _, rec := range buckets.active {
			keep[rec.TaskID] = struct{}{}
		}

		terminalAllowance := s.maxTasksPerChat - len(buckets.active)
		if terminalAllowance < 0 {
			terminalAllowance = 0
		}
		sort.Slice(buckets.terminal, func(i, j int) bool {
			return buckets.terminal[i].CreatedAt.After(buckets.terminal[j].CreatedAt)
		})
		for i := 0; i < terminalAllowance && i < len(buckets.terminal); i++ {
			keep[buckets.terminal[i].TaskID] = struct{}{}
		}

		for id, rec := range s.tasks {
			if rec.ChatID != chatID {
				continue
			}
			if _, ok := keep[id]; !ok {
				delete(s.tasks, id)
			}
		}
	}
}

func (s *TaskLocalStore) load() error {
	if s.filePath == "" {
		return nil
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read task store: %w", err)
	}
	var doc taskStoreDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode task store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range doc.Tasks {
		if strings.TrimSpace(rec.TaskID) == "" {
			continue
		}
		s.tasks[rec.TaskID] = rec
	}
	s.evictLocked(s.now())
	return nil
}

func (s *TaskLocalStore) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	doc := taskStoreDoc{
		Tasks: make([]TaskRecord, 0, len(s.tasks)),
	}
	for _, rec := range s.tasks {
		doc.Tasks = append(doc.Tasks, rec)
	}
	sort.Slice(doc.Tasks, func(i, j int) bool {
		return doc.Tasks[i].CreatedAt.Before(doc.Tasks[j].CreatedAt)
	})
	data, err := jsonx.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode task store: %w", err)
	}
	data = append(data, '\n')
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write task store temp: %w", err)
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit task store: %w", err)
	}
	return nil
}

func isTerminalTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

var _ TaskStore = (*TaskLocalStore)(nil)
