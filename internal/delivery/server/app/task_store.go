package app

import (
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/delivery/server/ports"
	"alex/internal/shared/logging"
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
	// Execution ownership bookkeeping for single-process claim/lease semantics.
	owners map[string]string
	leases map[string]time.Time

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
		owners:    make(map[string]string),
		leases:    make(map[string]time.Time),
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
