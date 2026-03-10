// Package taskstore provides a durable file-backed implementation of task.Store.
//
// It stores all task records in a single JSON file and uses an in-memory map
// for fast access. Mutations are atomically persisted to disk so state
// survives process restarts.
package taskstore

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"alex/internal/domain/task"
	"alex/internal/infra/filestore"
	"alex/internal/shared/logging"
)

const (
	defaultRetention    = 7 * 24 * time.Hour
	defaultMaxTasks     = 10000
	defaultEvictInterval = 5 * time.Minute
	persistVersion      = 1
)

// LocalStore implements task.Store with an in-memory map backed by a JSON file.
type LocalStore struct {
	mu     sync.RWMutex
	tasks  map[string]*task.Task
	owners map[string]string
	leases map[string]time.Time

	// Transition log kept in memory; persisted alongside tasks.
	transitions map[string][]task.Transition
	nextTransID int64

	filePath  string
	retention time.Duration
	maxTasks  int
	logger    logging.Logger

	stopOnce sync.Once
	stopCh   chan struct{}
}

// Option configures a LocalStore.
type Option func(*LocalStore)

// WithFilePath sets the persistence file path.
func WithFilePath(path string) Option {
	return func(s *LocalStore) { s.filePath = path }
}

// WithRetention sets how long terminal tasks are kept before eviction.
func WithRetention(d time.Duration) Option {
	return func(s *LocalStore) { s.retention = d }
}

// WithMaxTasks sets the hard cap on stored tasks.
func WithMaxTasks(n int) Option {
	return func(s *LocalStore) { s.maxTasks = n }
}

// New creates a new LocalStore. Call Close() to stop background eviction.
func New(opts ...Option) *LocalStore {
	s := &LocalStore{
		tasks:       make(map[string]*task.Task),
		owners:      make(map[string]string),
		leases:      make(map[string]time.Time),
		transitions: make(map[string][]task.Transition),
		retention:   defaultRetention,
		maxTasks:    defaultMaxTasks,
		logger:      logging.NewComponentLogger("taskstore"),
		stopCh:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.loadFromDisk()
	go s.evictLoop()
	return s
}

// Close stops the background eviction goroutine.
func (s *LocalStore) Close() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// EnsureSchema is a no-op for the file-backed store.
func (s *LocalStore) EnsureSchema(_ context.Context) error { return nil }

// persistedStore is the on-disk JSON structure.
type persistedStore struct {
	Version     int                          `json:"version"`
	Tasks       []*task.Task                 `json:"tasks"`
	Transitions map[string][]task.Transition `json:"transitions,omitempty"`
}

func (s *LocalStore) loadFromDisk() {
	if s.filePath == "" {
		return
	}
	data, err := filestore.ReadFileOrEmpty(s.filePath)
	if err != nil {
		s.logger.Warn("failed to load task file %s: %v", s.filePath, err)
		return
	}
	if len(data) == 0 {
		return
	}

	var persisted persistedStore
	if err := json.Unmarshal(data, &persisted); err != nil {
		s.logger.Warn("failed to parse task file %s: %v", s.filePath, err)
		return
	}

	for _, t := range persisted.Tasks {
		if t == nil || t.TaskID == "" {
			continue
		}
		cp := *t
		s.tasks[t.TaskID] = &cp
	}
	if persisted.Transitions != nil {
		s.transitions = persisted.Transitions
	}

	// Compute next transition ID.
	for _, trs := range s.transitions {
		for _, tr := range trs {
			if tr.ID >= s.nextTransID {
				s.nextTransID = tr.ID + 1
			}
		}
	}
}

func (s *LocalStore) persistLocked() {
	if s.filePath == "" {
		return
	}

	snapshot := make([]*task.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		cp := *t
		snapshot = append(snapshot, &cp)
	}

	payload := persistedStore{
		Version:     persistVersion,
		Tasks:       snapshot,
		Transitions: s.transitions,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("failed to encode task store: %v", err)
		return
	}

	if err := filestore.AtomicWrite(s.filePath, data, 0o600); err != nil {
		s.logger.Warn("failed to persist task store to %s: %v", s.filePath, err)
	}
}

func (s *LocalStore) evictLoop() {
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

func (s *LocalStore) evictExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for id, t := range s.tasks {
		if !t.Status.IsTerminal() {
			continue
		}
		if t.CompletedAt != nil && now.Sub(*t.CompletedAt) > s.retention {
			delete(s.tasks, id)
			delete(s.transitions, id)
			changed = true
		}
	}

	if len(s.tasks) > s.maxTasks {
		s.evictOldestTerminalLocked()
		changed = true
	}

	if changed {
		s.persistLocked()
	}
}

func (s *LocalStore) evictOldestTerminalLocked() {
	type candidate struct {
		id          string
		completedAt time.Time
	}
	var cands []candidate
	for id, t := range s.tasks {
		if t.Status.IsTerminal() && t.CompletedAt != nil {
			cands = append(cands, candidate{id: id, completedAt: *t.CompletedAt})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		return cands[i].completedAt.Before(cands[j].completedAt)
	})

	toRemove := len(s.tasks) - s.maxTasks
	for i := 0; i < toRemove && i < len(cands); i++ {
		delete(s.tasks, cands[i].id)
		delete(s.transitions, cands[i].id)
	}
}

func (s *LocalStore) copyTask(t *task.Task) *task.Task {
	cp := *t
	return &cp
}

func (s *LocalStore) addTransitionLocked(taskID string, from, to task.Status, params task.TransitionParams) {
	tr := task.Transition{
		ID:         s.nextTransID,
		TaskID:     taskID,
		FromStatus: from,
		ToStatus:   to,
		Reason:     params.Reason,
		CreatedAt:  time.Now(),
	}
	if params.Metadata != nil {
		if raw, err := json.Marshal(params.Metadata); err == nil {
			tr.MetadataJSON = raw
		}
	}
	s.nextTransID++
	s.transitions[taskID] = append(s.transitions[taskID], tr)
}
