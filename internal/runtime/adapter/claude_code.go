package adapter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/runtime/panel"
)

const (
	ccWelcomeDelay = 1500 * time.Millisecond // wait for CC welcome screen to render
	ccPollInterval = 3 * time.Second          // pane output poll interval
	ccShellPrompt  = "$ "                     // bash prompt that signals CC exited
)

// ClaudeCodeAdapter launches Claude Code in interactive mode inside a Kaku pane.
//
// Start() flow:
//  1. panel.Manager.Split() → new pane
//  2. Inject: unset CLAUDECODE && claude --dangerously-skip-permissions
//  3. Sleep 1.5 s for CC welcome screen
//  4. InjectText(goal) + Submit()
//  5. Background goroutine polls pane output for the bash prompt, signals completion.
type ClaudeCodeAdapter struct {
	pm       panel.ManagerIface
	sink     HookSink
	hooksURL string // RUNTIME_HOOKS_URL to pass to CC

	mu    sync.Mutex
	panes map[string]*panel.Pane // sessionID → pane
}

// NewClaudeCodeAdapter creates an adapter for Claude Code sessions.
// hooksURL is the base URL of the runtime hooks endpoint (used to inject
// RUNTIME_HOOKS_URL into the CC pane environment).
func NewClaudeCodeAdapter(pm panel.ManagerIface, sink HookSink, hooksURL string) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{
		pm:       pm,
		sink:     sink,
		hooksURL: hooksURL,
		panes:    make(map[string]*panel.Pane),
	}
}

// Start creates a Kaku pane, launches Claude Code, and injects the goal.
func (a *ClaudeCodeAdapter) Start(ctx context.Context, sessionID, goal, workDir string, parentPaneID int) error {
	pane, err := a.pm.Split(ctx, panel.SplitOpts{
		ParentPaneID: parentPaneID,
		Direction:    "bottom",
		Percent:      65,
		WorkDir:      workDir,
	})
	if err != nil {
		return fmt.Errorf("claude_code adapter: split pane: %w", err)
	}

	a.mu.Lock()
	a.panes[sessionID] = pane
	a.mu.Unlock()

	// Activate the pane so the user can watch.
	_ = pane.Activate(ctx)

	// Export runtime session env vars so CC hooks can call back.
	envLine := fmt.Sprintf(
		"export RUNTIME_SESSION_ID=%s RUNTIME_HOOKS_URL=%s",
		shellQuote(sessionID),
		shellQuote(a.hooksURL),
	)
	if err := pane.Send(ctx, envLine); err != nil {
		return fmt.Errorf("claude_code adapter: set env: %w", err)
	}

	// Launch CC in interactive mode. Must unset CLAUDECODE to avoid nested-session detection.
	if err := pane.Send(ctx, "unset CLAUDECODE && claude --dangerously-skip-permissions"); err != nil {
		return fmt.Errorf("claude_code adapter: launch cc: %w", err)
	}

	// Wait for CC welcome screen to render (❯ prompt appears).
	time.Sleep(ccWelcomeDelay)

	// Inject goal text (paste mode, then submit with \r).
	if err := pane.InjectText(ctx, goal); err != nil {
		return fmt.Errorf("claude_code adapter: inject goal: %w", err)
	}
	if err := pane.Submit(ctx); err != nil {
		return fmt.Errorf("claude_code adapter: submit goal: %w", err)
	}

	// Background goroutine: poll pane output for bash $ prompt (CC exited).
	go a.watchForCompletion(ctx, sessionID, pane)

	return nil
}

// Inject sends additional text to the running CC session.
func (a *ClaudeCodeAdapter) Inject(ctx context.Context, sessionID, text string) error {
	pane := a.getPane(sessionID)
	if pane == nil {
		return fmt.Errorf("claude_code adapter: no pane for session %s", sessionID)
	}
	if err := pane.InjectText(ctx, text); err != nil {
		return err
	}
	return pane.Submit(ctx)
}

// Stop kills the CC pane and removes it from the registry.
func (a *ClaudeCodeAdapter) Stop(ctx context.Context, sessionID string) error {
	pane := a.removePane(sessionID)
	if pane == nil {
		return nil
	}
	return pane.Kill(ctx)
}

// watchForCompletion polls the pane output every ccPollInterval until it
// detects a bash prompt (indicating CC has exited), then calls OnCompleted
// or OnFailed. This is a fallback — structured completion events arrive
// via the runtime hooks handler (notify_runtime.sh).
func (a *ClaudeCodeAdapter) watchForCompletion(ctx context.Context, sessionID string, pane *panel.Pane) {
	ticker := time.NewTicker(ccPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// Check if the pane is still registered (Stop may have been called).
		if a.getPane(sessionID) == nil {
			return
		}

		out, err := pane.CaptureOutput(ctx)
		if err != nil {
			// Pane gone — CC exited unexpectedly.
			a.sink.OnFailed(sessionID, "pane closed unexpectedly")
			a.removePane(sessionID)
			return
		}

		// Look for bash $ prompt as the last non-empty line.
		if hasBashPrompt(out) {
			a.sink.OnCompleted(sessionID, "")
			a.removePane(sessionID)
			return
		}
	}
}

// hasBashPrompt returns true when the pane output shows a bare $ prompt,
// indicating that CC has exited back to the shell.
func hasBashPrompt(output string) bool {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// A bare "$ " (with optional colour codes stripped) signals the shell is idle.
		return strings.HasSuffix(line, "$") || strings.HasSuffix(line, ccShellPrompt)
	}
	return false
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (a *ClaudeCodeAdapter) getPane(sessionID string) *panel.Pane {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.panes[sessionID]
}

func (a *ClaudeCodeAdapter) removePane(sessionID string) *panel.Pane {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.panes[sessionID]
	delete(a.panes, sessionID)
	return p
}
