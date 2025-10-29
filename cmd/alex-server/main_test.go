package main

import "testing"

func TestLoadConfigDefaultsSandboxBaseURL(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("SANDBOX_BASE_URL", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.SandboxBaseURL != defaultSandboxBaseURL {
		t.Fatalf("expected default sandbox URL %q, got %q", defaultSandboxBaseURL, cfg.Runtime.SandboxBaseURL)
	}
}

func TestLoadConfigWithSandboxBaseURL(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("SANDBOX_BASE_URL", "http://sandbox.example.com")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.SandboxBaseURL != "http://sandbox.example.com" {
		t.Fatalf("expected sandbox URL to be preserved, got %q", cfg.Runtime.SandboxBaseURL)
	}
}
