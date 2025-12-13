package main

import (
	"testing"

	runtimeconfig "alex/internal/config"
	serverBootstrap "alex/internal/server/bootstrap"
)

func TestLoadConfigDefaultsSandboxBaseURL(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "mock")

	cfg, _, _, err := serverBootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.SandboxBaseURL != runtimeconfig.DefaultSandboxBaseURL {
		t.Fatalf("expected default sandbox URL %q, got %q", runtimeconfig.DefaultSandboxBaseURL, cfg.Runtime.SandboxBaseURL)
	}
}

func TestLoadConfigWithSandboxBaseURL(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "mock")
	t.Setenv("SANDBOX_BASE_URL", "http://sandbox.example.com")

	cfg, _, _, err := serverBootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.SandboxBaseURL != "http://sandbox.example.com" {
		t.Fatalf("expected sandbox URL to be preserved, got %q", cfg.Runtime.SandboxBaseURL)
	}
}
