package oauth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrStateNotFound = errors.New("lark oauth state not found")

// StateStore stores short-lived OAuth state values.
// States must be one-time use; Consume must remove the state.
type StateStore interface {
	Save(ctx context.Context, state string, expiresAt time.Time) error
	Consume(ctx context.Context, state string) error
}

type memoryStateStore struct {
	mu    sync.Mutex
	items map[string]time.Time
	now   func() time.Time
}

// NewMemoryStateStore returns an in-memory state store.
// Suitable for single-process deployments; states are lost on restart.
func NewMemoryStateStore() StateStore {
	return &memoryStateStore{
		items: make(map[string]time.Time),
		now:   time.Now,
	}
}

func (s *memoryStateStore) Save(ctx context.Context, state string, expiresAt time.Time) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if state == "" {
		return fmt.Errorf("state required")
	}
	if expiresAt.IsZero() {
		return fmt.Errorf("expires_at required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[state] = expiresAt
	return nil
}

func (s *memoryStateStore) Consume(ctx context.Context, state string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if state == "" {
		return fmt.Errorf("state required")
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if exp, ok := s.items[state]; ok {
		delete(s.items, state)
		if now.After(exp) {
			return ErrStateNotFound
		}
		return nil
	}
	return ErrStateNotFound
}
