package admin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestFileStoreLoadMissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "missing.yaml"))

	overrides, err := store.LoadOverrides(context.Background())
	if err != nil {
		t.Fatalf("LoadOverrides returned error: %v", err)
	}
	if overrides != (runtimeconfig.Overrides{}) {
		t.Fatalf("expected zero overrides for missing file, got %#v", overrides)
	}
}

func TestFileStoreLoadInvalidYAMLReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(path, []byte("invalid: ["), 0o600); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	store := NewFileStore(path)
	if _, err := store.LoadOverrides(context.Background()); err == nil {
		t.Fatal("expected error for invalid JSON data")
	}
}

func TestFileStoreSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	store := NewFileStore(path)

	provider := "anthropic"
	iterations := 42
	overrides := runtimeconfig.Overrides{LLMProvider: &provider, MaxIterations: &iterations}

	if err := store.SaveOverrides(context.Background(), overrides); err != nil {
		t.Fatalf("SaveOverrides returned error: %v", err)
	}

	loaded, err := store.LoadOverrides(context.Background())
	if err != nil {
		t.Fatalf("LoadOverrides returned error: %v", err)
	}

	if loaded.LLMProvider == nil || *loaded.LLMProvider != provider {
		t.Fatalf("expected provider %q, got %#v", provider, loaded.LLMProvider)
	}
	if loaded.MaxIterations == nil || *loaded.MaxIterations != iterations {
		t.Fatalf("expected iterations %d, got %#v", iterations, loaded.MaxIterations)
	}
}
