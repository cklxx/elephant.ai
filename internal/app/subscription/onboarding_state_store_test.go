package subscription

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveOnboardingStatePath(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	custom := filepath.Join(tmp, "custom-onboarding.json")
	envLookup := func(key string) (string, bool) {
		switch key {
		case "ALEX_ONBOARDING_STATE_PATH":
			return custom, true
		default:
			return "", false
		}
	}

	if got := ResolveOnboardingStatePath(envLookup, nil); got != custom {
		t.Fatalf("expected custom path %q, got %q", custom, got)
	}
}

func TestOnboardingStateStoreSetGetClear(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "onboarding_state.json")
	store := NewOnboardingStateStore(path)

	state := OnboardingState{
		SelectedProvider: "codex",
		SelectedModel:    "gpt-5.2-codex",
		UsedSource:       "codex_cli",
	}
	if err := store.Set(context.Background(), state); err != nil {
		t.Fatalf("set state: %v", err)
	}

	got, ok, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if !ok {
		t.Fatalf("expected onboarding state to exist")
	}
	if got.SelectedProvider != "codex" || got.SelectedModel != "gpt-5.2-codex" {
		t.Fatalf("unexpected state: %+v", got)
	}
	if strings.TrimSpace(got.CompletedAt) == "" {
		t.Fatalf("expected completed_at to be auto-populated")
	}
	if _, err := time.Parse(time.RFC3339, got.CompletedAt); err != nil {
		t.Fatalf("invalid completed_at: %v (%q)", err, got.CompletedAt)
	}

	if err := store.Clear(context.Background()); err != nil {
		t.Fatalf("clear state: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected onboarding state file to be removed")
	}
	_, ok, err = store.Get(context.Background())
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if ok {
		t.Fatalf("expected no onboarding state after clear")
	}
}

func TestOnboardingStateStoreRejectsUnknownVersion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "onboarding_state.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"state":{"selected_provider":"codex"}}`), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}
	store := NewOnboardingStateStore(path)
	if _, _, err := store.Get(context.Background()); err == nil {
		t.Fatalf("expected version error")
	}
}
