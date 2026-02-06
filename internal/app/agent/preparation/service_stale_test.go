package preparation

import (
	"context"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
)

type stubHistoryManager struct {
	replayMessages []ports.Message
	clearedSession string
	clearCalled    bool
	replayCalled   bool
}

func (s *stubHistoryManager) AppendTurn(context.Context, string, []ports.Message) error { return nil }
func (s *stubHistoryManager) Replay(context.Context, string, int) ([]ports.Message, error) {
	s.replayCalled = true
	return s.replayMessages, nil
}
func (s *stubHistoryManager) ClearSession(_ context.Context, sessionID string) error {
	s.clearCalled = true
	s.clearedSession = sessionID
	return nil
}

func TestLoadSessionHistoryClearsStaleSession(t *testing.T) {
	now := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	session := &storage.Session{
		ID:        "sess-stale",
		Messages:  []ports.Message{{Role: "user", Content: "old"}},
		Metadata:  map[string]string{"title": "Old"},
		CreatedAt: now.Add(-72 * time.Hour),
		UpdatedAt: now.Add(-49 * time.Hour),
	}
	store := &stubSessionStore{session: session}
	history := &stubHistoryManager{replayMessages: []ports.Message{{Role: "assistant", Content: "history"}}}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		SessionStore: store,
		HistoryMgr:   history,
		Config:       appconfig.Config{SessionStaleAfter: 48 * time.Hour},
		Logger:       agent.NoopLogger{},
		Clock:        agent.ClockFunc(func() time.Time { return now }),
	})

	historyMessages := service.loadSessionHistory(context.Background(), session)
	if len(historyMessages) != 0 {
		t.Fatalf("expected stale session to return empty history, got %d", len(historyMessages))
	}
	if !history.clearCalled || history.clearedSession != session.ID {
		t.Fatalf("expected history to be cleared for %q", session.ID)
	}
	if store.session == nil || len(store.session.Messages) != 0 {
		t.Fatalf("expected session messages to be cleared")
	}
	if store.session.Metadata != nil {
		t.Fatalf("expected metadata cleared")
	}
}

func TestLoadSessionHistoryKeepsFreshSession(t *testing.T) {
	now := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	session := &storage.Session{
		ID:        "sess-fresh",
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		Metadata:  map[string]string{},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
	}
	store := &stubSessionStore{session: session}
	history := &stubHistoryManager{replayMessages: []ports.Message{{Role: "assistant", Content: "history"}}}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		SessionStore: store,
		HistoryMgr:   history,
		Config:       appconfig.Config{SessionStaleAfter: 48 * time.Hour},
		Logger:       agent.NoopLogger{},
		Clock:        agent.ClockFunc(func() time.Time { return now }),
	})

	historyMessages := service.loadSessionHistory(context.Background(), session)
	if len(historyMessages) != 1 {
		t.Fatalf("expected history to be returned, got %d", len(historyMessages))
	}
	if history.clearCalled {
		t.Fatalf("did not expect history clear for fresh session")
	}
}

func TestLoadSessionHistorySkipsWhenDisabled(t *testing.T) {
	now := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	session := &storage.Session{
		ID:        "sess-no-history",
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		Metadata:  map[string]string{},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
	}
	store := &stubSessionStore{session: session}
	history := &stubHistoryManager{replayMessages: []ports.Message{{Role: "assistant", Content: "history"}}}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		SessionStore: store,
		HistoryMgr:   history,
		Config:       appconfig.Config{SessionStaleAfter: 48 * time.Hour},
		Logger:       agent.NoopLogger{},
		Clock:        agent.ClockFunc(func() time.Time { return now }),
	})

	ctx := appcontext.WithSessionHistory(context.Background(), false)
	historyMessages := service.loadSessionHistory(ctx, session)
	if len(historyMessages) != 0 {
		t.Fatalf("expected no history when disabled, got %d", len(historyMessages))
	}
	if history.replayCalled {
		t.Fatal("expected history replay to be skipped when disabled")
	}
}
