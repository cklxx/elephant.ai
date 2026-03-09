package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	runtimepkg "alex/internal/runtime"
	"alex/internal/runtime/adapter"
	"alex/internal/runtime/panel"
	"alex/internal/runtime/session"
)

const (
	runtimeUsage        = "usage: alex runtime session {start|list|status|inject|stop}"
	runtimeStartUsage   = "usage: alex runtime session start --member <type> --goal <text> [--work-dir <dir>] [--parent-pane-id <N>] [--store-dir <dir>]"
	runtimeListUsage    = "usage: alex runtime session list [--state running|all] [--store-dir <dir>]"
	runtimeStatusUsage  = "usage: alex runtime session status <id> [--store-dir <dir>]"
	runtimeInjectUsage  = "usage: alex runtime session inject --id <id> --message <text> [--store-dir <dir>]"
	runtimeStopUsage    = "usage: alex runtime session stop --id <id> [--store-dir <dir>]"

	defaultRuntimeStoreDir = "~/.kaku/sessions"
)

func runRuntimeCommand(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stdout, runtimeUsage)
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "session":
		return runRuntimeSessionCommand(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown runtime subcommand %q (expected: session)", args[0])}
	}
}

func runRuntimeSessionCommand(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stdout, runtimeUsage)
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "start":
		return runRuntimeStart(args[1:])
	case "list":
		return runRuntimeList(args[1:])
	case "status":
		return runRuntimeStatus(args[1:])
	case "inject":
		return runRuntimeInject(args[1:])
	case "stop":
		return runRuntimeStop(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown session subcommand %q (expected: start|list|status|inject|stop)", args[0])}
	}
}

func runRuntimeStart(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex runtime session start")
	memberFlag := fs.String("member", "claude_code", "Member type: claude_code|codex|kimi|shell")
	goal := fs.String("goal", "", "Session goal (required)")
	workDir := fs.String("work-dir", "", "Working directory (defaults to current dir)")
	parentPaneID := fs.Int("parent-pane-id", -1, "Kaku pane ID to split from (env: KAKU_PANE_ID)")
	storeDir := fs.String("store-dir", "", "Session store directory (default: ~/.kaku/sessions)")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	goalVal := strings.TrimSpace(*goal)
	if goalVal == "" && len(fs.Args()) > 0 {
		goalVal = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if goalVal == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(runtimeStartUsage)}
	}

	member := session.MemberType(strings.TrimSpace(*memberFlag))
	if member == "" {
		member = session.MemberClaudeCode
	}

	dir := strings.TrimSpace(*workDir)
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("get working dir: %w", err)}
		}
	}

	// Resolve parent pane ID: flag > env.
	paneID := *parentPaneID
	if paneID < 0 {
		if env := strings.TrimSpace(os.Getenv("KAKU_PANE_ID")); env != "" {
			_, _ = fmt.Sscan(env, &paneID)
		}
	}

	sd := resolveStoreDir(*storeDir)
	rt, err := newRuntime(sd)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	// Wire a real adapter factory so CC/Codex actually launches.
	hooksURL := strings.TrimSpace(os.Getenv("RUNTIME_HOOKS_URL"))
	if hooksURL == "" {
		hooksURL = "http://localhost:8080"
	}
	pm, err := panel.NewManager()
	if err != nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("panel manager: %w", err)}
	}
	fac := adapter.NewFactory(pm, rt, hooksURL, nil)
	rt.SetFactory(fac)

	s, err := rt.CreateSession(member, goalVal, dir, "")
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := rt.StartSession(ctx, s.ID, paneID); err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	fmt.Fprintf(os.Stdout, "Session started: %s (member=%s pane=%d)\n", s.ID, member, paneID)
	return nil
}

