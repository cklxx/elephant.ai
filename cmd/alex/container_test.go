package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildContainerWithOptions_DisableSandbox ensures CLI mode never initializes sandbox execution.
func TestBuildContainerWithOptions_DisableSandbox(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("LLM_MODEL", "gpt-4o-mini")
	t.Setenv("SANDBOX_BASE_URL", "https://sandbox.example.com")

	t.Cleanup(func() {
		_ = filepath.Walk(homeDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			_ = os.Chmod(path, 0o700)
			return nil
		})
	})

	container, err := buildContainerWithOptions(true)
	if err != nil {
		t.Fatalf("buildContainerWithOptions returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Cleanup()
	})

	if container.SandboxManager != nil {
		t.Fatal("SandboxManager should be nil when sandbox execution is disabled")
	}

	if container.Runtime.SandboxBaseURL != "" {
		t.Fatalf("expected sandbox base URL to be cleared, got %q", container.Runtime.SandboxBaseURL)
	}
}
