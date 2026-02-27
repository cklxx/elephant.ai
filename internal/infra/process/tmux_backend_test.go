package process

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func tmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func TestTmuxBackend_Available(t *testing.T) {
	t.Parallel()
	b := &TmuxBackend{}
	got := b.Available()
	if tmuxAvailable() != got {
		t.Errorf("Available() = %v, want %v", got, tmuxAvailable())
	}
}

func TestTmuxSessionName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"backend", "elephant-backend"},
		{"bridge-claude_code-task.123", "elephant-bridge-claude_code-task-123"},
		{"dev:web", "elephant-dev-web"},
		{"a/b/c", "elephant-a-b-c"},
	}
	for _, tc := range tests {
		got := tmuxSessionName(tc.input)
		if got != tc.want {
			t.Errorf("tmuxSessionName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTmuxBackend_StartStop(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	t.Parallel()

	b := &TmuxBackend{}
	ctx := context.Background()

	h, err := b.Start(ctx, ProcessConfig{
		Name:    "test-tmux-sleep",
		Command: "sleep",
		Args:    []string{"30"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if h.Name() != "test-tmux-sleep" {
		t.Errorf("Name() = %q, want test-tmux-sleep", h.Name())
	}
	if h.PID() <= 0 {
		t.Errorf("PID() = %d, want > 0", h.PID())
	}
	if !h.Alive() {
		t.Error("expected Alive() = true after start")
	}

	// Verify tmux session exists.
	out, err := exec.Command("tmux", "-L", tmuxSocket, "has-session", "-t", "elephant-test-tmux-sleep").CombinedOutput()
	if err != nil {
		t.Errorf("tmux session not found: %s %v", string(out), err)
	}

	// StderrTail should return pane content.
	_ = h.StderrTail() // just ensure no panic

	// Stop.
	if err := h.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}

	// Wait for done.
	select {
	case <-h.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Done()")
	}

	if h.Alive() {
		t.Error("expected Alive() = false after stop")
	}
}

func TestTmuxBackend_ProcessExitsNaturally(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	t.Parallel()

	b := &TmuxBackend{}
	ctx := context.Background()

	// Use a command that lives long enough for tmux to register the pane PID.
	h, err := b.Start(ctx, ProcessConfig{
		Name:    "test-tmux-short-exit",
		Command: "sleep",
		Args:    []string{"0.5"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Process should exit within a few seconds.
	select {
	case <-h.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for natural exit")
	}

	if h.Alive() {
		t.Error("expected Alive() = false after natural exit")
	}
}

func TestTmuxBackend_InstantExit(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	t.Parallel()

	b := &TmuxBackend{}
	ctx := context.Background()

	// Instant-exit command: should return an already-done handle, not an error.
	h, err := b.Start(ctx, ProcessConfig{
		Name:    "test-tmux-instant-exit",
		Command: "true",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case <-h.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Done()")
	}
}

func TestTmuxBackend_WithEnv(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	t.Parallel()

	b := &TmuxBackend{}
	ctx := context.Background()

	h, err := b.Start(ctx, ProcessConfig{
		Name:    "test-tmux-env",
		Command: "sleep",
		Args:    []string{"30"},
		Env: map[string]string{
			"TEST_VAR": "hello",
		},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = h.Stop() }()

	if !h.Alive() {
		t.Error("expected process to be alive")
	}
}

func TestTmuxBackend_WorkingDir(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	t.Parallel()

	b := &TmuxBackend{}
	ctx := context.Background()
	dir := t.TempDir()

	h, err := b.Start(ctx, ProcessConfig{
		Name:       "test-tmux-workdir",
		Command:    "sleep",
		Args:       []string{"30"},
		WorkingDir: dir,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = h.Stop() }()

	if !h.Alive() {
		t.Error("expected process to be alive")
	}
}

func TestController_StartTmux(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	t.Parallel()

	ctrl := NewController()
	ctx := context.Background()

	h, err := ctrl.StartTmux(ctx, ProcessConfig{
		Name:    "test-ctrl-tmux",
		Command: "sleep",
		Args:    []string{"30"},
	})
	if err != nil {
		t.Fatalf("StartTmux: %v", err)
	}
	defer func() { _ = h.Stop() }()

	if h.PID() <= 0 {
		t.Errorf("PID = %d, want > 0", h.PID())
	}

	// Should appear in List().
	list := ctrl.List()
	found := false
	for _, info := range list {
		if info.Name == "test-ctrl-tmux" {
			found = true
			if info.Backend != "tmux" {
				t.Errorf("Backend = %q, want tmux", info.Backend)
			}
		}
	}
	if !found {
		t.Error("expected process in List()")
	}
}

func TestController_StartTmux_Fallback(t *testing.T) {
	t.Parallel()

	// Force tmux unavailable by testing the Available check.
	ctrl := NewController()
	if !ctrl.TmuxAvailable() {
		// Can't test fallback properly if tmux is actually missing — the test
		// would exercise the fallback path naturally. Just verify it works.
		ctx := context.Background()
		h, err := ctrl.StartTmux(ctx, ProcessConfig{
			Name:    "test-fallback",
			Command: "sleep",
			Args:    []string{"1"},
		})
		if err != nil {
			t.Fatalf("StartTmux fallback: %v", err)
		}
		defer func() { _ = h.Stop() }()
		if h.PID() <= 0 {
			t.Errorf("PID = %d, want > 0", h.PID())
		}
	}
}
