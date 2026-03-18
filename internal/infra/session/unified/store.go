package unified

import (
	"context"
	"fmt"
	"sync"
	"time"

	storage "alex/internal/domain/agent/ports/storage"
)

// Store is a surface-agnostic session store with cross-surface lookup.
type Store struct {
	inner storage.SessionStore
	index *SurfaceIndex
	mu    sync.RWMutex
	nowFn func() time.Time
}

// New creates a unified store wrapping an existing SessionStore.
func New(inner storage.SessionStore, indexPath string, nowFn func() time.Time) (*Store, error) {
	idx, err := NewSurfaceIndex(indexPath, nowFn)
	if err != nil {
		return nil, fmt.Errorf("init surface index: %w", err)
	}
	return &Store{inner: inner, index: idx, nowFn: nowFn}, nil
}

func (s *Store) Create(ctx context.Context) (*storage.Session, error) {
	return s.inner.Create(ctx)
}

func (s *Store) Get(ctx context.Context, id string) (*storage.Session, error) {
	return s.inner.Get(ctx, id)
}

func (s *Store) Save(ctx context.Context, session *storage.Session) error {
	return s.inner.Save(ctx, session)
}

func (s *Store) List(ctx context.Context, limit int, offset int) ([]string, error) {
	return s.inner.List(ctx, limit, offset)
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.inner.Delete(ctx, id); err != nil {
		return err
	}
	return s.index.RemoveBySession(id)
}

// Bind associates a surface-specific ID with a session.
func (s *Store) Bind(ctx context.Context, surface Surface, surfaceID string, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.inner.Get(ctx, sessionID); err != nil {
		return fmt.Errorf("session %s not found: %w", sessionID, err)
	}
	return s.index.Bind(surface, surfaceID, sessionID)
}

// LookupByBinding finds a session by its surface binding.
func (s *Store) LookupByBinding(ctx context.Context, surface Surface, surfaceID string) (*storage.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionID, ok := s.index.Lookup(surface, surfaceID)
	if !ok {
		return nil, storage.ErrSessionNotFound
	}
	return s.inner.Get(ctx, sessionID)
}

// Handoff creates a binding for a new surface on an existing session.
func (s *Store) Handoff(ctx context.Context, sessionID string, toSurface Surface, toSurfaceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.inner.Get(ctx, sessionID); err != nil {
		return fmt.Errorf("session %s not found: %w", sessionID, err)
	}
	return s.index.Bind(toSurface, toSurfaceID, sessionID)
}

// ListBindings returns all surface bindings for a session.
func (s *Store) ListBindings(ctx context.Context, sessionID string) ([]SurfaceBinding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index.ListForSession(sessionID), nil
}
