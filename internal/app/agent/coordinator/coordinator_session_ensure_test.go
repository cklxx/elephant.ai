package coordinator

import (
	"context"
	"errors"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	core "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
)

type ensureSessionStore struct {
	sessions    map[string]*storage.Session
	createID    string
	createErr   error
	getErr      error
	saveErr     error
	createCalls int
	saveCalls   int
}

func (s *ensureSessionStore) Create(_ context.Context) (*storage.Session, error) {
	s.createCalls++
	if s.createErr != nil {
		return nil, s.createErr
	}
	id := s.createID
	if id == "" {
		id = "session-created"
	}
	if s.sessions == nil {
		s.sessions = map[string]*storage.Session{}
	}
	session := &storage.Session{ID: id, Messages: []core.Message{}, Todos: []storage.Todo{}, Metadata: map[string]string{}}
	s.sessions[id] = session
	return session, nil
}

func (s *ensureSessionStore) Get(_ context.Context, id string) (*storage.Session, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.sessions != nil {
		if session, ok := s.sessions[id]; ok {
			return session, nil
		}
	}
	return nil, storage.ErrSessionNotFound
}

func (s *ensureSessionStore) Save(_ context.Context, session *storage.Session) error {
	s.saveCalls++
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.sessions == nil {
		s.sessions = map[string]*storage.Session{}
	}
	s.sessions[session.ID] = session
	return nil
}

func (s *ensureSessionStore) List(_ context.Context, _ int, _ int) ([]string, error) {
	return nil, nil
}

func (s *ensureSessionStore) Delete(_ context.Context, _ string) error {
	return nil
}

func TestAgentCoordinatorEnsureSessionReturnsExisting(t *testing.T) {
	store := &ensureSessionStore{sessions: map[string]*storage.Session{}}
	store.sessions["lark-1"] = &storage.Session{ID: "lark-1", Metadata: map[string]string{"ok": "true"}}

	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})
	session, err := coordinator.EnsureSession(context.Background(), "lark-1")
	if err != nil {
		t.Fatalf("EnsureSession error = %v", err)
	}
	if session.ID != "lark-1" {
		t.Fatalf("expected session id lark-1, got %q", session.ID)
	}
	if store.saveCalls != 0 {
		t.Fatalf("expected no Save calls, got %d", store.saveCalls)
	}
}

func TestAgentCoordinatorEnsureSessionCreatesMissing(t *testing.T) {
	store := &ensureSessionStore{sessions: map[string]*storage.Session{}}
	fixedTime := time.Date(2026, 1, 29, 10, 0, 0, 0, time.UTC)

	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{}, WithClock(agent.ClockFunc(func() time.Time {
		return fixedTime
	})))

	session, err := coordinator.EnsureSession(context.Background(), "lark-2")
	if err != nil {
		t.Fatalf("EnsureSession error = %v", err)
	}
	if session.ID != "lark-2" {
		t.Fatalf("expected session id lark-2, got %q", session.ID)
	}
	if session.Metadata == nil {
		t.Fatalf("expected metadata initialized")
	}
	if session.CreatedAt != fixedTime || session.UpdatedAt != fixedTime {
		t.Fatalf("expected timestamps to be %v, got created=%v updated=%v", fixedTime, session.CreatedAt, session.UpdatedAt)
	}
	if len(session.Messages) != 0 || len(session.Todos) != 0 {
		t.Fatalf("expected empty messages/todos")
	}
	if store.saveCalls != 1 {
		t.Fatalf("expected Save to be called once, got %d", store.saveCalls)
	}
}

func TestAgentCoordinatorEnsureSessionCreatesWhenIDEmpty(t *testing.T) {
	store := &ensureSessionStore{createID: "session-new"}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	session, err := coordinator.EnsureSession(context.Background(), "")
	if err != nil {
		t.Fatalf("EnsureSession error = %v", err)
	}
	if session.ID != "session-new" {
		t.Fatalf("expected session id session-new, got %q", session.ID)
	}
	if store.createCalls != 1 {
		t.Fatalf("expected Create to be called once, got %d", store.createCalls)
	}
}

func TestAgentCoordinatorEnsureSessionPropagatesErrors(t *testing.T) {
	store := &ensureSessionStore{getErr: errors.New("boom")}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	if _, err := coordinator.EnsureSession(context.Background(), "lark-3"); err == nil {
		t.Fatalf("expected error")
	}
}
