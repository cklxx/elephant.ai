package app

import (
	"context"
	"testing"
	"time"

	"alex/internal/agent/ports"
)

type shareSessionStore struct {
	sessions map[string]*ports.Session
}

func newShareSessionStore() *shareSessionStore {
	return &shareSessionStore{sessions: make(map[string]*ports.Session)}
}

func (s *shareSessionStore) Create(ctx context.Context) (*ports.Session, error) {
	session := &ports.Session{
		ID:        "session-share",
		Messages:  []ports.Message{},
		Metadata:  map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[session.ID] = session
	return session, nil
}

func (s *shareSessionStore) Get(ctx context.Context, id string) (*ports.Session, error) {
	session, ok := s.sessions[id]
	if !ok {
		return nil, errSessionNotFound()
	}
	return session, nil
}

func (s *shareSessionStore) Save(ctx context.Context, session *ports.Session) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *shareSessionStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *shareSessionStore) Delete(ctx context.Context, id string) error {
	delete(s.sessions, id)
	return nil
}

func errSessionNotFound() error {
	return &sessionNotFoundError{}
}

type sessionNotFoundError struct{}

func (sessionNotFoundError) Error() string {
	return "session not found"
}

func TestEnsureSessionShareToken(t *testing.T) {
	store := newShareSessionStore()
	session := &ports.Session{
		ID:        "session-1",
		Metadata:  map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.sessions[session.ID] = session
	coordinator := NewServerCoordinator(nil, nil, store, nil, nil)

	token, err := coordinator.EnsureSessionShareToken(context.Background(), "session-1", false)
	if err != nil {
		t.Fatalf("ensure share token: %v", err)
	}
	if token == "" {
		t.Fatalf("expected share token to be set")
	}

	token2, err := coordinator.EnsureSessionShareToken(context.Background(), "session-1", false)
	if err != nil {
		t.Fatalf("ensure share token again: %v", err)
	}
	if token2 != token {
		t.Fatalf("expected existing token, got %q and %q", token, token2)
	}

	token3, err := coordinator.EnsureSessionShareToken(context.Background(), "session-1", true)
	if err != nil {
		t.Fatalf("reset share token: %v", err)
	}
	if token3 == token {
		t.Fatalf("expected token to reset")
	}
}

func TestValidateShareToken(t *testing.T) {
	store := newShareSessionStore()
	session := &ports.Session{
		ID:        "session-1",
		Metadata:  map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.sessions[session.ID] = session
	coordinator := NewServerCoordinator(nil, nil, store, nil, nil)

	token, err := coordinator.EnsureSessionShareToken(context.Background(), "session-1", false)
	if err != nil {
		t.Fatalf("ensure share token: %v", err)
	}

	if _, err := coordinator.ValidateShareToken(context.Background(), "session-1", "bad-token"); err != ErrShareTokenInvalid {
		t.Fatalf("expected invalid token error, got %v", err)
	}

	if _, err := coordinator.ValidateShareToken(context.Background(), "session-1", token); err != nil {
		t.Fatalf("expected token to validate, got %v", err)
	}
}
