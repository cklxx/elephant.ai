package main

import (
	"os"
	"path/filepath"
	"testing"

	serverBootstrap "alex/internal/delivery/server/bootstrap"
)

func TestLoadConfigWithMockProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  llm_provider: \"mock\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", path)

	cfg, _, _, _, err := serverBootstrap.LoadConfig()
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
