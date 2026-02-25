package app

import (
	"context"
	"errors"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/analytics/journal"
	sessionstate "alex/internal/infra/session/state_store"
)

// ── test doubles ──

type fakeJournalReader struct {
	entries   []journal.TurnJournalEntry
	streamErr error
}

func (r *fakeJournalReader) Stream(_ context.Context, sessionID string, fn func(journal.TurnJournalEntry) error) error {
	if r.streamErr != nil {
		return r.streamErr
	}
	for _, e := range r.entries {
		entry := e
		if entry.SessionID == "" {
			entry.SessionID = sessionID
		}
		if err := fn(entry); err != nil {
			return err
		}
	}
	return nil
}

func (r *fakeJournalReader) ReadAll(_ context.Context, sessionID string) ([]journal.TurnJournalEntry, error) {
	if r.streamErr != nil {
		return nil, r.streamErr
	}
	out := make([]journal.TurnJournalEntry, len(r.entries))
	copy(out, r.entries)
	for i := range out {
		if out[i].SessionID == "" {
			out[i].SessionID = sessionID
		}
	}
	return out, nil
}

type failingStateStore struct {
	initErr  error
	clearErr error
	saveErr  error
}

func (s *failingStateStore) Init(_ context.Context, _ string) error {
	return s.initErr
}

func (s *failingStateStore) ClearSession(_ context.Context, _ string) error {
	return s.clearErr
}

func (s *failingStateStore) SaveSnapshot(_ context.Context, _ sessionstate.Snapshot) error {
	return s.saveErr
}

func (s *failingStateStore) LatestSnapshot(_ context.Context, _ string) (sessionstate.Snapshot, error) {
	return sessionstate.Snapshot{}, errors.New("not implemented")
}

func (s *failingStateStore) GetSnapshot(_ context.Context, _ string, _ int) (sessionstate.Snapshot, error) {
	return sessionstate.Snapshot{}, errors.New("not implemented")
}

func (s *failingStateStore) ListSnapshots(_ context.Context, _ string, _ string, _ int) ([]sessionstate.SnapshotMetadata, string, error) {
	return nil, "", errors.New("not implemented")
}

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
}

// ── ReplaySession ──

func TestReplaySession(t *testing.T) {
	t.Run("success rehydrates snapshots from journal", func(t *testing.T) {
		ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
		reader := &fakeJournalReader{
			entries: []journal.TurnJournalEntry{
				{SessionID: "s1", TurnID: 1, Summary: "hello", Timestamp: ts},
				{SessionID: "s1", TurnID: 2, Summary: "world", Timestamp: ts.Add(time.Minute)},
			},
		}
		store := sessionstate.NewInMemoryStore()

		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(store),
			WithSnapshotJournalReader(reader),
		)

		if err := svc.ReplaySession(context.Background(), "s1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		items, _, err := store.ListSnapshots(context.Background(), "s1", "", 10)
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 snapshots, got %d", len(items))
		}
	})

	t.Run("sets session ID when missing from entry", func(t *testing.T) {
		reader := &fakeJournalReader{
			entries: []journal.TurnJournalEntry{
				{TurnID: 1, Summary: "orphan", Timestamp: time.Now()},
			},
		}
		store := sessionstate.NewInMemoryStore()

		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(store),
			WithSnapshotJournalReader(reader),
		)

		if err := svc.ReplaySession(context.Background(), "fill-me"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		snap, err := store.GetSnapshot(context.Background(), "fill-me", 1)
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if snap.SessionID != "fill-me" {
			t.Fatalf("expected session ID 'fill-me', got %q", snap.SessionID)
		}
	})

	t.Run("sets created_at when timestamp is zero", func(t *testing.T) {
		reader := &fakeJournalReader{
			entries: []journal.TurnJournalEntry{
				{SessionID: "s1", TurnID: 1, Summary: "no-ts"},
			},
		}
		store := sessionstate.NewInMemoryStore()

		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(store),
			WithSnapshotJournalReader(reader),
		)

		before := time.Now().UTC()
		if err := svc.ReplaySession(context.Background(), "s1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		snap, err := store.GetSnapshot(context.Background(), "s1", 1)
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		if snap.CreatedAt.Before(before) {
			t.Fatalf("expected CreatedAt >= %v, got %v", before, snap.CreatedAt)
		}
	})

	t.Run("empty session ID returns validation error", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(sessionstate.NewInMemoryStore()),
			WithSnapshotJournalReader(&fakeJournalReader{}),
		)
		err := svc.ReplaySession(context.Background(), "")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("nil journal reader returns unavailable", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(sessionstate.NewInMemoryStore()),
		)
		err := svc.ReplaySession(context.Background(), "s1")
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})

	t.Run("nil state store returns unavailable", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil,
			WithSnapshotJournalReader(&fakeJournalReader{}),
		)
		err := svc.ReplaySession(context.Background(), "s1")
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})

	t.Run("no journal entries returns not found", func(t *testing.T) {
		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(sessionstate.NewInMemoryStore()),
			WithSnapshotJournalReader(&fakeJournalReader{entries: nil}),
		)
		err := svc.ReplaySession(context.Background(), "s1")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("journal stream error propagates", func(t *testing.T) {
		reader := &fakeJournalReader{streamErr: errors.New("disk failure")}
		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(sessionstate.NewInMemoryStore()),
			WithSnapshotJournalReader(reader),
		)
		err := svc.ReplaySession(context.Background(), "s1")
		if err == nil || !errors.Is(err, reader.streamErr) {
			t.Fatalf("expected wrapped disk failure, got %v", err)
		}
	})

	t.Run("clear store error propagates", func(t *testing.T) {
		clearErr := errors.New("clear failed")
		reader := &fakeJournalReader{
			entries: []journal.TurnJournalEntry{
				{SessionID: "s1", TurnID: 1, Summary: "x", Timestamp: time.Now()},
			},
		}
		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(&failingStateStore{clearErr: clearErr}),
			WithSnapshotJournalReader(reader),
		)
		err := svc.ReplaySession(context.Background(), "s1")
		if err == nil || !errors.Is(err, clearErr) {
			t.Fatalf("expected wrapped clear error, got %v", err)
		}
	})

	t.Run("save snapshot error propagates", func(t *testing.T) {
		saveErr := errors.New("save failed")
		reader := &fakeJournalReader{
			entries: []journal.TurnJournalEntry{
				{SessionID: "s1", TurnID: 1, Summary: "x", Timestamp: time.Now()},
			},
		}
		svc := NewSnapshotService(nil, nil,
			WithSnapshotStateStore(&failingStateStore{saveErr: saveErr}),
			WithSnapshotJournalReader(reader),
		)
		err := svc.ReplaySession(context.Background(), "s1")
		if err == nil || !errors.Is(err, saveErr) {
			t.Fatalf("expected wrapped save error, got %v", err)
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
