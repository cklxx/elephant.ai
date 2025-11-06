package main

import "testing"

// TestBuildContainerWithOptions_DisableSandbox ensures CLI mode never initializes sandbox execution.
func TestBuildContainerWithOptions_DisableSandbox(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("LLM_MODEL", "gpt-4o-mini")
	t.Setenv("SANDBOX_BASE_URL", "https://sandbox.example.com")

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
