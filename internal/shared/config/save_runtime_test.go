package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSaveRuntimeField_NewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	path, err := SaveRuntimeField("max_iterations", 200,
		WithConfigPath(cfgPath),
		WithFileReader(os.ReadFile),
	)
	if err != nil {
		t.Fatalf("SaveRuntimeField: %v", err)
	}
	if path != cfgPath {
		t.Fatalf("expected path %q, got %q", cfgPath, path)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	runtime, ok := parsed["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime section, got %#v", parsed)
	}
	if v, ok := runtime["max_iterations"]; !ok || v != 200 {
		t.Fatalf("expected max_iterations=200, got %v", v)
	}
}

func TestSaveRuntimeField_MergesExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	initial := []byte("runtime:\n  llm_provider: mock\n  max_iterations: 100\n")
	if err := os.WriteFile(cfgPath, initial, 0o600); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	_, err := SaveRuntimeField("max_iterations", 250,
		WithConfigPath(cfgPath),
		WithFileReader(os.ReadFile),
	)
	if err != nil {
		t.Fatalf("SaveRuntimeField: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	runtime := parsed["runtime"].(map[string]any)
	if v := runtime["max_iterations"]; v != 250 {
		t.Fatalf("expected max_iterations=250, got %v", v)
	}
	if v := runtime["llm_provider"]; v != "mock" {
		t.Fatalf("expected llm_provider preserved as mock, got %v", v)
	}
}

func TestSaveRuntimeField_StringValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	_, err := SaveRuntimeField("environment", "production",
		WithConfigPath(cfgPath),
		WithFileReader(os.ReadFile),
	)
	if err != nil {
		t.Fatalf("SaveRuntimeField: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "environment: production") {
		t.Fatalf("expected environment: production in config, got:\n%s", string(data))
	}
}
