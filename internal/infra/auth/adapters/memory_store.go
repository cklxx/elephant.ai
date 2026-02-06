package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/domain/auth"
)

// NewMemoryStores creates repositories backed by in-memory maps.
func NewMemoryStores() (*memoryUserRepo, *memoryIdentityRepo, *memorySessionRepo, *memoryStateStore) {
	users := &memoryUserRepo{users: map[string]domain.User{}, emailIdx: map[string]string{}}
	identities := &memoryIdentityRepo{identities: map[string]domain.Identity{}, providerIdx: map[string]string{}}
	sessions := &memorySessionRepo{sessions: map[string]domain.Session{}, fingerprintIdx: map[string]string{}, verifier: func(string, string) (bool, error) {
		return false, fmt.Errorf("refresh token verifier not configured")
	}}
	states := &memoryStateStore{states: map[string]stateRecord{}}
	return users, identities, sessions, states
}

type memoryUserRepo struct {
	mu       sync.RWMutex
	users    map[string]domain.User
	emailIdx map[string]string
}

func (r *memoryUserRepo) Create(_ context.Context, user domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.emailIdx[user.Email]; exists {
		return domain.User{}, domain.ErrUserExists
	}
	r.users[user.ID] = user
	r.emailIdx[user.Email] = user.ID
	return user, nil
}

func (r *memoryUserRepo) Update(_ context.Context, user domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[user.ID]; !exists {
		return domain.User{}, domain.ErrUserNotFound
	}
	r.users[user.ID] = user
	r.emailIdx[user.Email] = user.ID
	return user, nil
}

func (r *memoryUserRepo) FindByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.emailIdx[email]; ok {
		return r.users[id], nil
	}
	return domain.User{}, domain.ErrUserNotFound
}

func (r *memoryUserRepo) FindByID(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrUserNotFound
}

type memoryIdentityRepo struct {
	mu          sync.RWMutex
	identities  map[string]domain.Identity
	providerIdx map[string]string
}

func key(provider domain.ProviderType, providerID string) string {
	return string(provider) + ":" + providerID
}

func (r *memoryIdentityRepo) Create(_ context.Context, identity domain.Identity) (domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := key(identity.Provider, identity.ProviderID)
	r.identities[identity.ID] = identity
	r.providerIdx[idx] = identity.ID
	return identity, nil
}

func (r *memoryIdentityRepo) Update(_ context.Context, identity domain.Identity) (domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.identities[identity.ID]; !ok {
		return domain.Identity{}, domain.ErrIdentityNotFound
	}
	r.identities[identity.ID] = identity
	idx := key(identity.Provider, identity.ProviderID)
	r.providerIdx[idx] = identity.ID
	return identity, nil
}

func (r *memoryIdentityRepo) FindByProvider(_ context.Context, provider domain.ProviderType, providerID string) (domain.Identity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	idx := key(provider, providerID)
	if id, ok := r.providerIdx[idx]; ok {
		return r.identities[id], nil
	}
	return domain.Identity{}, domain.ErrIdentityNotFound
}

type memorySessionRepo struct {
	mu             sync.RWMutex
	sessions       map[string]domain.Session
	fingerprintIdx map[string]string
	verifier       func(string, string) (bool, error)
}

// SetVerifier configures the refresh token verification callback.
func (r *memorySessionRepo) SetVerifier(verifier func(string, string) (bool, error)) {
	if verifier == nil {
		return
	}
	r.mu.Lock()
	r.verifier = verifier
	r.mu.Unlock()
}

func (r *memorySessionRepo) Create(_ context.Context, session domain.Session) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	if session.RefreshTokenFingerprint != "" {
		r.fingerprintIdx[session.RefreshTokenFingerprint] = session.ID
	}
	return session, nil
}

func (r *memorySessionRepo) DeleteByID(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[id]; ok {
		if session.RefreshTokenFingerprint != "" {
			delete(r.fingerprintIdx, session.RefreshTokenFingerprint)
		}
	}
	delete(r.sessions, id)
	return nil
}

func (r *memorySessionRepo) DeleteByUser(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, session := range r.sessions {
		if session.UserID == userID {
			if session.RefreshTokenFingerprint != "" {
				delete(r.fingerprintIdx, session.RefreshTokenFingerprint)
			}
			delete(r.sessions, id)
		}
	}
	return nil
}

func (r *memorySessionRepo) FindByRefreshToken(_ context.Context, refreshToken string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fingerprint := domain.FingerprintRefreshToken(refreshToken)
	if id, ok := r.fingerprintIdx[fingerprint]; ok {
		session := r.sessions[id]
		match, err := r.verifier(refreshToken, session.RefreshTokenHash)
		if err != nil {
			return domain.Session{}, err
		}
		if match {
			return session, nil
		}
	}
	return domain.Session{}, domain.ErrSessionNotFound
}

type memoryStateStore struct {
	mu     sync.Mutex
	states map[string]stateRecord
}

type stateRecord struct {
	provider domain.ProviderType
	expires  time.Time
}

func (s *memoryStateStore) Save(_ context.Context, state string, provider domain.ProviderType, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state] = stateRecord{provider: provider, expires: expiresAt}
	return nil
}

func (s *memoryStateStore) Consume(_ context.Context, state string, provider domain.ProviderType) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.states[state]
	if !ok {
		return domain.ErrStateMismatch
	}
	if record.provider != provider {
		return domain.ErrStateMismatch
	}
	if !record.expires.IsZero() && time.Now().After(record.expires) {
		delete(s.states, state)
		return domain.ErrStateMismatch
	}
	delete(s.states, state)
	return nil
}

func (s *memoryStateStore) PurgeExpired(_ context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed int64
	for state, record := range s.states {
		if !record.expires.IsZero() && record.expires.Before(before) {
			delete(s.states, state)
			removed++
		}
	}
	return removed, nil
}
