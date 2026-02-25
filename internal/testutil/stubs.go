// Package testutil provides reusable test stubs shared across agent test files.
// These are extracted from coordinator and preparation test packages to avoid duplication.
package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/shared/utils"
)

// StubSessionStore is a thread-safe in-memory session store for tests.
type StubSessionStore struct {
	mu      sync.Mutex
	Session *storage.Session
}

func (s *StubSessionStore) Create(ctx context.Context) (*storage.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &storage.Session{
		ID:        "session-stub",
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.Session = sess
	return sess, nil
}

func (s *StubSessionStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return s.Create(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Session != nil && s.Session.ID == id {
		return s.cloneSession(s.Session), nil
	}
	sess := &storage.Session{
		ID:        id,
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.Session = sess
	return s.cloneSession(sess), nil
}

func (s *StubSessionStore) cloneSession(sess *storage.Session) *storage.Session {
	return &storage.Session{
		ID:        sess.ID,
		Messages:  utils.CloneSlice(sess.Messages),
		Metadata:  utils.CloneMap(sess.Metadata),
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
	}
}

func (s *StubSessionStore) Save(ctx context.Context, session *storage.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Session = session
	return nil
}

func (s *StubSessionStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Session == nil {
		return []string{}, nil
	}
	return []string{s.Session.ID}, nil
}

func (s *StubSessionStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Session != nil && s.Session.ID == id {
		s.Session = nil
	}
	return nil
}

// SessionTitle returns the current session title safely.
func (s *StubSessionStore) SessionTitle() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Session == nil || s.Session.Metadata == nil {
		return ""
	}
	return s.Session.Metadata["title"]
}

// StubContextManager is a no-op context manager for tests.
type StubContextManager struct{}

func (StubContextManager) EstimateTokens(messages []ports.Message) int { return len(messages) * 10 }
func (StubContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}
func (StubContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	return messages, false
}
func (StubContextManager) ShouldCompress(messages []ports.Message, limit int) bool { return false }
func (StubContextManager) Preload(context.Context) error                           { return nil }
func (StubContextManager) BuildWindow(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}
func (StubContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error { return nil }

// StubToolRegistry is a no-op tool registry for tests.
type StubToolRegistry struct{}

func (StubToolRegistry) Register(tool tools.ToolExecutor) error { return nil }
func (StubToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	return nil, fmt.Errorf("tool %s not found", name)
}
func (StubToolRegistry) List() []ports.ToolDefinition { return nil }
func (StubToolRegistry) Unregister(name string) error { return nil }

// StubParser is a no-op tool call parser for tests.
type StubParser struct{}

func (StubParser) Parse(content string) ([]ports.ToolCall, error)                      { return nil, nil }
func (StubParser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error { return nil }

// RecordingEventListener collects events in a thread-safe manner for test assertions.
type RecordingEventListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (l *RecordingEventListener) OnEvent(event agent.AgentEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

// Events returns a snapshot of all collected events.
func (l *RecordingEventListener) Events() []agent.AgentEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return utils.CloneSlice(l.events)
}

// StubHistoryManager is a no-op history manager for tests.
type StubHistoryManager struct {
	mu               sync.Mutex
	ClearedSessionID string
	AppendCalled     bool
}

func (s *StubHistoryManager) AppendTurn(context.Context, string, []ports.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AppendCalled = true
	return nil
}

func (s *StubHistoryManager) Replay(context.Context, string, int) ([]ports.Message, error) {
	return nil, nil
}

func (s *StubHistoryManager) ClearSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ClearedSessionID = sessionID
	return nil
}
