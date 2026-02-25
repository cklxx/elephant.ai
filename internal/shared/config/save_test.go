package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSaveFollowPreferencesCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	saved, err := SaveFollowPreferences(true, false, WithConfigPath(path))
	if err != nil {
		t.Fatalf("SaveFollowPreferences returned error: %v", err)
	}
	if saved != path {
		t.Fatalf("expected path %q, got %q", path, saved)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}

	runtimeSection, ok := raw["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime section, got %v", raw["runtime"])
	}
	if v, ok := runtimeSection["follow_transcript"].(bool); !ok || !v {
		t.Fatalf("expected follow_transcript true, got %v", runtimeSection["follow_transcript"])
	}
	if v, ok := runtimeSection["follow_stream"].(bool); !ok || v {
		t.Fatalf("expected follow_stream false, got %v", runtimeSection["follow_stream"])
	}
}

func TestSaveFollowPreferencesPreservesExistingValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	existing := map[string]any{
		"runtime": map[string]any{
			"api_key": "secret",
		},
		"nested": map[string]any{"value": 42.0},
	}
	data, err := yaml.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if _, err := SaveFollowPreferences(false, true, WithConfigPath(path)); err != nil {
		t.Fatalf("SaveFollowPreferences returned error: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(updated, &raw); err != nil {
		t.Fatalf("decode updated: %v", err)
	}

	runtimeSection, ok := raw["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime section, got %v", raw["runtime"])
	}
	if runtimeSection["api_key"] != "secret" {
		t.Fatalf("expected api_key preserved, got %v", runtimeSection["api_key"])
	}
	nested, ok := raw["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map preserved, got %v", raw["nested"])
	}
	switch value := nested["value"].(type) {
	case int:
		if value != 42 {
			t.Fatalf("expected nested value 42, got %v", value)
		}
	case int64:
		if value != 42 {
			t.Fatalf("expected nested value 42, got %v", value)
		}
	case float64:
		if value != 42 {
			t.Fatalf("expected nested value 42, got %v", value)
		}
	default:
		t.Fatalf("expected numeric nested value, got %T", nested["value"])
	}
	if v, ok := runtimeSection["follow_transcript"].(bool); !ok || v {
		t.Fatalf("expected follow_transcript false, got %v", runtimeSection["follow_transcript"])
	}
	if v, ok := runtimeSection["follow_stream"].(bool); !ok || !v {
		t.Fatalf("expected follow_stream true, got %v", runtimeSection["follow_stream"])
	}
}

func TestSaveFollowPreferencesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("invalid: ["), 0o600); err != nil {
		t.Fatalf("write invalid: %v", err)
	}

	if _, err := SaveFollowPreferences(true, true, WithConfigPath(path)); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestSaveFollowPreferencesHonorsEnvConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	saved, err := SaveFollowPreferences(
		true,
		true,
		WithEnv(envMap{"ALEX_CONFIG_PATH": path}.Lookup),
		WithHomeDir(func() (string, error) { return "", errors.New("home disabled") }),
	)
	if err != nil {
		t.Fatalf("SaveFollowPreferences returned error: %v", err)
	}
	if saved != path {
		t.Fatalf("expected path %q, got %q", path, saved)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}
