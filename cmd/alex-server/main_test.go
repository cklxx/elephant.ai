package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	serverBootstrap "alex/internal/delivery/server/bootstrap"
)

func TestRun_DefaultModeUsesLark(t *testing.T) {
	t.Parallel()

	var larkCalls int
	err := run(
		[]string{"alex-server"},
		"obs.yaml",
		runners{
			runLark: func(obs string) error {
				larkCalls++
				if obs != "obs.yaml" {
					t.Fatalf("unexpected obs config: %q", obs)
				}
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if larkCalls != 1 {
		t.Fatalf("expected lark to be called once, got %d", larkCalls)
	}
}

func TestRun_UnknownSubcommandReturnsError(t *testing.T) {
	t.Parallel()

	err := run(
		[]string{"alex-server", "unexpected-mode"},
		"",
		runners{
			runLark: func(string) error {
				t.Fatal("lark should not be called")
				return nil
			},
		},
	)
	if err == nil {
		t.Fatal("expected unknown subcommand error")
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Fatalf("expected unknown subcommand error, got %v", err)
	}
}

func TestLoadConfigWithMockProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  llm_provider: \"mock\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", path)

	cr, err := serverBootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cr.Config.Runtime.LLMProvider != "mock" {
		t.Fatalf("expected mock provider, got %q", cr.Config.Runtime.LLMProvider)
	}
	if cr.Config.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cr.Config.Port)
	}
}
