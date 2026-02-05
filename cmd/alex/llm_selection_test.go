package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appcontext "alex/internal/agent/app/context"
	runtimeconfig "alex/internal/config"
	"alex/internal/subscription"
)

func TestApplyPinnedCLILLMSelectionAttachesResolvedSelection(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return configPath, true
		}
		return "", false
	}
	storePath := subscription.ResolveSelectionStorePath(envLookup, nil)
	store := subscription.NewSelectionStore(storePath)
	if err := store.Set(context.Background(), subscription.SelectionScope{Channel: "cli"}, subscription.Selection{
		Mode:     "cli",
		Provider: "ollama",
		Model:    "llama3:latest",
	}); err != nil {
		t.Fatalf("seed selection store: %v", err)
	}

	ctx := applyPinnedCLILLMSelection(context.Background(), runtimeconfig.EnvLookup(envLookup), nil)
	selection, ok := appcontext.GetLLMSelection(ctx)
	if !ok {
		t.Fatalf("expected selection on context")
	}
	if selection.Provider != "ollama" || selection.Model != "llama3:latest" {
		t.Fatalf("unexpected selection: %#v", selection)
	}
	if !selection.Pinned {
		t.Fatalf("expected pinned selection")
	}
	if _, err := os.Stat(configPath); err == nil {
		t.Fatalf("expected config yaml to remain untouched")
	}
}

func TestApplyPinnedCLILLMSelectionIgnoresInvalidSelection(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return configPath, true
		}
		return "", false
	}
	storePath := subscription.ResolveSelectionStorePath(envLookup, nil)
	store := subscription.NewSelectionStore(storePath)
	if err := store.Set(context.Background(), subscription.SelectionScope{Channel: "cli"}, subscription.Selection{
		Mode:     "yaml",
		Provider: "ollama",
		Model:    "llama3:latest",
	}); err != nil {
		t.Fatalf("seed selection store: %v", err)
	}

	ctx := applyPinnedCLILLMSelection(context.Background(), runtimeconfig.EnvLookup(envLookup), nil)
	if _, ok := appcontext.GetLLMSelection(ctx); ok {
		t.Fatalf("expected selection to be ignored")
	}
}
