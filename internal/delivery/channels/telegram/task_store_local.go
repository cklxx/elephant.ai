package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

type taskStoreDoc struct {
	Tasks []TaskRecord `json:"tasks"`
}

// TaskLocalStore is a local (memory/file) TaskStore.
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
	return &TaskLocalStore{
		tasks:           make(map[string]TaskRecord),
		retention:       retention,
		maxTasksPerChat: maxTasksPerChat,
		now:             time.Now,
	}
}

// NewTaskFileStore creates a file-backed task store.
func NewTaskFileStore(dir string, retention time.Duration, maxTasksPerChat int) (*TaskLocalStore, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("task file store dir is required")
	}
	if err := filestore.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("create task store dir: %w", err)
	}
	s := &TaskLocalStore{
		tasks:           make(map[string]TaskRecord),
		filePath:        dir + "/telegram_tasks.json",
		retention:       retention,
		maxTasksPerChat: maxTasksPerChat,
		now:             time.Now,
	}
	if err := s.loadFromFile(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *TaskLocalStore) EnsureSchema(_ context.Context) error {
	return nil
}

func (s *TaskLocalStore) SaveTask(_ context.Context, task TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	s.tasks[task.TaskID] = task
	s.evictLocked()
	return s.persistLocked()
}

func (s *TaskLocalStore) UpdateStatus(_ context.Context, taskID, status string, opts ...TaskUpdateOption) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	o := resolveTaskUpdateOptions(opts)
	task.Status = status
	task.UpdatedAt = s.clock()
	if o.answerPreview != "" {
		task.AnswerPreview = o.answerPreview
	}
	if o.errorText != "" {
		task.Error = o.errorText
	}
	if o.tokensUsed > 0 {
		task.TokensUsed = o.tokensUsed
	}
	if status == "completed" || status == "failed" || status == "cancelled" {
		task.CompletedAt = s.clock()
	}
	s.tasks[taskID] = task
	return s.persistLocked()
}

func (s *TaskLocalStore) GetTask(_ context.Context, taskID string) (TaskRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[taskID]
	return task, ok, nil
}

func (s *TaskLocalStore) ListByChat(_ context.Context, chatID int64, activeOnly bool, limit int) ([]TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []TaskRecord
	for _, task := range s.tasks {
		if task.ChatID != chatID {
			continue
		}
		if activeOnly && isTerminalStatus(task.Status) {
			continue
		}
		result = append(result, task)
	}
	// Sort by CreatedAt descending.
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *TaskLocalStore) DeleteExpired(_ context.Context, before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, task := range s.tasks {
		if isTerminalStatus(task.Status) && task.UpdatedAt.Before(before) {
			delete(s.tasks, id)
		}
	}
	return s.persistLocked()
}

func (s *TaskLocalStore) MarkStaleRunning(_ context.Context, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	changed := false
	for id, task := range s.tasks {
		if task.Status == "running" || task.Status == "pending" {
			task.Status = "failed"
			task.Error = reason
			task.UpdatedAt = now
			task.CompletedAt = now
			s.tasks[id] = task
			changed = true
		}
	}
	if changed {
		return s.persistLocked()
	}
	return nil
}

func (s *TaskLocalStore) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *TaskLocalStore) evictLocked() {
	now := s.clock()
	// Evict by retention.
	if s.retention > 0 {
		cutoff := now.Add(-s.retention)
		for id, task := range s.tasks {
			if isTerminalStatus(task.Status) && task.UpdatedAt.Before(cutoff) {
				delete(s.tasks, id)
			}
		}
	}
	// Evict by max per chat.
	if s.maxTasksPerChat <= 0 {
		return
	}
	chatCounts := make(map[int64]int)
	for _, task := range s.tasks {
		chatCounts[task.ChatID]++
	}
	for chatID, count := range chatCounts {
		if count <= s.maxTasksPerChat {
			continue
		}
		// Collect and sort completed tasks for this chat.
		var completed []string
		for id, task := range s.tasks {
			if task.ChatID == chatID && isTerminalStatus(task.Status) {
				completed = append(completed, id)
			}
		}
		excess := count - s.maxTasksPerChat
		if excess > len(completed) {
			excess = len(completed)
		}
		for i := 0; i < excess; i++ {
			delete(s.tasks, completed[i])
		}
	}
}

func (s *TaskLocalStore) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	doc := taskStoreDoc{Tasks: make([]TaskRecord, 0, len(s.tasks))}
	for _, task := range s.tasks {
		doc.Tasks = append(doc.Tasks, task)
	}
	data, err := filestore.MarshalJSONIndent(doc)
	if err != nil {
		return fmt.Errorf("marshal task store: %w", err)
	}
	return filestore.AtomicWrite(s.filePath, data, 0o600)
}

func (s *TaskLocalStore) loadFromFile() error {
	if s.filePath == "" {
		return nil
	}
	data, err := filestore.ReadFileOrEmpty(s.filePath)
	if err != nil {
		return fmt.Errorf("read task store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var doc taskStoreDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode task store: %w", err)
	}
	for _, task := range doc.Tasks {
		s.tasks[task.TaskID] = task
	}
	return nil
}

func isTerminalStatus(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}

var _ TaskStore = (*TaskLocalStore)(nil)
