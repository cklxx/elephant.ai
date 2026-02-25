package preparation

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

type strictSessionStore struct {
	sessions map[string]*storage.Session
	saveErr  error
	saveCalls int
}

func newStrictSessionStore() *strictSessionStore {
	return &strictSessionStore{
		sessions: make(map[string]*storage.Session),
	}
}

func (s *strictSessionStore) Create(context.Context) (*storage.Session, error) {
	now := time.Now()
	session := &storage.Session{
		ID:        "session-created",
		Messages:  []ports.Message{},
		Todos:     []storage.Todo{},
		Metadata:  map[string]string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.sessions[session.ID] = cloneStrictSession(session)
	return cloneStrictSession(session), nil
}

func (s *strictSessionStore) Get(_ context.Context, id string) (*storage.Session, error) {
	session, ok := s.sessions[id]
	if !ok {
		return nil, storage.ErrSessionNotFound
	}
	return cloneStrictSession(session), nil
}

func (s *strictSessionStore) Save(_ context.Context, session *storage.Session) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saveCalls++
	s.sessions[session.ID] = cloneStrictSession(session)
	return nil
}

func (s *strictSessionStore) List(context.Context, int, int) ([]string, error) {
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *strictSessionStore) Delete(_ context.Context, id string) error {
	delete(s.sessions, id)
	return nil
}

func cloneStrictSession(session *storage.Session) *storage.Session {
	if session == nil {
		return nil
	}
	meta := make(map[string]string, len(session.Metadata))
	for k, v := range session.Metadata {
		meta[k] = v
	}
	return &storage.Session{
		ID:        session.ID,
		Messages:  append([]ports.Message(nil), session.Messages...),
		Todos:     append([]storage.Todo(nil), session.Todos...),
		Metadata:  meta,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
}

func TestLoadSessionCreatesExplicitMissingSession(t *testing.T) {
	store := newStrictSessionStore()
	now := time.Date(2026, time.February, 10, 22, 0, 0, 0, time.UTC)
	service := &ExecutionPreparationService{
		sessionStore: store,
		logger:       agent.NoopLogger{},
		clock:        newMockClock(now),
	}

	session, err := service.loadSession(context.Background(), "lark-missing-session")
	if err != nil {
		t.Fatalf("loadSession returned error: %v", err)
	}
	if session.ID != "lark-missing-session" {
		t.Fatalf("expected explicit session ID to be preserved, got %q", session.ID)
	}
	if store.saveCalls != 1 {
		t.Fatalf("expected one save call, got %d", store.saveCalls)
	}

	persisted, err := store.Get(context.Background(), "lark-missing-session")
	if err != nil {
		t.Fatalf("persisted session missing: %v", err)
	}
	if persisted.CreatedAt.IsZero() || !persisted.CreatedAt.Equal(now) {
		t.Fatalf("expected created_at to use service clock, got %s", persisted.CreatedAt)
	}
	if persisted.Metadata == nil {
		t.Fatalf("expected metadata to be initialized")
	}
}

func TestLoadSessionUsesExistingSessionWithoutSave(t *testing.T) {
	store := newStrictSessionStore()
	existing := &storage.Session{
		ID:        "lark-existing",
		Messages:  []ports.Message{{Role: "user", Content: "hello"}},
		Todos:     []storage.Todo{},
		Metadata:  map[string]string{"title": "Existing"},
		CreatedAt: time.Date(2026, time.February, 9, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.February, 9, 10, 0, 0, 0, time.UTC),
	}
	store.sessions[existing.ID] = cloneStrictSession(existing)

	service := &ExecutionPreparationService{
		sessionStore: store,
		logger:       agent.NoopLogger{},
		clock:        newMockClock(time.Now()),
	}

	session, err := service.loadSession(context.Background(), existing.ID)
	if err != nil {
		t.Fatalf("loadSession returned error: %v", err)
	}
	if session.ID != existing.ID {
		t.Fatalf("expected existing session, got %q", session.ID)
	}
	if store.saveCalls != 0 {
		t.Fatalf("expected no save call for existing session, got %d", store.saveCalls)
	}
}

func TestLoadSessionReturnsErrorWhenAutoCreateSaveFails(t *testing.T) {
	store := newStrictSessionStore()
	store.saveErr = errors.New("save failed")

	service := &ExecutionPreparationService{
		sessionStore: store,
		logger:       agent.NoopLogger{},
		clock:        newMockClock(time.Now()),
	}

	session, err := service.loadSession(context.Background(), "lark-save-fail")
	if err == nil {
		t.Fatalf("expected error when save fails")
	}
	if !errors.Is(err, store.saveErr) {
		t.Fatalf("expected wrapped save error, got %v", err)
	}
	if session != nil {
		t.Fatalf("expected nil session on save failure")
	}
}
