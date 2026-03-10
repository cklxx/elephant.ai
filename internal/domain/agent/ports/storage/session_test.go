package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	core "alex/internal/domain/agent/ports"
)

func TestNewSessionInitializesLifecycleFields(t *testing.T) {
	now := time.Date(2026, time.March, 11, 8, 0, 0, 0, time.UTC)

	session := NewSession("session-1", now)

	if session.ID != "session-1" {
		t.Fatalf("expected session id to be set, got %q", session.ID)
	}
	if session.CreatedAt != now || session.UpdatedAt != now {
		t.Fatalf("expected timestamps to equal %v, got created=%v updated=%v", now, session.CreatedAt, session.UpdatedAt)
	}
	if session.Metadata == nil {
		t.Fatal("expected metadata initialized")
	}
	if len(session.Messages) != 0 || len(session.Todos) != 0 {
		t.Fatalf("expected empty messages/todos, got %d/%d", len(session.Messages), len(session.Todos))
	}
}

func TestSessionResetClearsPersistedState(t *testing.T) {
	now := time.Date(2026, time.March, 11, 9, 0, 0, 0, time.UTC)
	session := &Session{
		ID:          "session-1",
		Messages:    []core.Message{{Role: "user", Content: "hello"}},
		Todos:       []Todo{{Description: "todo"}},
		Metadata:    map[string]string{"title": "hello"},
		Attachments: map[string]core.Attachment{"a": {Name: "a"}},
		Important:   map[string]core.ImportantNote{"note": {Content: "remember"}},
		UserPersona: &core.UserPersonaProfile{DecisionStyle: "direct"},
	}

	session.Reset(now)

	if session.Messages != nil || session.Todos != nil || session.Metadata != nil {
		t.Fatalf("expected core session state cleared, got %+v", session)
	}
	if session.Attachments != nil || session.Important != nil || session.UserPersona != nil {
		t.Fatalf("expected optional persisted state cleared, got %+v", session)
	}
	if session.UpdatedAt != now {
		t.Fatalf("expected updated_at=%v, got %v", now, session.UpdatedAt)
	}
}

func TestEnsureMetadataNilSession(t *testing.T) {
	if got := EnsureMetadata(nil); got != nil {
		t.Fatalf("expected nil metadata for nil session, got %v", got)
	}
}

func TestEnsureMetadataInitializesWhenMissing(t *testing.T) {
	session := &Session{}

	metadata := EnsureMetadata(session)
	if metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
	if session.Metadata == nil {
		t.Fatal("expected session metadata to be initialized")
	}

	metadata["k"] = "v"
	if session.Metadata["k"] != "v" {
		t.Fatalf("expected metadata mutation to persist on session, got %q", session.Metadata["k"])
	}
}

func TestEnsureMetadataPreservesExistingMap(t *testing.T) {
	session := &Session{Metadata: map[string]string{"title": "hello"}}

	metadata := EnsureMetadata(session)
	if metadata["title"] != "hello" {
		t.Fatalf("expected existing key to be preserved, got %q", metadata["title"])
	}

	metadata["title"] = "updated"
	if session.Metadata["title"] != "updated" {
		t.Fatalf("expected metadata updates to apply to session map, got %q", session.Metadata["title"])
	}
}

func TestCloneMetadataNilInput(t *testing.T) {
	if cloned := CloneMetadata(nil); cloned != nil {
		t.Fatalf("expected nil clone for nil input, got %v", cloned)
	}
}

func TestCloneMetadataEmptyInputReturnsNil(t *testing.T) {
	if cloned := CloneMetadata(map[string]string{}); cloned != nil {
		t.Fatalf("expected nil clone for empty input, got %v", cloned)
	}
}

func TestCloneMetadataDeepCopiesValues(t *testing.T) {
	original := map[string]string{"a": "1", "b": "2"}

	cloned := CloneMetadata(original)
	if len(cloned) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cloned))
	}
	if cloned["a"] != "1" || cloned["b"] != "2" {
		t.Fatalf("expected cloned values copied, got %v", cloned)
	}

	original["a"] = "mutated"
	if cloned["a"] == "mutated" {
		t.Fatal("expected clone to be independent from original mutations")
	}

	cloned["b"] = "changed"
	if original["b"] == "changed" {
		t.Fatal("expected original to be independent from clone mutations")
	}
}

// --- ClearContent ---

func TestClearContentNilSession(t *testing.T) {
	var s *Session
	s.ClearContent() // must not panic
}

