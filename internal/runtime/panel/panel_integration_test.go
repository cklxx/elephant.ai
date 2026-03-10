//go:build integration

package panel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeBinary creates a shell script that logs its args to a file and
// prints the given output to stdout. Returns the binary path and the args
// log file path.
func writeFakeBinary(t *testing.T, output string) (binPath, argsPath string) {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.log")
	script := filepath.Join(dir, "kaku")
	// Each invocation appends "---\n" + all args (one per line) to the log.
	content := fmt.Sprintf(`#!/bin/sh
ARGSFILE="%s"
echo "---" >> "$ARGSFILE"
for arg in "$@"; do echo "$arg" >> "$ARGSFILE"; done
echo '%s'
`, argsFile, output)
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return script, argsFile
}

// readArgsLog reads the args log and returns per-invocation arg slices.
func readArgsLog(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}

	var result [][]string
	var current []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "---" {
			if current != nil {
				result = append(result, current)
			}
			current = []string{}
			continue
		}
		if current != nil {
			current = append(current, line)
		}
	}
	if current != nil {
		result = append(result, current)
	}
	return result
}

// ---------------------------------------------------------------------------
// Test 1: KakuManager Split — mock exec returns pane ID
// ---------------------------------------------------------------------------

func TestKakuManager_Split(t *testing.T) {
	binPath, argsPath := writeFakeBinary(t, "42")
	mgr := &Manager{binary: binPath}

	ctx := context.Background()
	pane, err := mgr.Split(ctx, SplitOpts{
		ParentPaneID: 1,
		Direction:    "bottom",
		Percent:      70,
		WorkDir:      "/tmp/test",
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if pane.PaneID() != 42 {
		t.Fatalf("PaneID() = %d, want 42", pane.PaneID())
	}

	// Verify the CLI args passed to the fake binary.
	invocations := readArgsLog(t, argsPath)
	if len(invocations) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(invocations))
	}
	args := invocations[0]
	assertContainsInteg(t, args, "cli")
	assertContainsInteg(t, args, "split-pane")
	assertContainsInteg(t, args, "--pane-id")
	assertContainsInteg(t, args, "1")
	assertContainsInteg(t, args, "--bottom")
	assertContainsInteg(t, args, "--percent")
	assertContainsInteg(t, args, "70")
	assertContainsInteg(t, args, "--cwd")
	assertContainsInteg(t, args, "/tmp/test")
}

// ---------------------------------------------------------------------------
// Test 2: KakuPane SendText — InjectText uses paste mode, Submit uses --no-paste + CR
// ---------------------------------------------------------------------------

func TestKakuPane_SendText(t *testing.T) {
	binPath, argsPath := writeFakeBinary(t, "")
	pane := &Pane{ID: 7, binary: binPath}
	ctx := context.Background()

	// InjectText: paste mode (no --no-paste flag)
	if err := pane.InjectText(ctx, "hello world"); err != nil {
		t.Fatalf("InjectText: %v", err)
	}

	// Submit: --no-paste + carriage return
	if err := pane.Submit(ctx); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	invocations := readArgsLog(t, argsPath)
	if len(invocations) != 2 {
		t.Fatalf("expected 2 invocations, got %d", len(invocations))
	}

	// InjectText args: cli send-text --pane-id 7 "hello world"
	injectArgs := invocations[0]
	assertContainsInteg(t, injectArgs, "send-text")
	assertContainsInteg(t, injectArgs, "--pane-id")
	assertContainsInteg(t, injectArgs, "7")
	assertContainsInteg(t, injectArgs, "hello world")
	// InjectText should NOT have --no-paste (paste mode)
	for _, a := range injectArgs {
		if a == "--no-paste" {
			t.Fatal("InjectText should NOT include --no-paste (it uses paste mode)")
		}
	}

	// Submit args: cli send-text --no-paste --pane-id 7 \r
	// Note: the final \r arg is a raw carriage return that shell echo cannot
	// reliably log, so we verify the critical flags instead.
	submitArgs := invocations[1]
	assertContainsInteg(t, submitArgs, "send-text")
	assertContainsInteg(t, submitArgs, "--no-paste")
	assertContainsInteg(t, submitArgs, "--pane-id")
	assertContainsInteg(t, submitArgs, "7")
}

// ---------------------------------------------------------------------------
// Test 3: KakuPane CaptureOutput — mock returns terminal text
// ---------------------------------------------------------------------------

func TestKakuPane_CaptureOutput(t *testing.T) {
	termOutput := "$ ls\nfile1.go\nfile2.go\n$"
	binPath, argsPath := writeFakeBinary(t, termOutput)
	pane := &Pane{ID: 3, binary: binPath}
	ctx := context.Background()

	out, err := pane.CaptureOutput(ctx)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}

	if out != termOutput {
		t.Fatalf("CaptureOutput() = %q, want %q", out, termOutput)
	}

	invocations := readArgsLog(t, argsPath)
	if len(invocations) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(invocations))
	}
	args := invocations[0]
	assertContainsInteg(t, args, "cli")
	assertContainsInteg(t, args, "get-text")
	assertContainsInteg(t, args, "--pane-id")
	assertContainsInteg(t, args, "3")
}

// ---------------------------------------------------------------------------
// Test 4: AutoManager Fallback — Kaku not found → tmux manager
// ---------------------------------------------------------------------------

func TestAutoManager_Fallback(t *testing.T) {
	// Point KAKU_BIN to a non-existent path so Kaku init fails.
	t.Setenv("KAKU_BIN", "/nonexistent/kaku-binary-that-does-not-exist")

	// If tmux is available on this machine, AutoManager should fall back to it.
	// If neither is available, it should return a clear error.
	mgr, err := NewAutoManager()
	if err != nil {
		// Both unavailable — verify error mentions "no backend available".
		if !strings.Contains(err.Error(), "no backend available") {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Log("Neither kaku nor tmux available — fallback error verified")
		return
	}

	// If we got a manager, it must be a *TmuxManager (since kaku was blocked).
	if _, ok := mgr.(*TmuxManager); !ok {
		t.Fatalf("expected *TmuxManager fallback, got %T", mgr)
	}
}

// ---------------------------------------------------------------------------
// Test 5: TmuxPane SendKey — verify Ctrl-C maps to correct tmux args
// ---------------------------------------------------------------------------

func TestTmuxPane_SendKeys_CtrlC(t *testing.T) {
	rec := &commandRecorder{}
	pane := newTestTmuxPane(8, rec)
	ctx := context.Background()

	if err := pane.SendKey(ctx, "C-c"); err != nil {
		t.Fatalf("SendKey: %v", err)
	}

	if rec.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", rec.callCount())
	}

	c := rec.lastCall()
	// tmux send-keys -t %8 C-c
	wantArgs := []string{"send-keys", "-t", "%8", "C-c"}
	if c.Binary != "tmux" {
		t.Fatalf("binary = %q, want tmux", c.Binary)
	}
	if len(c.Args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", c.Args, wantArgs)
	}
	for i, a := range c.Args {
		if a != wantArgs[i] {
			t.Fatalf("args[%d] = %q, want %q", i, a, wantArgs[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertContainsInteg(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Fatalf("args %v does not contain %q", args, want)
}
