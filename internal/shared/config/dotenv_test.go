package config

import (
	"os"
	"path/filepath"
	"testing"
)

func resetManagedDotEnvStateForTest(t *testing.T) {
	t.Helper()
	managedDotEnvState.mu.Lock()
	defer managedDotEnvState.mu.Unlock()
	for key := range managedDotEnvState.keys {
		_ = os.Unsetenv(key)
	}
	managedDotEnvState.keys = map[string]struct{}{}
}

func TestReloadManagedDotEnvUpdatesManagedKeys(t *testing.T) {
	resetManagedDotEnvStateForTest(t)
	defer resetManagedDotEnvStateForTest(t)

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("A=one\nB=two\n"), 0o644); err != nil {
		t.Fatalf("write initial .env: %v", err)
	}

	if err := LoadDotEnv(envPath); err != nil {
		t.Fatalf("load .env: %v", err)
	}
	if got, ok := lookupEnvOrEmpty("A"); !ok || got != "one" {
		t.Fatalf("expected A=one, got %q", got)
	}
	if got, ok := lookupEnvOrEmpty("B"); !ok || got != "two" {
		t.Fatalf("expected B=two, got %q", got)
	}

	if err := os.WriteFile(envPath, []byte("A=updated\nC=three\n"), 0o644); err != nil {
		t.Fatalf("write updated .env: %v", err)
	}
	if err := ReloadManagedDotEnv(envPath); err != nil {
		t.Fatalf("reload managed .env: %v", err)
	}
	if got, ok := lookupEnvOrEmpty("A"); !ok || got != "updated" {
		t.Fatalf("expected A=updated after reload, got %q", got)
	}
	if got, ok := lookupEnvOrEmpty("C"); !ok || got != "three" {
		t.Fatalf("expected C=three after reload, got %q", got)
	}
	if _, ok := os.LookupEnv("B"); ok {
		t.Fatalf("expected B to be unset after removal from .env")
	}
}

func TestReloadManagedDotEnvPreservesExternalEnvKeys(t *testing.T) {
	resetManagedDotEnvStateForTest(t)
	defer resetManagedDotEnvStateForTest(t)

	t.Setenv("EXTERNAL_ONLY", "shell-value")

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("EXTERNAL_ONLY=file-value\nMANAGED_ONLY=one\n"), 0o644); err != nil {
		t.Fatalf("write initial .env: %v", err)
	}

	if err := LoadDotEnv(envPath); err != nil {
		t.Fatalf("load .env: %v", err)
	}
	if got, ok := lookupEnvOrEmpty("EXTERNAL_ONLY"); !ok || got != "shell-value" {
		t.Fatalf("expected EXTERNAL_ONLY to preserve shell value, got %q", got)
	}
	if got, ok := lookupEnvOrEmpty("MANAGED_ONLY"); !ok || got != "one" {
		t.Fatalf("expected MANAGED_ONLY=one, got %q", got)
	}

	if err := os.WriteFile(envPath, []byte("EXTERNAL_ONLY=file-updated\nMANAGED_ONLY=two\n"), 0o644); err != nil {
		t.Fatalf("write updated .env: %v", err)
	}
	if err := ReloadManagedDotEnv(envPath); err != nil {
		t.Fatalf("reload managed .env: %v", err)
	}
	if got, ok := lookupEnvOrEmpty("EXTERNAL_ONLY"); !ok || got != "shell-value" {
		t.Fatalf("expected EXTERNAL_ONLY to remain shell value after reload, got %q", got)
	}
	if got, ok := lookupEnvOrEmpty("MANAGED_ONLY"); !ok || got != "two" {
		t.Fatalf("expected MANAGED_ONLY=two after reload, got %q", got)
	}
}

func lookupEnvOrEmpty(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	return value, true
}
