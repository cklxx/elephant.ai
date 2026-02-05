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
	want := Selection{Mode: "cli", Provider: "ollama", Model: "llama3:latest", Source: "ollama"}

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
	want := Selection{Mode: "cli", Provider: "ollama", Model: "llama3:latest"}

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
	if err := os.WriteFile(path, []byte(`{"version":2,"selections":{"cli":{"mode":"cli","provider":"ollama","model":"llama3"}}}`), 0o600); err != nil {
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
	if err := os.WriteFile(path, []byte(`{"selections":{"cli":{"mode":"cli","provider":"ollama","model":"llama3"}}}`), 0o600); err != nil {
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
	if selection.Provider != "ollama" || selection.Model != "llama3" {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}
