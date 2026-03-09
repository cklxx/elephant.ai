package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"alex/internal/runtime/panel"
)

const (
	ccShellReadyDelay = 800 * time.Millisecond  // wait for zsh login shell to initialise
	ccWelcomeDelay    = 2500 * time.Millisecond // wait for CC welcome screen to render (❯ prompt)
	ccPollInterval    = 3 * time.Second         // pane output poll interval
	ccShellPrompt     = "$ "                    // bash prompt that signals CC exited
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

// ensureCCHooks reads ~/.claude/settings.json and ensures notify_runtime.sh is
// registered under hooks.PostToolUse and hooks.Stop. The write is atomic
// (temp file + rename). The function is idempotent and non-fatal: on any
// error it logs and returns without propagating.
func ensureCCHooks(hooksScriptPath string) {
	settingsPath := filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")

	// Read existing settings (create empty object if file absent).
	raw, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("ensureCCHooks: read %s: %v (skipping)", settingsPath, err)
		return
	}
	if os.IsNotExist(err) {
		raw = []byte("{}")
	}

	// Unmarshal into a generic map to preserve unknown keys.
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		log.Printf("ensureCCHooks: parse settings.json: %v (skipping)", err)
		return
	}

	// Retrieve or initialise the hooks map.
	hooksRaw, _ := settings["hooks"]
	hooksMap, ok := hooksRaw.(map[string]any)
	if !ok {
		hooksMap = make(map[string]any)
	}

	// hookEntry is the object we inject under each event key.
	hookEntry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hooksScriptPath,
				"async":   true,
			},
		},
	}

	changed := false
	for _, event := range []string{"PostToolUse", "Stop"} {
		if alreadyRegistered(hooksMap, event, hooksScriptPath) {
			continue
		}
		// Append the hook entry to the event slice.
		var entries []any
		if existing, ok := hooksMap[event]; ok {
			if sl, ok := existing.([]any); ok {
				entries = sl
			}
		}
		hooksMap[event] = append(entries, hookEntry)
		changed = true
	}

	if !changed {
		return // already up-to-date, nothing to write
	}

	settings["hooks"] = hooksMap

	// Marshal with indentation so the file stays human-readable.
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("ensureCCHooks: marshal: %v (skipping)", err)
		return
	}

	// Atomic write: temp file in the same directory + rename.
	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("ensureCCHooks: mkdir %s: %v (skipping)", dir, err)
		return
	}
	tmp, err := os.CreateTemp(dir, "settings-*.json.tmp")
	if err != nil {
		log.Printf("ensureCCHooks: create temp: %v (skipping)", err)
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		log.Printf("ensureCCHooks: write temp: %v (skipping)", err)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		log.Printf("ensureCCHooks: close temp: %v (skipping)", err)
		return
	}
	if err := os.Rename(tmpName, settingsPath); err != nil {
		os.Remove(tmpName)
		log.Printf("ensureCCHooks: rename to %s: %v (skipping)", settingsPath, err)
		return
	}

	log.Printf("ensureCCHooks: registered notify_runtime.sh hooks in %s", settingsPath)
}

// alreadyRegistered returns true if hooksScriptPath already appears in the
// command of any hook entry under the given event key.
func alreadyRegistered(hooksMap map[string]any, event, scriptPath string) bool {
	raw, ok := hooksMap[event]
	if !ok {
		return false
	}
	entries, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, e := range entries {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := em["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == scriptPath {
				return true
			}
		}
	}
	return false
}

// findNotifyScript locates notify_runtime.sh by searching workDir and its
// ancestors. This avoids registering a non-existent path when workDir is /tmp.
func findNotifyScript(workDir string) string {
	dir := workDir
	for {
		candidate := filepath.Join(dir, "scripts", "cc_hooks", "notify_runtime.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root without finding the script
		}
		dir = parent
	}
}

// Start creates a Kaku pane, launches Claude Code, and injects the goal.
func (a *ClaudeCodeAdapter) Start(ctx context.Context, sessionID, goal, workDir string, parentPaneID int) error {
	// Auto-register CC hooks. Walk up from workDir so /tmp doesn't register a wrong path.
	if script := findNotifyScript(workDir); script != "" {
		ensureCCHooks(script)
	}

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

	// Wait for the zsh login shell to finish initialising.
	// spawn/split starts a fresh zsh -l; sending commands before it's ready silently drops them.
	time.Sleep(ccShellReadyDelay)

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

	// Inject goal text then submit. A brief pause between inject and submit is
	// required: CC renders the input buffer asynchronously; submitting too soon
	// sends \r before CC's readline is ready, silently dropping the submission.
	if err := pane.InjectText(ctx, goal); err != nil {
		return fmt.Errorf("claude_code adapter: inject goal: %w", err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := pane.Submit(ctx); err != nil {
		return fmt.Errorf("claude_code adapter: submit goal: %w", err)
	}

	// Background goroutine: poll pane output for zsh % or bash $ prompt (CC exited).
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

		// Look for zsh % or bash $ prompt as the last non-empty line.
		if hasBashPrompt(out) {
			a.sink.OnCompleted(sessionID, "")
			a.removePane(sessionID)
			return
		}
	}
}

// hasBashPrompt returns true when the pane output shows a shell prompt,
// indicating that CC has exited back to the shell.
// Supports both bash (ends with "$" or "$ ") and zsh (ends with "%" or "% ").
func hasBashPrompt(output string) bool {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		return strings.HasSuffix(line, "$") || strings.HasSuffix(line, "$ ") ||
			strings.HasSuffix(line, "%") || strings.HasSuffix(line, "% ")
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
