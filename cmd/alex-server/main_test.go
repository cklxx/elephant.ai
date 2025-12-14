package main

import (
	"testing"

	serverBootstrap "alex/internal/server/bootstrap"
)

func TestLoadConfigWithMockProvider(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "mock")

	cfg, _, _, err := serverBootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Runtime.LLMProvider != "mock" {
		t.Fatalf("expected mock provider, got %q", cfg.Runtime.LLMProvider)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Port)
	}
}
