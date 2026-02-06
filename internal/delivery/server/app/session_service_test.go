package app

import (
	"context"
	"errors"
	"testing"
	"time"

	core "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	sessionstate "alex/internal/session/state_store"
)

// --- Test doubles ---

// strictSessionStore returns ErrSessionNotFound for unknown IDs (unlike MockSessionStore).
type strictSessionStore struct {
	sessions map[string]*storage.Session
	nextID   int
	saveErr  error
}

func newStrictSessionStore() *strictSessionStore {
	return &strictSessionStore{sessions: make(map[string]*storage.Session)}
}

func (s *strictSessionStore) Create(ctx context.Context) (*storage.Session, error) {
	s.nextID++
	sess := &storage.Session{
		ID:        "sess-" + time.Now().Format("150405") + "-" + string(rune('0'+s.nextID)),
		Messages:  []core.Message{},
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[sess.ID] = sess
	return sess, nil
}

func (s *strictSessionStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return nil, storage.ErrSessionNotFound
}

func (s *strictSessionStore) Save(ctx context.Context, session *storage.Session) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.sessions[session.ID] = session
	return nil
}

func (s *strictSessionStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	if offset >= len(ids) {
		return nil, nil
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}
	return ids[offset:end], nil
}

func (s *strictSessionStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.sessions[id]; !ok {
		return storage.ErrSessionNotFound
	}
	delete(s.sessions, id)
	return nil
}

// Seed populates a session for testing.
func (s *strictSessionStore) Seed(id string, meta map[string]string) *storage.Session {
	sess := &storage.Session{
		ID:        id,
		Messages:  []core.Message{},
		Metadata:  meta,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now(),
	}
	if sess.Metadata == nil {
		sess.Metadata = make(map[string]string)
	}
	s.sessions[id] = sess
	return sess
}

type stubAgentExecutor struct {
	sessionStore storage.SessionStore
}

func (s *stubAgentExecutor) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return s.sessionStore.Create(ctx)
	}
	return s.sessionStore.Get(ctx, id)
}

func (s *stubAgentExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{Answer: "ok"}, nil
}

func (s *stubAgentExecutor) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (s *stubAgentExecutor) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{}, nil
}

type spyStateStore struct {
	sessionstate.Store
	initCalls    []string
	clearCalls   []string
	initErr      error
}

func newSpyStateStore() *spyStateStore {
	return &spyStateStore{Store: sessionstate.NewInMemoryStore()}
}

func (s *spyStateStore) Init(ctx context.Context, sessionID string) error {
	s.initCalls = append(s.initCalls, sessionID)
	if s.initErr != nil {
		return s.initErr
	}
	return s.Store.Init(ctx, sessionID)
}

func (s *spyStateStore) ClearSession(ctx context.Context, sessionID string) error {
	s.clearCalls = append(s.clearCalls, sessionID)
	return s.Store.ClearSession(ctx, sessionID)
}

// --- Tests ---