func runRuntimeList(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex runtime session list")
	stateFilter := fs.String("state", "all", "Filter by state: running|all")
	storeDir := fs.String("store-dir", "", "Session store directory")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	rt, err := newRuntime(resolveStoreDir(*storeDir))
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	sessions := rt.ListSessions()
	filter := strings.ToLower(strings.TrimSpace(*stateFilter))

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tMEMBER\tSTATE\tGOAL\tAGE")
	count := 0
	for i := range sessions {
		s := &sessions[i]
		if filter == "running" && s.State != session.StateRunning && s.State != session.StateStarting {
			continue
		}
		age := time.Since(s.CreatedAt).Round(time.Second)
		goal := s.Goal
		if len(goal) > 50 {
			goal = goal[:47] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Member, s.State, goal, age)
		count++
	}
	_ = tw.Flush()
	if count == 0 {
		fmt.Fprintln(os.Stdout, "(no sessions)")
	}
	return nil
}

func runRuntimeStatus(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: errors.New(runtimeStatusUsage)}
	}

	fs, flagBuf := newBufferedFlagSet("alex runtime session status")
	storeDir := fs.String("store-dir", "", "Session store directory")
	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	id := strings.TrimSpace(fs.Args()[0])
	if id == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(runtimeStatusUsage)}
	}

	rt, err := newRuntime(resolveStoreDir(*storeDir))
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	s, ok := rt.GetSession(id)
	if !ok {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("session %s not found", id)}
	}

	// Marshal via a plain map to avoid copying the mutex embedded in session.Session.
	snap := map[string]any{
		"id":             s.ID,
		"member":         s.Member,
		"goal":           s.Goal,
		"work_dir":       s.WorkDir,
		"state":          s.State,
		"pane_id":        s.PaneID,
		"tab_id":         s.TabID,
		"created_at":     s.CreatedAt,
		"updated_at":     s.UpdatedAt,
		"started_at":     s.StartedAt,
		"ended_at":       s.EndedAt,
		"last_heartbeat": s.LastHeartbeat,
		"error_msg":      s.ErrorMsg,
		"answer":         s.Answer,
	}
	data, _ := json.MarshalIndent(snap, "", "  ")
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

func runRuntimeInject(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex runtime session inject")
	id := fs.String("id", "", "Session ID (required)")
	message := fs.String("message", "", "Text to inject (required)")
	storeDir := fs.String("store-dir", "", "Session store directory")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	sessionID := strings.TrimSpace(*id)
	msg := strings.TrimSpace(*message)
	if msg == "" && len(fs.Args()) > 0 {
		msg = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if sessionID == "" || msg == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(runtimeInjectUsage)}
	}

	rt, err := newRuntime(resolveStoreDir(*storeDir))
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rt.InjectText(ctx, sessionID, msg); err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	fmt.Fprintf(os.Stdout, "Injected into session %s\n", sessionID)
	return nil
}

func runRuntimeStop(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex runtime session stop")
	id := fs.String("id", "", "Session ID (required)")
	storeDir := fs.String("store-dir", "", "Session store directory")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	sessionID := strings.TrimSpace(*id)
	if sessionID == "" && len(fs.Args()) > 0 {
		sessionID = strings.TrimSpace(fs.Args()[0])
	}
	if sessionID == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(runtimeStopUsage)}
	}

	rt, err := newRuntime(resolveStoreDir(*storeDir))
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := rt.StopSession(ctx, sessionID); err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	fmt.Fprintf(os.Stdout, "Session stopped: %s\n", sessionID)
	return nil
}

// newRuntime creates a Runtime without a factory (no adapter required for CLI
// management commands that only query/modify persisted state).
func newRuntime(storeDir string) (*runtimepkg.Runtime, error) {
	rt, err := runtimepkg.New(storeDir, runtimepkg.Config{})
	if err != nil {
		return nil, fmt.Errorf("runtime: init: %w", err)
	}
	return rt, nil
}

// resolveStoreDir expands ~ and returns the effective store directory.
func resolveStoreDir(flag string) string {
	if d := strings.TrimSpace(flag); d != "" {
		return expandHome(d)
	}
	if d := strings.TrimSpace(os.Getenv("KAKU_STORE_DIR")); d != "" {
		return expandHome(d)
	}
	return expandHome(defaultRuntimeStoreDir)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + p[1:]
		}
	}
	return p
}
