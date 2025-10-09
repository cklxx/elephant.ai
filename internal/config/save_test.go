package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFollowPreferencesCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

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
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}

	if v, ok := raw["follow_transcript"].(bool); !ok || !v {
		t.Fatalf("expected follow_transcript true, got %v", raw["follow_transcript"])
	}
	if v, ok := raw["follow_stream"].(bool); !ok || v {
		t.Fatalf("expected follow_stream false, got %v", raw["follow_stream"])
	}
}

func TestSaveFollowPreferencesPreservesExistingValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := map[string]any{
		"api_key": "secret",
		"nested":  map[string]any{"value": 42.0},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
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
	if err := json.Unmarshal(updated, &raw); err != nil {
		t.Fatalf("decode updated: %v", err)
	}

	if raw["api_key"] != "secret" {
		t.Fatalf("expected api_key preserved, got %v", raw["api_key"])
	}
	nested, ok := raw["nested"].(map[string]any)
	if !ok || nested["value"].(float64) != 42 {
		t.Fatalf("expected nested map preserved, got %v", raw["nested"])
	}
	if v, ok := raw["follow_transcript"].(bool); !ok || v {
		t.Fatalf("expected follow_transcript false, got %v", raw["follow_transcript"])
	}
	if v, ok := raw["follow_stream"].(bool); !ok || !v {
		t.Fatalf("expected follow_stream true, got %v", raw["follow_stream"])
	}
}

func TestSaveFollowPreferencesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write invalid: %v", err)
	}

	if _, err := SaveFollowPreferences(true, true, WithConfigPath(path)); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