func TestGetSession_Found(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", nil)
	svc := NewSessionService(nil, store, nil)

	sess, err := svc.GetSession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID != "s1" {
		t.Fatalf("expected s1, got %s", sess.ID)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.GetSession(context.Background(), "nonexistent")
	if !errors.Is(err, storage.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestCreateSession_Success(t *testing.T) {
	store := newStrictSessionStore()
	state := newSpyStateStore()
	executor := &stubAgentExecutor{sessionStore: store}
	svc := NewSessionService(executor, store, nil, WithSessionStateStore(state))

	sess, err := svc.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if len(state.initCalls) != 1 || state.initCalls[0] != sess.ID {
		t.Fatalf("expected state store Init to be called with %s, got %v", sess.ID, state.initCalls)
	}
}

func TestCreateSession_NilCoordinator(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.CreateSession(context.Background())
	if err == nil {
		t.Fatal("expected error for nil coordinator")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestCreateSession_StateStoreInitFailureNonFatal(t *testing.T) {
	store := newStrictSessionStore()
	state := newSpyStateStore()
	state.initErr = errors.New("init boom")
	executor := &stubAgentExecutor{sessionStore: store}
	svc := NewSessionService(executor, store, nil, WithSessionStateStore(state))

	sess, err := svc.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("state store init should not cause failure, got: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected session to be created despite state store failure")
	}
}

func TestDeleteSession_Success(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", nil)
	state := newSpyStateStore()
	_ = state.Init(context.Background(), "s1")
	history := newSpyStateStore()
	broadcaster := NewEventBroadcaster()

	svc := NewSessionService(nil, store, broadcaster,
		WithSessionStateStore(state),
		WithSessionHistoryStore(history),
	)

	err := svc.DeleteSession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify session removed from store
	_, getErr := store.Get(context.Background(), "s1")
	if !errors.Is(getErr, storage.ErrSessionNotFound) {
		t.Fatalf("session should be deleted, got %v", getErr)
	}

	// Verify state and history stores were cleared
	if len(state.clearCalls) != 1 || state.clearCalls[0] != "s1" {
		t.Fatalf("expected state store clear for s1, got %v", state.clearCalls)
	}
	if len(history.clearCalls) != 1 || history.clearCalls[0] != "s1" {
		t.Fatalf("expected history store clear for s1, got %v", history.clearCalls)
	}
}

func TestDeleteSession_PartialFailureJoinsErrors(t *testing.T) {
	store := newStrictSessionStore()
	// Don't seed — delete will fail with ErrSessionNotFound
	state := newSpyStateStore()
	svc := NewSessionService(nil, store, nil,
		WithSessionStateStore(state),
	)

	err := svc.DeleteSession(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error from failed delete")
	}
	// Should still have attempted state store clear
	if len(state.clearCalls) != 1 {
		t.Fatalf("expected state store clear attempt, got %d calls", len(state.clearCalls))
	}
}

func TestUpdateSessionPersona_Success(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", nil)
	svc := NewSessionService(nil, store, nil)

	persona := &core.UserPersonaProfile{Version: "v1", Summary: "Alice"}
	sess, err := svc.UpdateSessionPersona(context.Background(), "s1", persona)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.UserPersona == nil || sess.UserPersona.Summary != "Alice" {
		t.Fatalf("expected persona Summary=Alice, got %v", sess.UserPersona)
	}
}

func TestUpdateSessionPersona_NilPersona(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", nil)
	svc := NewSessionService(nil, store, nil)

	_, err := svc.UpdateSessionPersona(context.Background(), "s1", nil)
	if err == nil {
		t.Fatal("expected error for nil persona")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestUpdateSessionPersona_SessionNotFound(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.UpdateSessionPersona(context.Background(), "gone", &core.UserPersonaProfile{Version: "v1"})
	if !errors.Is(err, storage.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestForkSession_Success(t *testing.T) {
	store := newStrictSessionStore()
	original := store.Seed("s-original", map[string]string{"title": "Hello"})
	original.Messages = []core.Message{{Role: "user", Content: "hi"}}

	svc := NewSessionService(nil, store, nil)

	forked, err := svc.ForkSession(context.Background(), "s-original")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forked.ID == "s-original" {
		t.Fatal("forked session should have a different ID")
	}
	if len(forked.Messages) != 1 || forked.Messages[0].Content != "hi" {
		t.Fatalf("expected messages to be copied, got %v", forked.Messages)
	}
	if forked.Metadata["forked_from"] != "s-original" {
		t.Fatalf("expected forked_from metadata, got %v", forked.Metadata)
	}
	if forked.Metadata["title"] != "Hello" {
		t.Fatalf("expected title metadata to be copied, got %v", forked.Metadata)
	}
}

func TestForkSession_OriginalNotFound(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.ForkSession(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing original session")
	}
	if !errors.Is(err, storage.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound in chain, got %v", err)
	}
}

func TestForkSession_SaveFailure(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s-original", nil)
	store.saveErr = errors.New("disk full")

	svc := NewSessionService(nil, store, nil)

	_, err := svc.ForkSession(context.Background(), "s-original")
	if err == nil {
		t.Fatal("expected error from save failure")
	}
}

func TestListSessions(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", nil)
	store.Seed("s2", nil)
	store.Seed("s3", nil)

	svc := NewSessionService(nil, store, nil)

	ids, err := svc.ListSessions(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(ids))
	}
}

func TestEnsureSessionShareToken_NewToken(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", nil)
	svc := NewSessionService(nil, store, nil)

	token, err := svc.EnsureSessionShareToken(context.Background(), "s1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if token[:6] != "share-" {
		t.Fatalf("expected share- prefix, got %s", token)
	}

	// Verify token persisted
	sess, _ := store.Get(context.Background(), "s1")
	if sess.Metadata["share_token"] != token {
		t.Fatalf("expected token in metadata, got %s", sess.Metadata["share_token"])
	}
	if sess.Metadata["share_enabled"] != "true" {
		t.Fatalf("expected share_enabled=true, got %s", sess.Metadata["share_enabled"])
	}
}

func TestEnsureSessionShareToken_ExistingToken(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", map[string]string{"share_token": "share-existing", "share_enabled": "true"})
	svc := NewSessionService(nil, store, nil)

	token, err := svc.EnsureSessionShareToken(context.Background(), "s1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "share-existing" {
		t.Fatalf("expected existing token, got %s", token)
	}
}

func TestEnsureSessionShareToken_Reset(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", map[string]string{"share_token": "share-old", "share_enabled": "true"})
	svc := NewSessionService(nil, store, nil)

	token, err := svc.EnsureSessionShareToken(context.Background(), "s1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "share-old" {
		t.Fatal("expected new token on reset")
	}
	if token[:6] != "share-" {
		t.Fatalf("expected share- prefix, got %s", token)
	}
}

func TestEnsureSessionShareToken_EmptySessionID(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.EnsureSessionShareToken(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected error for empty session ID")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestEnsureSessionShareToken_SessionNotFound(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.EnsureSessionShareToken(context.Background(), "missing", false)
	if !errors.Is(err, storage.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestValidateShareToken_Valid(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", map[string]string{"share_token": "share-abc123", "share_enabled": "true"})
	svc := NewSessionService(nil, store, nil)

	sess, err := svc.ValidateShareToken(context.Background(), "s1", "share-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID != "s1" {
		t.Fatalf("expected s1, got %s", sess.ID)
	}
}

func TestValidateShareToken_Invalid(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", map[string]string{"share_token": "share-abc123"})
	svc := NewSessionService(nil, store, nil)

	_, err := svc.ValidateShareToken(context.Background(), "s1", "share-wrong")
	if !errors.Is(err, ErrShareTokenInvalid) {
		t.Fatalf("expected ErrShareTokenInvalid, got %v", err)
	}
}

func TestValidateShareToken_EmptyToken(t *testing.T) {
	store := newStrictSessionStore()
	store.Seed("s1", map[string]string{"share_token": "share-abc123"})
	svc := NewSessionService(nil, store, nil)

	_, err := svc.ValidateShareToken(context.Background(), "s1", "")
	if !errors.Is(err, ErrShareTokenInvalid) {
		t.Fatalf("expected ErrShareTokenInvalid, got %v", err)
	}
}

func TestValidateShareToken_EmptySessionID(t *testing.T) {
	store := newStrictSessionStore()
	svc := NewSessionService(nil, store, nil)

	_, err := svc.ValidateShareToken(context.Background(), "", "share-token")
	if err == nil {
		t.Fatal("expected error for empty session ID")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestValidateShareToken_NoMetadata(t *testing.T) {
	store := newStrictSessionStore()
	sess := store.Seed("s1", nil)
	sess.Metadata = nil // explicitly nil metadata
	svc := NewSessionService(nil, store, nil)

	_, err := svc.ValidateShareToken(context.Background(), "s1", "share-any")
	if !errors.Is(err, ErrShareTokenInvalid) {
		t.Fatalf("expected ErrShareTokenInvalid, got %v", err)
	}
}