func TestClearContentClearsMessagesAttachmentsImportant(t *testing.T) {
	session := &Session{
		ID:       "s1",
		Messages: []core.Message{{Role: "user", Content: "hi"}},
		Todos:    []Todo{{Description: "keep"}},
		Metadata: map[string]string{"title": "keep"},
		Attachments: map[string]core.Attachment{
			"a": {Name: "a"},
		},
		Important:   map[string]core.ImportantNote{"n": {Content: "note"}},
		UserPersona: &core.UserPersonaProfile{DecisionStyle: "direct"},
	}

	session.ClearContent()

	if session.Messages != nil {
		t.Fatal("expected Messages cleared")
	}
	if session.Attachments != nil {
		t.Fatal("expected Attachments cleared")
	}
	if session.Important != nil {
		t.Fatal("expected Important cleared")
	}
	// These should be preserved:
	if session.Metadata == nil || session.Metadata["title"] != "keep" {
		t.Fatal("expected Metadata preserved")
	}
	if len(session.Todos) != 1 {
		t.Fatal("expected Todos preserved")
	}
	if session.UserPersona == nil {
		t.Fatal("expected UserPersona preserved")
	}
}

// --- GetOrCreate ---

type stubSessionStore struct {
	sessions  map[string]*Session
	createID  string
	createErr error
	saveErr   error
	saveCalls int
}

func (s *stubSessionStore) Create(context.Context) (*Session, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	id := s.createID
	if id == "" {
		id = "auto-created"
	}
	session := NewSession(id, time.Now())
	if s.sessions == nil {
		s.sessions = map[string]*Session{}
	}
	s.sessions[id] = session
	return session, nil
}

func (s *stubSessionStore) Get(_ context.Context, id string) (*Session, error) {
	if s.sessions != nil {
		if session, ok := s.sessions[id]; ok {
			return session, nil
		}
	}
	return nil, ErrSessionNotFound
}

func (s *stubSessionStore) Save(_ context.Context, session *Session) error {
	s.saveCalls++
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.sessions == nil {
		s.sessions = map[string]*Session{}
	}
	s.sessions[session.ID] = session
	return nil
}

func (s *stubSessionStore) List(context.Context, int, int) ([]string, error) { return nil, nil }
func (s *stubSessionStore) Delete(context.Context, string) error              { return nil }

func TestGetOrCreateEmptyIDDelegatesToCreate(t *testing.T) {
	store := &stubSessionStore{createID: "new-id"}
	session, err := GetOrCreate(context.Background(), store, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "new-id" {
		t.Fatalf("expected created session id, got %q", session.ID)
	}
}

func TestGetOrCreateReturnsExisting(t *testing.T) {
	existing := NewSession("exists", time.Now())
	store := &stubSessionStore{sessions: map[string]*Session{"exists": existing}}

	session, err := GetOrCreate(context.Background(), store, "exists", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "exists" {
		t.Fatalf("expected existing session, got %q", session.ID)
	}
	if store.saveCalls != 0 {
		t.Fatalf("expected no save for existing session, got %d", store.saveCalls)
	}
}

func TestGetOrCreateCreatesMissing(t *testing.T) {
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	store := &stubSessionStore{}

	session, err := GetOrCreate(context.Background(), store, "missing-id", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "missing-id" {
		t.Fatalf("expected session id missing-id, got %q", session.ID)
	}
	if session.CreatedAt != now {
		t.Fatalf("expected created_at=%v, got %v", now, session.CreatedAt)
	}
	if store.saveCalls != 1 {
		t.Fatalf("expected one save call, got %d", store.saveCalls)
	}
}

func TestGetOrCreatePropagatesSaveError(t *testing.T) {
	store := &stubSessionStore{saveErr: fmt.Errorf("disk full")}
	_, err := GetOrCreate(context.Background(), store, "fail-save", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetOrCreatePropagatesGetError(t *testing.T) {
	store := &stubSessionStore{}
	// Inject a non-ErrSessionNotFound error by using a custom store.
	badStore := &errorGetStore{err: errors.New("db down")}
	_, err := GetOrCreate(context.Background(), badStore, "some-id", time.Now())
	if err == nil {
		t.Fatal("expected error from Get")
	}
	_ = store // avoid unused
}

type errorGetStore struct {
	err error
}

func (s *errorGetStore) Create(context.Context) (*Session, error)          { return nil, nil }
func (s *errorGetStore) Get(context.Context, string) (*Session, error)     { return nil, s.err }
func (s *errorGetStore) Save(context.Context, *Session) error              { return nil }
func (s *errorGetStore) List(context.Context, int, int) ([]string, error)  { return nil, nil }
func (s *errorGetStore) Delete(context.Context, string) error              { return nil }
