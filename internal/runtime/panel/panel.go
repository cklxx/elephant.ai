// Package panel wraps the Kaku CLI to manage terminal panes as session containers.
// It provides the primitive operations that MemberAdapters use to control
// their CLI processes: create a pane, inject text, capture output, kill.
//
// All operations call the kaku binary (resolved via KAKU_BIN env or default path).
// Users see all pane content live in the Kaku GUI — no separate UI required.
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

// ManagerIface is the interface satisfied by Manager.
// It allows adapters to accept a mock in tests without requiring a real Kaku binary.
type ManagerIface interface {
	Split(ctx context.Context, opts SplitOpts) (*Pane, error)
	List(ctx context.Context) (string, error)
}

const (
	defaultKakuBin     = "/Applications/Kaku.app/Contents/MacOS/kaku"
	defaultSplitBottom = "bottom"
	defaultSplitRight  = "right"
)

// Pane represents a single Kaku terminal pane.
type Pane struct {
	ID     int
	TabID  int
	binary string
}

// Manager creates and controls Kaku panes.
type Manager struct {
	binary string
}

// NewManager creates a Manager. Uses KAKU_BIN env var or the default binary path.
func NewManager() (*Manager, error) {
	bin := os.Getenv("KAKU_BIN")
	if bin == "" {
		bin = defaultKakuBin
	}
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("panel: kaku binary not found at %s (set KAKU_BIN to override)", bin)
	}
	return &Manager{binary: bin}, nil
}

// SplitOpts controls how a new pane is created.
type SplitOpts struct {
	ParentPaneID int
	Direction    string // "bottom" or "right"
	Percent      int    // percentage of the parent pane to allocate
	WorkDir      string
}

// Split creates a new pane by splitting an existing one.
// Returns the new Pane. The pane starts a login shell (zsh -l).
func (m *Manager) Split(ctx context.Context, opts SplitOpts) (*Pane, error) {
	dir := opts.Direction
	if dir == "" {
		dir = defaultSplitBottom
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
		"cli", "split-pane",
		"--pane-id", strconv.Itoa(opts.ParentPaneID),
		"--" + dir,
		"--percent", strconv.Itoa(pct),
		"--cwd", cwd,
		"--", "zsh", "-l",
	}

	out, err := m.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("panel: split-pane: %w", err)
	}

	paneID, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return nil, fmt.Errorf("panel: split-pane returned unexpected output %q: %w", out, err)
	}

	return &Pane{ID: paneID, binary: m.binary}, nil
}

// List returns the raw output of kaku cli list for introspection.
func (m *Manager) List(ctx context.Context) (string, error) {
	out, err := m.run(ctx, "cli", "list")
	if err != nil {
		return "", fmt.Errorf("panel: list: %w", err)
	}
	return out, nil
}

// InjectText sends text to the pane in paste mode (does not submit).
// Use Submit() after to trigger Enter.
func (p *Pane) InjectText(ctx context.Context, text string) error {
	_, err := p.run(ctx, "cli", "send-text", "--pane-id", strconv.Itoa(p.ID), text)
	if err != nil {
		return fmt.Errorf("panel: inject text to pane %d: %w", p.ID, err)
	}
	return nil
}

// Submit sends a carriage return to the pane, submitting whatever is in the
// input buffer. Required for interactive CLIs like Claude Code.
func (p *Pane) Submit(ctx context.Context) error {
	_, err := p.run(ctx, "cli", "send-text", "--no-paste", "--pane-id", strconv.Itoa(p.ID), "\r")
	if err != nil {
		return fmt.Errorf("panel: submit to pane %d: %w", p.ID, err)
	}
	return nil
}

// Send is a convenience that injects text and immediately submits.
// Use this for single-line shell commands. Do NOT use for interactive UIs
// that need to render the input before submit (use InjectText + Submit separately).
func (p *Pane) Send(ctx context.Context, text string) error {
	if err := p.InjectText(ctx, text); err != nil {
		return err
	}
	// Small delay so the pane processes the paste before the carriage return.
	time.Sleep(50 * time.Millisecond)
	return p.Submit(ctx)
}

// SendKey sends a special key sequence to the pane (e.g. "C-c" for Ctrl-C).
// Uses kaku cli send-text --no-paste to bypass paste mode.
func (p *Pane) SendKey(ctx context.Context, key string) error {
	_, err := p.run(ctx, "cli", "send-text", "--no-paste", "--pane-id", strconv.Itoa(p.ID), key)
	if err != nil {
		return fmt.Errorf("panel: send-key %q to pane %d: %w", key, p.ID, err)
	}
	return nil
}

// CaptureOutput returns the current visible screen content of the pane.
// This is a snapshot — long output is truncated to the visible terminal area.
func (p *Pane) CaptureOutput(ctx context.Context) (string, error) {
	out, err := p.run(ctx, "cli", "get-text", "--pane-id", strconv.Itoa(p.ID))
	if err != nil {
		return "", fmt.Errorf("panel: get-text pane %d: %w", p.ID, err)
	}
	return out, nil
}

// Activate focuses the pane in the Kaku GUI.
func (p *Pane) Activate(ctx context.Context) error {
	_, err := p.run(ctx, "cli", "activate-pane", "--pane-id", strconv.Itoa(p.ID))
	if err != nil {
		return fmt.Errorf("panel: activate pane %d: %w", p.ID, err)
	}
	return nil
}

// Kill terminates the pane and its running process.
func (p *Pane) Kill(ctx context.Context) error {
	_, err := p.run(ctx, "cli", "kill-pane", "--pane-id", strconv.Itoa(p.ID))
	if err != nil {
		return fmt.Errorf("panel: kill pane %d: %w", p.ID, err)
	}
	return nil
}

// run executes the kaku binary with the given args and returns trimmed stdout.
func (m *Manager) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, m.binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (p *Pane) run(ctx context.Context, args ...string) (string, error) {
	mgr := &Manager{binary: p.binary}
	return mgr.run(ctx, args...)
}
