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
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("LLM_MODEL", "gpt-4o-mini")

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
		_ = container.Cleanup()
	})
}
