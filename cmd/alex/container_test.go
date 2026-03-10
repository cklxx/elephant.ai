package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildContainer(t *testing.T) {
	// Use os.MkdirTemp instead of t.TempDir() because buildContainer spawns
	// background goroutines (Go telemetry, file watchers) that write to $HOME
	// after the test returns, causing t.TempDir()'s strict cleanup to fail
	// with "directory not empty".
	homeDir, err := os.MkdirTemp("", "TestBuildContainer")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(homeDir) })

	t.Setenv("HOME", homeDir)
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOTELEMETRY", "off")

	configPath := filepath.Join(homeDir, ".alex", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`runtime:
  api_key: "test-key"
  llm_provider: "openai"
  llm_model: "gpt-4o-mini"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)

	container, err := buildContainer()
	if err != nil {
		t.Fatalf("buildContainer returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Container.Shutdown()
	})
}
