package coordinator

import (
	"context"
	"sync"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
)

type titleUpdateStore struct {
	mu        sync.Mutex
	session   *storage.Session
	saveCh    chan struct{}
	saveCount int
}

func newTitleUpdateStore(session *storage.Session) *titleUpdateStore {
	if session == nil {
		session = &storage.Session{ID: "session-1", Metadata: map[string]string{}}
	}
	return &titleUpdateStore{
		session: session,
		saveCh:  make(chan struct{}, 1),
	}
}

func (s *titleUpdateStore) Create(ctx context.Context) (*storage.Session, error) {
	return s.Get(ctx, "")
}

func (s *titleUpdateStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		s.session = &storage.Session{ID: id, Metadata: map[string]string{}}
	}
	return s.session, nil
}

func (s *titleUpdateStore) Save(ctx context.Context, session *storage.Session) error {
	s.mu.Lock()
	s.session = session
	s.saveCount++
	s.mu.Unlock()
	select {
	case s.saveCh <- struct{}{}:
	default:
	}
	return nil
}

func (s *titleUpdateStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return []string{}, nil
	}
	return []string{s.session.ID}, nil
}

func (s *titleUpdateStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil && s.session.ID == id {
		s.session = nil
	}
	return nil
}

func TestPersistSessionTitleWritesOnce(t *testing.T) {
	store := newTitleUpdateStore(&storage.Session{ID: "session-1", Metadata: map[string]string{}})
	coordinator := &AgentCoordinator{
		sessionStore: store,
		logger:       agent.NoopLogger{},
		clock:        agent.SystemClock{},
	}

	coordinator.persistSessionTitle(context.Background(), "session-1", "Early title")

	select {
	case <-store.saveCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for title save")
	}

	store.mu.Lock()
	savedTitle := store.session.Metadata["title"]
	savedCount := store.saveCount
	store.mu.Unlock()

	if savedTitle != "Early title" {
		t.Fatalf("expected title to be saved, got %q", savedTitle)
	}
	if savedCount != 1 {
		t.Fatalf("expected one save, got %d", savedCount)
	}

	coordinator.persistSessionTitle(context.Background(), "session-1", "New title")

	select {
	case <-store.saveCh:
		t.Fatal("unexpected additional save")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPersistSessionTitleSkipsBlankTitle(t *testing.T) {
	store := newTitleUpdateStore(&storage.Session{ID: "session-2", Metadata: map[string]string{}})
	coordinator := &AgentCoordinator{
		sessionStore: store,
		logger:       agent.NoopLogger{},
		clock:        agent.SystemClock{},
	}

	coordinator.persistSessionTitle(context.Background(), "session-2", "  ")

	select {
	case <-store.saveCh:
		t.Fatal("unexpected save for blank title")
	case <-time.After(100 * time.Millisecond):
	}
}
