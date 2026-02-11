package coding

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func disableFallbackCLIPaths(t *testing.T) {
	t.Helper()
	oldHomeDir := detectUserHomeDir
	detectUserHomeDir = func() (string, error) {
		return "", fmt.Errorf("home dir disabled for deterministic test")
	}
	t.Cleanup(func() {
		detectUserHomeDir = oldHomeDir
	})
}

func TestDetectLocalCLIs_IncludesSupportedAndUnsupported(t *testing.T) {
	disableFallbackCLIPaths(t)

	old := detectLookPath
	defer func() { detectLookPath = old }()

	detectLookPath = func(name string) (string, error) {
		switch name {
		case "codex":
			return "/fake/codex", nil
		case "claude":
			return "/fake/claude", nil
		case "kimi":
			return "/fake/kimi", nil
		default:
			return "", fmt.Errorf("%s not found", name)
		}
	}

	got := DetectLocalCLIs()
	if len(got) != 3 {
		t.Fatalf("expected 3 detected CLIs, got %d: %+v", len(got), got)
	}
	if got[0].ID != "codex" || !got[0].AdapterSupport || got[0].AgentType != "codex" {
		t.Fatalf("unexpected codex detection: %+v", got[0])
	}
	if got[1].ID != "claude" || !got[1].AdapterSupport || got[1].AgentType != "claude_code" {
		t.Fatalf("unexpected claude detection: %+v", got[1])
	}
	if got[2].ID != "kimi" || got[2].AdapterSupport || got[2].AgentType != "" {
		t.Fatalf("unexpected kimi detection: %+v", got[2])
	}
}

func TestDetectLocalAdapters_ReturnsOnlyIntegratedAdapters(t *testing.T) {
	disableFallbackCLIPaths(t)

	old := detectLookPath
	defer func() { detectLookPath = old }()

	detectLookPath = func(name string) (string, error) {
		switch name {
		case "codex":
			return "/fake/codex", nil
		case "k2":
			return "/fake/k2", nil
		default:
			return "", fmt.Errorf("%s not found", name)
		}
	}

	got := DetectLocalAdapters()
	want := []string{"codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected adapters: got=%v want=%v", got, want)
	}
}

func TestDetectLocalCLIs_UsesFallbackBinaryNames(t *testing.T) {
	disableFallbackCLIPaths(t)

	old := detectLookPath
	defer func() { detectLookPath = old }()

	detectLookPath = func(name string) (string, error) {
		switch name {
		case "claude-code":
			return "/fake/claude-code", nil
		default:
			return "", fmt.Errorf("%s not found", name)
		}
	}

	got := DetectLocalCLIs()
	if len(got) != 1 {
		t.Fatalf("expected single detected CLI, got %d: %+v", len(got), got)
	}
	if got[0].ID != "claude" || got[0].Binary != "claude-code" {
		t.Fatalf("unexpected detection result: %+v", got[0])
	}
}

func TestDetectLocalCLIs_UsesFallbackHomePathsWhenPATHMissing(t *testing.T) {
	oldLookPath := detectLookPath
	oldHomeDir := detectUserHomeDir
	defer func() {
		detectLookPath = oldLookPath
		detectUserHomeDir = oldHomeDir
	}()

	detectLookPath = func(name string) (string, error) {
		return "", fmt.Errorf("%s not found in PATH", name)
	}

	tmpHome := t.TempDir()
	detectUserHomeDir = func() (string, error) { return tmpHome, nil }

	codexPath := filepath.Join(tmpHome, ".bun", "bin", "codex")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o755); err != nil {
		t.Fatalf("mkdir codex fallback dir: %v", err)
	}
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\necho codex"), 0o755); err != nil {
		t.Fatalf("write codex fallback binary: %v", err)
	}

	got := DetectLocalCLIs()
	if len(got) != 1 {
		t.Fatalf("expected one detected cli from fallback path, got %d: %+v", len(got), got)
	}
	if got[0].ID != "codex" || got[0].Path != codexPath {
		t.Fatalf("unexpected fallback detection: %+v", got[0])
	}
}
