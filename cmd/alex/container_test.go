package main

import (
	"io/fs"
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
		_ = filepath.Walk(homeDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			_ = os.Chmod(path, 0o700)
			return nil
		})
	})

	container, err := buildContainer()
	if err != nil {
		t.Fatalf("buildContainer returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Shutdown()
	})
}
