package configcenter

import (
	"context"
	"errors"
	"sync"
	"time"

	"alex/internal/serverconfig"
)

// Snapshot represents the cached configuration along with metadata.
type Snapshot struct {
	Config    serverconfig.Config `json:"config"`
	Version   int64               `json:"version"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// IsZero reports whether the snapshot contains a valid configuration.
func (s Snapshot) IsZero() bool {
	return s.Version == 0 && s.UpdatedAt.IsZero()
}

// Service orchestrates access to the configuration store with caching and
// subscription support.
type Service struct {
	store Store
	ttl   time.Duration
	clock func() time.Time

	mu    sync.RWMutex
	cache Snapshot

	subsMu         sync.Mutex
	subscribers    map[uint64]chan Snapshot
	nextSubscriber uint64
}

// NewService builds a new configuration center service using the provided
// store and cache TTL.
func NewService(store Store, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Service{
		store:       store,
		ttl:         ttl,
		clock:       time.Now,
		subscribers: map[uint64]chan Snapshot{},
	}
}

// Get returns the cached configuration snapshot, refreshing it from the store
// when the cache is stale.
func (s *Service) Get(ctx context.Context) (Snapshot, error) {
	if err := contextError(ctx); err != nil {
		return Snapshot{}, err
	}

	s.mu.RLock()
	snapshot := s.cache
	if !snapshot.IsZero() && s.clock().Sub(snapshot.UpdatedAt) < s.ttl {
		s.mu.RUnlock()
		return snapshot, nil
	}
	s.mu.RUnlock()
	return s.refresh(ctx)
}

// Update persists a new configuration snapshot and notifies subscribers.
func (s *Service) Update(ctx context.Context, cfg serverconfig.Config) (Snapshot, error) {
	if err := contextError(ctx); err != nil {
		return Snapshot{}, err
	}

	if s.store == nil {
		return Snapshot{}, errors.New("config center store not configured")
	}

	if err := s.store.Save(cfg); err != nil {
		return Snapshot{}, err
	}

	s.mu.Lock()
	version := s.cache.Version + 1
	if version == 0 {
		version = 1
	}
	snapshot := Snapshot{Config: cfg.Clone(), Version: version, UpdatedAt: s.clock()}
	s.cache = snapshot
	s.mu.Unlock()

	s.notify(snapshot)
	return snapshot, nil
}

// SeedIfEmpty writes the provided configuration when the store is empty. It
// returns the existing snapshot when data is already present.
func (s *Service) SeedIfEmpty(ctx context.Context, cfg serverconfig.Config) (Snapshot, error) {
	if err := contextError(ctx); err != nil {
		return Snapshot{}, err
	}

	if s.store == nil {
		return Snapshot{}, errors.New("config center store not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cache.IsZero() {
		return s.cache, nil
	}

	existing, err := s.store.Load()
	if err == nil {
		snapshot := Snapshot{Config: existing, Version: 1, UpdatedAt: s.clock()}
		s.cache = snapshot
		return snapshot, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Snapshot{}, err
	}

	if err := s.store.Save(cfg); err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Config: cfg.Clone(), Version: 1, UpdatedAt: s.clock()}
	s.cache = snapshot
	s.notify(snapshot)
	return snapshot, nil
}

// Subscribe registers a listener for configuration changes. The returned
// channel receives the current snapshot (if available) followed by future
// updates. Call the cleanup function to stop receiving notifications.
func (s *Service) Subscribe() (<-chan Snapshot, func()) {
	ch := make(chan Snapshot, 1)

	s.subsMu.Lock()
	id := s.nextSubscriber
	s.nextSubscriber++
	s.subscribers[id] = ch
	s.subsMu.Unlock()

	s.mu.RLock()
	snapshot := s.cache
	s.mu.RUnlock()
	if !snapshot.IsZero() {
		ch <- snapshot
	}

	cleanup := func() {
		s.subsMu.Lock()
		if sub, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(sub)
		}
		s.subsMu.Unlock()
	}

	return ch, cleanup
}

func (s *Service) refresh(ctx context.Context) (Snapshot, error) {
	if err := contextError(ctx); err != nil {
		return Snapshot{}, err
	}

	if s.store == nil {
		return Snapshot{}, errors.New("config center store not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cache.IsZero() && s.clock().Sub(s.cache.UpdatedAt) < s.ttl {
		return s.cache, nil
	}

	cfg, err := s.store.Load()
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.cache = Snapshot{}
		}
		return Snapshot{}, err
	}

	version := s.cache.Version
	if version == 0 {
		version = 1
	}
	snapshot := Snapshot{Config: cfg.Clone(), Version: version, UpdatedAt: s.clock()}
	s.cache = snapshot
	return snapshot, nil
}

func (s *Service) notify(snapshot Snapshot) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
