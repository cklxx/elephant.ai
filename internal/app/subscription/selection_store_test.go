package subscription

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionStoreSetGetClearCLI(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "llm_selection.json")

	store := NewSelectionStore(path)
	scope := SelectionScope{Channel: "cli"}
	want := Selection{Mode: "cli", Provider: "anthropic", Model: "claude-sonnet-4", Source: "anthropic"}

	if err := store.Set(context.Background(), scope, want); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	got, ok, err := store.Get(context.Background(), scope)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected selection to exist")
	}
	if got.Provider != want.Provider || got.Model != want.Model || got.Mode != want.Mode {
		t.Fatalf("unexpected selection: %#v", got)
	}

	if err := store.Clear(context.Background(), scope); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected store file to be removed after clear")
	}
}

func TestSelectionStoreSetGetLarkScope(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "llm_selection.json")

	store := NewSelectionStore(path)
	scope := SelectionScope{Channel: "lark", ChatID: "oc_chat", UserID: "ou_user"}
	want := Selection{Mode: "cli", Provider: "anthropic", Model: "claude-sonnet-4"}

	if err := store.Set(context.Background(), scope, want); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	got, ok, err := store.Get(context.Background(), scope)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected selection to exist")
	}
	if got.Provider != want.Provider || got.Model != want.Model || got.Mode != want.Mode {
		t.Fatalf("unexpected selection: %#v", got)
	}
}

func TestSelectionStoreRejectsInvalidScope(t *testing.T) {
	t.Parallel()
	store := NewSelectionStore(filepath.Join(t.TempDir(), "llm_selection.json"))

	if err := store.Set(context.Background(), SelectionScope{}, Selection{Mode: "cli"}); err == nil {
		t.Fatalf("expected error for missing channel")
	}
	if err := store.Set(context.Background(), SelectionScope{Channel: "lark", ChatID: "c"}, Selection{Mode: "cli"}); err == nil {
		t.Fatalf("expected error for missing user_id")
	}
	if err := store.Set(context.Background(), SelectionScope{Channel: "lark", UserID: "u"}, Selection{Mode: "cli"}); err == nil {
		t.Fatalf("expected error for missing chat_id")
	}
}

func TestSelectionStoreRejectsUnsupportedVersion(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "llm_selection.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"selections":{"cli":{"mode":"cli","provider":"anthropic","model":"claude-sonnet-4"}}}`), 0o600); err != nil {
		t.Fatalf("write store: %v", err)
	}

	store := NewSelectionStore(path)
	if _, _, err := store.Get(context.Background(), SelectionScope{Channel: "cli"}); err == nil {
		t.Fatalf("expected version error")
	}
}

func TestSelectionStoreDefaultsMissingVersion(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "llm_selection.json")
	if err := os.WriteFile(path, []byte(`{"selections":{"cli":{"mode":"cli","provider":"anthropic","model":"claude-sonnet-4"}}}`), 0o600); err != nil {
		t.Fatalf("write store: %v", err)
	}

	store := NewSelectionStore(path)
	selection, ok, err := store.Get(context.Background(), SelectionScope{Channel: "cli"})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected selection to exist")
	}
	if selection.Provider != "anthropic" || selection.Model != "claude-sonnet-4" {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestGetWithFallback_ChatSpecificFirst(t *testing.T) {
	t.Parallel()
	store := NewSelectionStore(filepath.Join(t.TempDir(), "llm_selection.json"))
	ctx := context.Background()

	channel := SelectionScope{Channel: "lark"}
	chat := SelectionScope{Channel: "lark", ChatID: "oc_chat", UserID: "ou_user"}

	channelSel := Selection{Mode: "cli", Provider: "codex", Model: "gpt-5.2-codex"}
	chatSel := Selection{Mode: "cli", Provider: "anthropic", Model: "claude-sonnet-4"}

	if err := store.Set(ctx, channel, channelSel); err != nil {
		t.Fatalf("Set channel: %v", err)
	}
	if err := store.Set(ctx, chat, chatSel); err != nil {
		t.Fatalf("Set chat: %v", err)
	}

	got, matchedScope, ok, err := store.GetWithFallback(ctx, chat, channel)
	if err != nil {
		t.Fatalf("GetWithFallback: %v", err)
	}
	if !ok {
		t.Fatal("expected a match")
	}
	if got.Provider != "anthropic" || got.Model != "claude-sonnet-4" {
		t.Fatalf("expected chat-specific selection, got %#v", got)
	}
	if matchedScope.ChatID != "oc_chat" {
		t.Fatalf("expected chat-specific scope, got %#v", matchedScope)
	}
}

func TestGetWithFallback_ChannelFallback(t *testing.T) {
	t.Parallel()
	store := NewSelectionStore(filepath.Join(t.TempDir(), "llm_selection.json"))
	ctx := context.Background()

	channel := SelectionScope{Channel: "lark"}
	chat := SelectionScope{Channel: "lark", ChatID: "oc_chat", UserID: "ou_user"}

	channelSel := Selection{Mode: "cli", Provider: "codex", Model: "gpt-5.2-codex"}
	if err := store.Set(ctx, channel, channelSel); err != nil {
		t.Fatalf("Set channel: %v", err)
	}

	got, matchedScope, ok, err := store.GetWithFallback(ctx, chat, channel)
	if err != nil {
		t.Fatalf("GetWithFallback: %v", err)
	}
	if !ok {
		t.Fatal("expected a match via channel fallback")
	}
	if got.Provider != "codex" || got.Model != "gpt-5.2-codex" {
		t.Fatalf("expected channel-level selection, got %#v", got)
	}
	if matchedScope.ChatID != "" || matchedScope.Channel != "lark" {
		t.Fatalf("expected channel-level scope, got %#v", matchedScope)
	}
}

func TestGetWithFallback_NoneFound(t *testing.T) {
	t.Parallel()
	store := NewSelectionStore(filepath.Join(t.TempDir(), "llm_selection.json"))
	ctx := context.Background()

	channel := SelectionScope{Channel: "lark"}
	chat := SelectionScope{Channel: "lark", ChatID: "oc_chat", UserID: "ou_user"}

	_, _, ok, err := store.GetWithFallback(ctx, chat, channel)
	if err != nil {
		t.Fatalf("GetWithFallback: %v", err)
	}
	if ok {
		t.Fatal("expected no match")
	}
}

func TestGetWithFallback_ReturnsMatchedScope(t *testing.T) {
	t.Parallel()
	store := NewSelectionStore(filepath.Join(t.TempDir(), "llm_selection.json"))
	ctx := context.Background()

	// Only set channel-level; query with chat first, channel second.
	channel := SelectionScope{Channel: "lark"}
	chatA := SelectionScope{Channel: "lark", ChatID: "oc_a", UserID: "ou_user"}
	chatB := SelectionScope{Channel: "lark", ChatID: "oc_b", UserID: "ou_user"}

	channelSel := Selection{Mode: "cli", Provider: "codex", Model: "gpt-5.2-codex"}
	if err := store.Set(ctx, channel, channelSel); err != nil {
		t.Fatalf("Set channel: %v", err)
	}

	// Both chat scopes should fall back to channel.
	for _, chat := range []SelectionScope{chatA, chatB} {
		_, matched, ok, err := store.GetWithFallback(ctx, chat, channel)
		if err != nil {
			t.Fatalf("GetWithFallback: %v", err)
		}
		if !ok {
			t.Fatal("expected match")
		}
		if matched.Channel != "lark" || matched.ChatID != "" {
			t.Fatalf("expected channel-level scope match, got %#v", matched)
		}
	}
}
