package panel

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultTmuxBin = "tmux"

// runFunc is the signature for the command runner used by tmux types.
// It exists so tests can replace exec calls with a recorder.
type runFunc func(ctx context.Context, binary string, args ...string) (string, error)

// defaultRun executes a binary and returns trimmed stdout.
func defaultRun(ctx context.Context, binary string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// TmuxPane implements PaneIface using tmux commands.
type TmuxPane struct {
	ID     int
	binary string
	runner runFunc
}

// PaneID returns the pane's numeric identifier.
func (p *TmuxPane) PaneID() int { return p.ID }

// target returns the tmux pane target string (e.g. "%12").
func (p *TmuxPane) target() string { return "%" + strconv.Itoa(p.ID) }

// InjectText sends text to the pane in literal/paste mode (does not submit).
func (p *TmuxPane) InjectText(ctx context.Context, text string) error {
	_, err := p.run(ctx, "send-keys", "-l", "-t", p.target(), text)
	if err != nil {
		return fmt.Errorf("panel: tmux inject text to pane %d: %w", p.ID, err)
	}
	return nil
}

// Submit sends Enter to the pane, submitting whatever is in the input buffer.
func (p *TmuxPane) Submit(ctx context.Context) error {
	_, err := p.run(ctx, "send-keys", "-t", p.target(), "Enter")
	if err != nil {
		return fmt.Errorf("panel: tmux submit to pane %d: %w", p.ID, err)
	}
	return nil
}

// Send injects text and immediately submits, with a small delay between.
func (p *TmuxPane) Send(ctx context.Context, text string) error {
	if err := p.InjectText(ctx, text); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return p.Submit(ctx)
}

// SendKey sends a special key sequence to the pane (e.g. "C-c" for Ctrl-C).
func (p *TmuxPane) SendKey(ctx context.Context, key string) error {
	_, err := p.run(ctx, "send-keys", "-t", p.target(), key)
	if err != nil {
		return fmt.Errorf("panel: tmux send-key %q to pane %d: %w", key, p.ID, err)
	}
	return nil
}

// CaptureOutput returns the current visible screen content of the pane.
func (p *TmuxPane) CaptureOutput(ctx context.Context) (string, error) {
	out, err := p.run(ctx, "capture-pane", "-t", p.target(), "-p")
	if err != nil {
		return "", fmt.Errorf("panel: tmux capture-pane %d: %w", p.ID, err)
	}
	return out, nil
}

// Activate focuses the pane.
func (p *TmuxPane) Activate(ctx context.Context) error {
	_, err := p.run(ctx, "select-pane", "-t", p.target())
	if err != nil {
		return fmt.Errorf("panel: tmux activate pane %d: %w", p.ID, err)
	}
	return nil
}

// Kill terminates the pane and its running process.
func (p *TmuxPane) Kill(ctx context.Context) error {
	_, err := p.run(ctx, "kill-pane", "-t", p.target())
	if err != nil {
		return fmt.Errorf("panel: tmux kill pane %d: %w", p.ID, err)
	}
	return nil
}

func (p *TmuxPane) run(ctx context.Context, args ...string) (string, error) {
	return p.runner(ctx, p.binary, args...)
}

// TmuxManager implements ManagerIface using tmux commands.
type TmuxManager struct {
	binary string
	runner runFunc
}

// NewTmuxManager creates a TmuxManager. Uses TMUX_BIN env var or looks for
// tmux on PATH.
func NewTmuxManager() (*TmuxManager, error) {
	bin := os.Getenv("TMUX_BIN")
	if bin == "" {
		bin = defaultTmuxBin
	}
	// Resolve via PATH to verify it exists.
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("panel: tmux binary not found (%s): %w", bin, err)
	}
	return &TmuxManager{binary: resolved, runner: defaultRun}, nil
}

// Split creates a new pane by splitting. Returns the new TmuxPane.
func (m *TmuxManager) Split(ctx context.Context, opts SplitOpts) (PaneIface, error) {
	dir := "-v" // vertical = bottom
	if opts.Direction == defaultSplitRight || opts.Direction == "right" {
		dir = "-h"
	}
	pct := opts.Percent
	if pct <= 0 || pct >= 100 {
		pct = 65
	}
	cwd := opts.WorkDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	args := []string{
		"split-window",
		dir,
		"-p", strconv.Itoa(pct),
		"-c", cwd,
		"-P", "-F", "#{pane_id}",
	}
	if opts.ParentPaneID > 0 {
		args = append(args, "-t", "%"+strconv.Itoa(opts.ParentPaneID))
	}

	out, err := m.runner(ctx, m.binary, args...)
	if err != nil {
		return nil, fmt.Errorf("panel: tmux split-window: %w", err)
	}

	paneID, err := parseTmuxPaneID(out)
	if err != nil {
		return nil, fmt.Errorf("panel: tmux split-window returned unexpected output %q: %w", out, err)
	}

	return &TmuxPane{ID: paneID, binary: m.binary, runner: m.runner}, nil
}

// List returns pane listing from tmux.
func (m *TmuxManager) List(ctx context.Context) (string, error) {
	out, err := m.runner(ctx, m.binary,
		"list-panes", "-a", "-F", "#{pane_id} #{window_id} #{pane_current_command}")
	if err != nil {
		return "", fmt.Errorf("panel: tmux list-panes: %w", err)
	}
	return out, nil
}

// parseTmuxPaneID parses a tmux pane ID like "%12" and returns the numeric part.
func parseTmuxPaneID(s string) (int, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "%") {
		return 0, fmt.Errorf("expected %%N format, got %q", s)
	}
	return strconv.Atoi(s[1:])
}

// NewAutoManager tries Kaku first, then falls back to tmux.
// Returns an error if neither backend is available.
func NewAutoManager() (ManagerIface, error) {
	if mgr, err := NewManager(); err == nil {
		return mgr, nil
	}
	if mgr, err := NewTmuxManager(); err == nil {
		return mgr, nil
	}
	return nil, fmt.Errorf("panel: no backend available (neither kaku nor tmux found)")
}

// Compile-time interface checks.
var (
	_ PaneIface    = (*TmuxPane)(nil)
	_ ManagerIface = (*TmuxManager)(nil)
)
