package lark

import (
	"context"
	"testing"
	"time"

	"alex/internal/logging"
	"alex/internal/memory"
)

func TestSaveMessage_UserMessage(t *testing.T) {
	store := &stubMemoryStore{}
	svc := memory.NewService(store)
	mgr := newLarkMemoryManager(svc, logging.OrNop(nil))

	mgr.SaveMessage(context.Background(), "mem-123", "user", "hello world", "ou_user1")

	if len(store.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(store.entries))
	}
	entry := store.entries[0]
	if entry.UserID != "mem-123" {
		t.Fatalf("expected user_id 'mem-123', got %q", entry.UserID)
	}
	if entry.Content != "hello world" {
		t.Fatalf("expected content 'hello world', got %q", entry.Content)
	}
	if entry.Slots["type"] != "chat_message" {
		t.Fatalf("expected slot type 'chat_message', got %q", entry.Slots["type"])
	}
	if entry.Slots["role"] != "user" {
		t.Fatalf("expected slot role 'user', got %q", entry.Slots["role"])
	}
	if entry.Slots["sender_id"] != "ou_user1" {
		t.Fatalf("expected slot sender_id 'ou_user1', got %q", entry.Slots["sender_id"])
	}
}

func TestSaveMessage_AssistantMessage(t *testing.T) {
	store := &stubMemoryStore{}
	svc := memory.NewService(store)
	mgr := newLarkMemoryManager(svc, logging.OrNop(nil))

	mgr.SaveMessage(context.Background(), "mem-456", "assistant", "I can help with that", "bot")

	if len(store.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(store.entries))
	}
	entry := store.entries[0]
	if entry.Slots["role"] != "assistant" {
		t.Fatalf("expected slot role 'assistant', got %q", entry.Slots["role"])
	}
	if entry.Slots["sender_id"] != "bot" {
		t.Fatalf("expected slot sender_id 'bot', got %q", entry.Slots["sender_id"])
	}
}

func TestSaveMessage_EmptyContent(t *testing.T) {
	store := &stubMemoryStore{}
	svc := memory.NewService(store)
	mgr := newLarkMemoryManager(svc, logging.OrNop(nil))

	mgr.SaveMessage(context.Background(), "mem-789", "user", "", "ou_user1")
	mgr.SaveMessage(context.Background(), "mem-789", "user", "   ", "ou_user1")

	if len(store.entries) != 0 {
		t.Fatalf("expected 0 entries for empty content, got %d", len(store.entries))
	}
}

func TestSaveMessage_NilManager(t *testing.T) {
	var mgr *larkMemoryManager
	// Should not panic.
	mgr.SaveMessage(context.Background(), "mem-000", "user", "hello", "ou_user1")
}

func TestSaveMessage_Keywords(t *testing.T) {
	store := &stubMemoryStore{}
	svc := memory.NewService(store)
	mgr := newLarkMemoryManager(svc, logging.OrNop(nil))

	mgr.SaveMessage(context.Background(), "mem-kw", "user", "deploy production server", "ou_user1")

	if len(store.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(store.entries))
	}
	entry := store.entries[0]
	if len(entry.Keywords) == 0 {
		t.Fatal("expected non-empty keywords for message with content")
	}
}

// stubMemoryStore implements memory.Store for testing.
type stubMemoryStore struct {
	entries []memory.Entry
}

func (s *stubMemoryStore) EnsureSchema(_ context.Context) error { return nil }

func (s *stubMemoryStore) Insert(_ context.Context, entry memory.Entry) (memory.Entry, error) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.entries = append(s.entries, entry)
	return entry, nil
}

func (s *stubMemoryStore) Search(_ context.Context, _ memory.Query) ([]memory.Entry, error) {
	return s.entries, nil
}
