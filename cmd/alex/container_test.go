package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildContainer(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("GOTOOLCHAIN", "local")
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

	t.Cleanup(func() {
		// Go telemetry may create files under $HOME asynchronously.
		// Force-remove everything so t.TempDir() cleanup succeeds.
		_ = os.RemoveAll(homeDir)
	})

	container, err := buildContainer()
	if err != nil {
		t.Fatalf("buildContainer returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Container.Shutdown()
	})
}
