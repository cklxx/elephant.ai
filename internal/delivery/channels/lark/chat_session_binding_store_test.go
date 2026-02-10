package lark

import (
	"context"
	"testing"

	"alex/internal/shared/testutil"
)

func TestChatSessionBindingPostgresStore_SaveGetDelete(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := NewChatSessionBindingPostgresStore(pool)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	binding := ChatSessionBinding{
		Channel:   "lark",
		ChatID:    "oc_chat_1",
		SessionID: "lark-session-1",
	}
	if err := store.SaveBinding(ctx, binding); err != nil {
		t.Fatalf("save binding: %v", err)
	}

	got, ok, err := store.GetBinding(ctx, "lark", "oc_chat_1")
	if err != nil {
		t.Fatalf("get binding: %v", err)
	}
	if !ok {
		t.Fatal("expected binding to exist")
	}
	if got.SessionID != "lark-session-1" {
		t.Fatalf("unexpected session id: %q", got.SessionID)
	}

	// Upsert update.
	if err := store.SaveBinding(ctx, ChatSessionBinding{
		Channel:   "lark",
		ChatID:    "oc_chat_1",
		SessionID: "lark-session-2",
	}); err != nil {
		t.Fatalf("update binding: %v", err)
	}

	got, ok, err = store.GetBinding(ctx, "lark", "oc_chat_1")
	if err != nil {
		t.Fatalf("get updated binding: %v", err)
	}
	if !ok {
		t.Fatal("expected updated binding to exist")
	}
	if got.SessionID != "lark-session-2" {
		t.Fatalf("expected updated session id, got %q", got.SessionID)
	}

	if err := store.DeleteBinding(ctx, "lark", "oc_chat_1"); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	_, ok, err = store.GetBinding(ctx, "lark", "oc_chat_1")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if ok {
		t.Fatal("expected binding deleted")
	}
}
