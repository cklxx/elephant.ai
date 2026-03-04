package coding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeCLIID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"claude", "claude_code"},
		{"claude-code", "claude_code"},
		{"k2", "kimi"},
		{"gemini-cli", "gemini_cli"},
		{"Open Code", "open_code"},
	}
	for _, tc := range tests {
		if got := normalizeCLIID(tc.in); got != tc.want {
			t.Fatalf("normalizeCLIID(%q)=%q want=%q", tc.in, got, tc.want)
		}
	}
}

func TestDiscoverCodingCLIs_UsesSeedsAndCandidates(t *testing.T) {
	oldLookPath := detectLookPath
	oldExec := discoveryExecCommand
	t.Cleanup(func() {
		detectLookPath = oldLookPath
		discoveryExecCommand = oldExec
	})

	detectLookPath = func(file string) (string, error) {
		switch file {
		case "codex":
			return "/fake/codex", nil
		case "gemini":
			return "/fake/gemini", nil
		default:
			return "", os.ErrNotExist
		}
	}
	discoveryExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		_ = ctx
		joined := strings.Join(args, " ")
		script := "echo ok"
		if strings.Contains(joined, "--version") {
			script = "echo " + name + " v1.2.3"
		}
		if strings.Contains(joined, "--help") {
			script = "echo 'plan stream tool file web context'"
		}
		return exec.Command("sh", "-lc", script)
	}

	got := DiscoverCodingCLIs(context.Background(), DiscoveryOptions{
		Candidates:      []string{"gemini"},
		IncludePathScan: false,
		ProbeTimeout:    time.Second,
	})
	if len(got) < 2 {
		t.Fatalf("expected at least codex + gemini, got %d", len(got))
	}

	byID := map[string]DiscoveredCLICapability{}
	for _, item := range got {
		byID[item.ID] = item
	}
	if _, ok := byID["codex"]; !ok {
		t.Fatalf("expected codex in discovery results: %+v", got)
	}
	gem, ok := byID["gemini"]
	if !ok {
		t.Fatalf("expected gemini in discovery results: %+v", got)
	}
	if !gem.SupportsPlan || !gem.SupportsExecute || !gem.SupportsStream {
		t.Fatalf("expected gemini capabilities inferred, got %+v", gem)
	}
}

func TestScanPATHExecutableNames(t *testing.T) {
	tmpDir := t.TempDir()
	pathBin := filepath.Join(tmpDir, "codex-helper")
	if err := os.WriteFile(pathBin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	otherBin := filepath.Join(tmpDir, "plain-tool")
	if err := os.WriteFile(otherBin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write other bin: %v", err)
	}

	oldPath := discoveryPathEnv
	oldReadDir := discoveryReadDir
	t.Cleanup(func() {
		discoveryPathEnv = oldPath
		discoveryReadDir = oldReadDir
	})
	discoveryPathEnv = func(key string) string {
		if key != "PATH" {
			return ""
		}
		return tmpDir
	}

	names := scanPATHExecutableNames()
	if len(names) != 1 || names[0] != "codex-helper" {
		t.Fatalf("unexpected PATH scan result: %v", names)
	}
}
