package subscription

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jsonx "alex/internal/shared/json"
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

func TestOnboardingStateStoreGetIgnoresTrailingGarbage(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "onboarding_state.json")
	malformed := `{
  "version": 1,
  "state": {
    "completed_at": "2026-02-11T00:00:00Z",
    "selected_provider": "llama_server",
    "selected_model": "unsloth/GLM-4.7-Flash-GGUF:Q4_K_M",
    "used_source": "llama_server"
  }
}
}`
	if err := os.WriteFile(path, []byte(malformed), 0o600); err != nil {
		t.Fatalf("write malformed state file: %v", err)
	}

	store := NewOnboardingStateStore(path)
	got, ok, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get malformed state: %v", err)
	}
	if !ok {
		t.Fatalf("expected onboarding state to be loaded")
	}
	if got.SelectedProvider != "llama_server" {
		t.Fatalf("unexpected provider: %q", got.SelectedProvider)
	}
	if got.SelectedModel != "unsloth/GLM-4.7-Flash-GGUF:Q4_K_M" {
		t.Fatalf("unexpected model: %q", got.SelectedModel)
	}
}

func TestOnboardingStateStoreSetRepairsTrailingGarbage(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "onboarding_state.json")
	malformed := `{
  "version": 1,
  "state": {
    "selected_provider": "codex",
    "selected_model": "gpt-5.2-codex"
  }
}
}`
	if err := os.WriteFile(path, []byte(malformed), 0o600); err != nil {
		t.Fatalf("write malformed state file: %v", err)
	}

	store := NewOnboardingStateStore(path)
	updated := OnboardingState{
		SelectedProvider: "llama_server",
		SelectedModel:    "unsloth/GLM-4.7-Flash-GGUF:Q4_K_M",
		UsedSource:       "llama_server",
	}
	if err := store.Set(context.Background(), updated); err != nil {
		t.Fatalf("set state over malformed file: %v", err)
	}

	got, ok, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get state after repair: %v", err)
	}
	if !ok {
		t.Fatalf("expected onboarding state after repair")
	}
	if got.SelectedProvider != updated.SelectedProvider || got.SelectedModel != updated.SelectedModel {
		t.Fatalf("unexpected state after repair: %+v", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired file: %v", err)
	}
	var doc onboardingStateDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		t.Fatalf("expected repaired file to be valid JSON: %v", err)
	}
	if doc.State.SelectedModel != updated.SelectedModel {
		t.Fatalf("unexpected repaired model: %q", doc.State.SelectedModel)
	}
}
