package app

import (
	"context"
	"errors"
	"testing"
	"time"

	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	sessionstate "alex/internal/infra/session/state_store"
)

// ── test doubles ──

type stubAgentExec struct {
	preview agent.ContextWindowPreview
	err     error
}

func (s *stubAgentExec) GetSession(context.Context, string) (*storage.Session, error) {
	return nil, errors.New("not implemented")
}

func (s *stubAgentExec) ExecuteTask(context.Context, string, string, agent.EventListener) (*agent.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubAgentExec) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (s *stubAgentExec) PreviewContextWindow(_ context.Context, _ string) (agent.ContextWindowPreview, error) {
	return s.preview, s.err
}

// ── ListSnapshots ──

func TestListSnapshots(t *testing.T) {
	t.Run("returns items from state store", func(t *testing.T) {
		store := sessionstate.NewInMemoryStore()
		_ = store.Init(context.Background(), "s1")
		_ = store.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 1, Summary: "first",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		})
		_ = store.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 2, Summary: "second",
			CreatedAt: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		})

		svc := NewSnapshotService(nil, nil, WithSnapshotStateStore(store))
		items, _, err := svc.ListSnapshots(context.Background(), "s1", "", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("nil state store returns unavailable", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil)
		_, _, err := svc.ListSnapshots(context.Background(), "s1", "", 10)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})
}

// ── GetSnapshot ──

func TestGetSnapshot(t *testing.T) {
	t.Run("returns snapshot from state store", func(t *testing.T) {
		store := sessionstate.NewInMemoryStore()
		_ = store.Init(context.Background(), "s1")
		_ = store.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 3, Summary: "turn three",
			CreatedAt: time.Now().UTC(),
		})

		svc := NewSnapshotService(nil, nil, WithSnapshotStateStore(store))
		snap, err := svc.GetSnapshot(context.Background(), "s1", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if snap.Summary != "turn three" {
			t.Fatalf("expected summary 'turn three', got %q", snap.Summary)
		}
	})

	t.Run("nil state store returns unavailable", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil)
		_, err := svc.GetSnapshot(context.Background(), "s1", 1)
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})

	t.Run("merges messages from history store when snapshot has none", func(t *testing.T) {
		stateStore := sessionstate.NewInMemoryStore()
		historyStore := sessionstate.NewInMemoryStore()

		_ = stateStore.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 5, Summary: "structural only",
			CreatedAt: time.Now().UTC(),
		})
		_ = historyStore.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 1,
			Messages:  []ports.Message{{Role: "user", Content: "hello"}},
			CreatedAt: time.Now().UTC(),
		})
		_ = historyStore.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 2,
			Messages:  []ports.Message{{Role: "assistant", Content: "hi"}},
			CreatedAt: time.Now().UTC(),
		})

		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(stateStore),
			WithSnapshotHistoryStore(historyStore),
		)
		snap, err := svc.GetSnapshot(context.Background(), "s1", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(snap.Messages) != 2 {
			t.Fatalf("expected 2 messages from history, got %d", len(snap.Messages))
		}
		if snap.Messages[0].Content != "hello" || snap.Messages[1].Content != "hi" {
			t.Fatalf("unexpected messages: %+v", snap.Messages)
		}
	})

	t.Run("does not merge when snapshot already has messages", func(t *testing.T) {
		stateStore := sessionstate.NewInMemoryStore()
		historyStore := sessionstate.NewInMemoryStore()

		_ = stateStore.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 1,
			Messages:  []ports.Message{{Role: "system", Content: "existing"}},
			CreatedAt: time.Now().UTC(),
		})
		_ = historyStore.SaveSnapshot(context.Background(), sessionstate.Snapshot{
			SessionID: "s1", TurnID: 1,
			Messages:  []ports.Message{{Role: "user", Content: "from history"}},
			CreatedAt: time.Now().UTC(),
		})

		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(stateStore),
			WithSnapshotHistoryStore(historyStore),
		)
		snap, err := svc.GetSnapshot(context.Background(), "s1", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(snap.Messages) != 1 || snap.Messages[0].Content != "existing" {
			t.Fatalf("expected original messages preserved, got %+v", snap.Messages)
		}
	})
}

// ── PreviewContextWindow ──

func TestPreviewContextWindow(t *testing.T) {
	t.Run("delegates to agent coordinator", func(t *testing.T) {
		expected := agent.ContextWindowPreview{
			TokenEstimate: 500,
			TokenLimit:    4096,
			ToolMode:      "web",
		}
		exec := &stubAgentExec{preview: expected}
		svc := NewSnapshotService(exec, nil)

		result, err := svc.PreviewContextWindow(context.Background(), "s1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TokenEstimate != 500 || result.TokenLimit != 4096 {
			t.Fatalf("unexpected preview: %+v", result)
		}
	})

	t.Run("nil agent coordinator returns unavailable", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil)
		_, err := svc.PreviewContextWindow(context.Background(), "s1")
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})

	t.Run("propagates coordinator error", func(t *testing.T) {
		coordErr := errors.New("coordinator down")
		exec := &stubAgentExec{err: coordErr}
		svc := NewSnapshotService(exec, nil)

		_, err := svc.PreviewContextWindow(context.Background(), "s1")
		if !errors.Is(err, coordErr) {
			t.Fatalf("expected coordinator error, got %v", err)
		}
	})
}

// ── GetContextSnapshots ──

func TestGetContextSnapshots(t *testing.T) {
	t.Run("nil broadcaster returns nil", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil)
		result := svc.GetContextSnapshots("s1")
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("empty session ID returns nil", func(t *testing.T) {
		svc := NewSnapshotService(nil, NewEventBroadcaster())
		result := svc.GetContextSnapshots("")
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("returns empty for session with no events", func(t *testing.T) {
		svc := NewSnapshotService(nil, NewEventBroadcaster())
		result := svc.GetContextSnapshots("unknown")
		if result != nil {
			t.Fatalf("expected nil for unknown session, got %v", result)
		}
	})
}
