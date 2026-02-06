package preparation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

// historySummaryResponse provides a shared stubbed summary string so history-aware
// execution-preparation tests don't redeclare their own copies (which previously
// triggered golangci-lint duplicate symbol errors when merged).
func historySummaryResponse() string {
	return "History Summary: Marketing experiments and automation context recalled."
}

type stubSessionStore struct {
	mu      sync.Mutex
	session *storage.Session
}

func (s *stubSessionStore) Create(ctx context.Context) (*storage.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &storage.Session{
		ID:        "session-stub",
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.session = sess
	return sess, nil
}

func (s *stubSessionStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return s.Create(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil && s.session.ID == id {
		return s.cloneSession(s.session), nil
	}
	sess := &storage.Session{
		ID:        id,
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.session = sess
	return s.cloneSession(sess), nil
}

func (s *stubSessionStore) cloneSession(sess *storage.Session) *storage.Session {
	meta := make(map[string]string, len(sess.Metadata))
	for k, v := range sess.Metadata {
		meta[k] = v
	}
	return &storage.Session{
		ID:        sess.ID,
		Messages:  append([]ports.Message(nil), sess.Messages...),
		Metadata:  meta,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
	}
}

func (s *stubSessionStore) Save(ctx context.Context, session *storage.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = session
	return nil
}

func (s *stubSessionStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return []string{}, nil
	}
	return []string{s.session.ID}, nil
}

func (s *stubSessionStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil && s.session.ID == id {
		s.session = nil
	}
	return nil
}

// SessionTitle returns the current session title safely.
func (s *stubSessionStore) SessionTitle() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil || s.session.Metadata == nil {
		return ""
	}
	return s.session.Metadata["title"]
}

type stubContextManager struct{}

func (stubContextManager) EstimateTokens(messages []ports.Message) int { return len(messages) * 10 }
func (stubContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}
func (stubContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	return messages, false
}
func (stubContextManager) ShouldCompress(messages []ports.Message, limit int) bool { return false }
func (stubContextManager) Preload(context.Context) error                           { return nil }
func (stubContextManager) BuildWindow(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}
func (stubContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error { return nil }

type stubParser struct{}

func (stubParser) Parse(content string) ([]ports.ToolCall, error)                      { return nil, nil }
func (stubParser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error { return nil }

type testLogger struct {
	messages []string
}

func (l *testLogger) Debug(format string, args ...interface{}) {
	l.messages = append(l.messages, "DEBUG")
}

func (l *testLogger) Info(format string, args ...interface{}) {
	l.messages = append(l.messages, "INFO")
}

func (l *testLogger) Warn(format string, args ...interface{}) {
	l.messages = append(l.messages, "WARN")
}

func (l *testLogger) Error(format string, args ...interface{}) {
	l.messages = append(l.messages, "ERROR")
}

type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time { return c.now }

func newMockClock(t time.Time) *mockClock {
	return &mockClock{now: t}
}
