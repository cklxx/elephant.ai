package main

import (
	"bytes"
	"errors"
	"log"
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
		log.New(&bytes.Buffer{}, "", 0),
		runners{
			runLark: func(obs string) error {
				larkCalls++
				if obs != "obs.yaml" {
					t.Fatalf("unexpected obs config: %q", obs)
				}
				return nil
			},
			runKernelOnce: func(string) error {
				t.Fatal("kernel-once should not be called")
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

func TestRun_LegacyLarkSubcommandUsesLarkAndLogsDeprecation(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	err := run(
		[]string{"alex-server", "lark"},
		"",
		logger,
		runners{
			runLark: func(string) error { return nil },
			runKernelOnce: func(string) error {
				t.Fatal("kernel-once should not be called")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "DEPRECATED: 'alex-server lark'") {
		t.Fatalf("expected deprecation log, got: %q", buf.String())
	}
}

func TestRun_KernelOnceSubcommandUsesKernelRunner(t *testing.T) {
	t.Parallel()

	var kernelCalls int
	err := run(
		[]string{"alex-server", "kernel-once"},
		"",
		log.New(&bytes.Buffer{}, "", 0),
		runners{
			runLark: func(string) error {
				t.Fatal("lark should not be called")
				return nil
			},
			runKernelOnce: func(string) error {
				kernelCalls++
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if kernelCalls != 1 {
		t.Fatalf("expected kernel-once to be called once, got %d", kernelCalls)
	}
}

func TestRun_KernelOnceErrorPropagates(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	err := run(
		[]string{"alex-server", "kernel-once"},
		"",
		log.New(&bytes.Buffer{}, "", 0),
		runners{
			runLark:       func(string) error { return nil },
			runKernelOnce: func(string) error { return boom },
		},
	)
	if !errors.Is(err, boom) {
		t.Fatalf("expected %v, got %v", boom, err)
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
