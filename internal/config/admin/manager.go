package admin

import (
	"context"
	"sync"
	"time"

	runtimeconfig "alex/internal/config"
)

// Manager coordinates cached access to configuration overrides and propagates updates.
type Manager struct {
	store       Store
	ttl         time.Duration
	mu          sync.RWMutex
	cache       runtimeconfig.Overrides
	cachedAt    time.Time
	hasCache    bool
	subscribers map[chan runtimeconfig.Overrides]struct{}
}

// ManagerOption customizes manager behaviour.
type ManagerOption func(*Manager)

// WithCacheTTL sets the cache TTL. Defaults to 30 seconds.
func WithCacheTTL(ttl time.Duration) ManagerOption {
	return func(m *Manager) {
		if ttl > 0 {
			m.ttl = ttl
		}
	}
}

// NewManager constructs a manager with the provided store and optional initial overrides.
func NewManager(store Store, initial runtimeconfig.Overrides, opts ...ManagerOption) *Manager {
	m := &Manager{
		store:       store,
		ttl:         30 * time.Second,
		cache:       initial,
		cachedAt:    time.Now(),
		hasCache:    true,
		subscribers: map[chan runtimeconfig.Overrides]struct{}{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// CurrentOverrides returns cached overrides, reloading from the store when stale.
func (m *Manager) CurrentOverrides(ctx context.Context) (runtimeconfig.Overrides, error) {
	if m == nil {
		return runtimeconfig.Overrides{}, nil
	}
	m.mu.RLock()
	if m.hasCache && time.Since(m.cachedAt) < m.ttl {
		defer m.mu.RUnlock()
		return m.cache, nil
	}
	m.mu.RUnlock()

	overrides, err := m.store.LoadOverrides(ctx)
	if err != nil {
		return runtimeconfig.Overrides{}, err
	}

	m.mu.Lock()
	m.cache = overrides
	m.cachedAt = time.Now()
	m.hasCache = true
	m.mu.Unlock()

	return overrides, nil
}

// UpdateOverrides persists overrides and notifies subscribers.
func (m *Manager) UpdateOverrides(ctx context.Context, overrides runtimeconfig.Overrides) error {
	if m == nil {
		return nil
	}
	if err := m.store.SaveOverrides(ctx, overrides); err != nil {
		return err
	}
	m.mu.Lock()
	m.cache = overrides
	m.cachedAt = time.Now()
	m.hasCache = true
	subs := make([]chan runtimeconfig.Overrides, 0, len(m.subscribers))
	for ch := range m.subscribers {
		subs = append(subs, ch)
	}
	m.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- overrides:
		default:
		}
	}
	return nil
}

// Subscribe returns a channel that receives override updates and an unsubscribe func.
func (m *Manager) Subscribe() (<-chan runtimeconfig.Overrides, func()) {
	if m == nil {
		ch := make(chan runtimeconfig.Overrides)
		close(ch)
		return ch, func() {}
	}
	ch := make(chan runtimeconfig.Overrides, 1)
	m.mu.Lock()
	if m.subscribers == nil {
		m.subscribers = map[chan runtimeconfig.Overrides]struct{}{}
	}
	m.subscribers[ch] = struct{}{}
	m.mu.Unlock()

	unsubscribe := func() {
		m.mu.Lock()
		delete(m.subscribers, ch)
		m.mu.Unlock()
	}
	return ch, unsubscribe
}
