package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/internal/infra/coding"
	"alex/internal/infra/teamruntime"
	"alex/internal/shared/utils"
	"gopkg.in/yaml.v3"
)

const (
	teamStatusUsage     = "usage: alex team status [--runtime-root path] [--session-id id] [--team-id id] [--all] [--json] [--tail N]"
	defaultEventTailNum = 20
)

type teamStatusOptions struct {
	runtimeRoot string
	sessionID   string
	teamID      string
	includeAll  bool
	jsonOutput  bool
	eventsTail  int
}

type teamStatusReport struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Count       int                 `json:"count"`
	Entries     []teamRuntimeStatus `json:"entries"`
}

type teamRuntimeStatus struct {
	BaseDir                 string                           `json:"base_dir"`
	SessionID               string                           `json:"session_id"`
	TeamID                  string                           `json:"team_id"`
	Template                string                           `json:"template"`
	Goal                    string                           `json:"goal,omitempty"`
	InitializedAt           time.Time                        `json:"initialized_at"`
	TmuxSession             string                           `json:"tmux_session,omitempty"`
	CapabilitiesGeneratedAt time.Time                        `json:"capabilities_generated_at,omitempty"`
	CapabilitiesTTLSeconds  int                              `json:"capabilities_ttl_seconds,omitempty"`
	Capabilities            []coding.DiscoveredCLICapability `json:"capabilities,omitempty"`
	Roles                   []teamruntime.RoleBinding        `json:"roles,omitempty"`
	RuntimeState            teamruntime.RuntimeState         `json:"runtime_state"`
	RecentEvents            []map[string]any                 `json:"recent_events,omitempty"`
}

func runTeamStatus(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex team status")
	runtimeRoot := fs.String("runtime-root", "", "Team runtime root (_team_runtime). Default: auto-discover.")
	sessionID := fs.String("session-id", "", "Filter by session_id.")
	teamID := fs.String("team-id", "", "Filter by team_id.")
	all := fs.Bool("all", false, "Show all matched team runtimes (default: newest only).")
	jsonOut := fs.Bool("json", false, "Print JSON report.")
	tail := fs.Int("tail", defaultEventTailNum, "Number of recent events to show from events.jsonl.")
	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}
	if *tail < 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--tail must be >= 0")}
	}

	statuses, err := loadTeamRuntimeStatus(teamStatusOptions{
		runtimeRoot: strings.TrimSpace(*runtimeRoot),
		sessionID:   strings.TrimSpace(*sessionID),
		teamID:      strings.TrimSpace(*teamID),
		includeAll:  *all,
		jsonOutput:  *jsonOut,
		eventsTail:  *tail,
	})
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	if len(statuses) == 0 {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("no team runtime artifacts found")}
	}
	if !*all && len(statuses) > 1 {
		statuses = statuses[:1]
	}

	report := teamStatusReport{
		GeneratedAt: time.Now().UTC(),
		Count:       len(statuses),
		Entries:     statuses,
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Print(renderTeamStatusReport(report))
	return nil
}

func loadTeamRuntimeStatus(opts teamStatusOptions) ([]teamRuntimeStatus, error) {
	roots, err := resolveTeamRuntimeRoots(opts.runtimeRoot)
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{})
	out := make([]teamRuntimeStatus, 0, 16)
	for _, root := range roots {
		teamDirs, listErr := listTeamRuntimeDirs(root)
		if listErr != nil {
			return nil, listErr
		}
		for _, teamDir := range teamDirs {
			if _, ok := seen[teamDir]; ok {
				continue
			}
			seen[teamDir] = struct{}{}

			status, loadErr := loadSingleTeamRuntime(teamDir, opts.eventsTail)
			if loadErr != nil {
				continue
			}
			if opts.sessionID != "" && !strings.EqualFold(status.SessionID, opts.sessionID) {
				continue
			}
			if opts.teamID != "" && !strings.EqualFold(status.TeamID, opts.teamID) {
				continue
			}
			out = append(out, status)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].InitializedAt.Equal(out[j].InitializedAt) {
			if out[i].SessionID == out[j].SessionID {
				return out[i].TeamID < out[j].TeamID
			}
			return out[i].SessionID < out[j].SessionID
		}
		return out[i].InitializedAt.After(out[j].InitializedAt)
	})
	return out, nil
}

func resolveTeamRuntimeRoots(explicit string) ([]string, error) {
	trimmed := strings.TrimSpace(explicit)
	if trimmed != "" {
		if isDir(trimmed) {
			return []string{trimmed}, nil
		}
		return nil, fmt.Errorf("runtime root does not exist: %s", trimmed)
	}

	seen := make(map[string]struct{})
	add := func(path string) {
		if !isDir(path) {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		seen[abs] = struct{}{}
	}

	add(filepath.Join(".elephant", "tasks", "_team_runtime"))
	add(filepath.Join(".worktrees", "test", "tmp", "tasks", "_team_runtime"))

	worktreesRoot := ".worktrees"
	if isDir(worktreesRoot) {
		baseDepth := strings.Count(filepath.Clean(worktreesRoot), string(os.PathSeparator))
		_ = filepath.WalkDir(worktreesRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if strings.Count(filepath.Clean(path), string(os.PathSeparator))-baseDepth > 8 {
				return filepath.SkipDir
			}
			if d.Name() == "_team_runtime" {
				add(path)
				return filepath.SkipDir
			}
			return nil
		})
	}

	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func listTeamRuntimeDirs(root string) ([]string, error) {
	if !isDir(root) {
		return nil, nil
	}
	root = strings.TrimSpace(root)

	dirs := make(map[string]struct{})
	add := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" || !isDir(trimmed) {
			return
		}
		dirs[trimmed] = struct{}{}
	}

	if fileExists(filepath.Join(root, "bootstrap.yaml")) {
		add(root)
	}

	sessionTeamsRoot := filepath.Join(root, "teams")
	if isDir(sessionTeamsRoot) {
		entries, err := os.ReadDir(sessionTeamsRoot)
		if err != nil {
			return nil, fmt.Errorf("read teams dir %s: %w", sessionTeamsRoot, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				add(filepath.Join(sessionTeamsRoot, entry.Name()))
			}
		}
	}

	sessions, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read runtime root %s: %w", root, err)
	}
	for _, session := range sessions {
		if !session.IsDir() {
			continue
		}
		teamsDir := filepath.Join(root, session.Name(), "teams")
		if !isDir(teamsDir) {
			continue
		}
		teams, readErr := os.ReadDir(teamsDir)
		if readErr != nil {
			return nil, fmt.Errorf("read teams dir %s: %w", teamsDir, readErr)
		}
		for _, team := range teams {
			if team.IsDir() {
				add(filepath.Join(teamsDir, team.Name()))
			}
		}
	}

	out := make([]string, 0, len(dirs))
	for path := range dirs {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func loadSingleTeamRuntime(teamDir string, eventsTail int) (teamRuntimeStatus, error) {
	bootstrapPath := filepath.Join(teamDir, "bootstrap.yaml")
	var bootstrap teamruntime.BootstrapState
	if err := readYAMLFile(bootstrapPath, &bootstrap); err != nil {
		return teamRuntimeStatus{}, err
	}

	status := teamRuntimeStatus{
		BaseDir:       strings.TrimSpace(teamDir),
		SessionID:     strings.TrimSpace(bootstrap.SessionID),
		TeamID:        strings.TrimSpace(bootstrap.TeamID),
		Template:      strings.TrimSpace(bootstrap.Template),
		Goal:          strings.TrimSpace(bootstrap.Goal),
		InitializedAt: bootstrap.InitializedAt,
		TmuxSession:   strings.TrimSpace(bootstrap.TmuxSession),
	}

	capabilitiesPath := strings.TrimSpace(bootstrap.CapabilitiesPath)
	if capabilitiesPath == "" {
		capabilitiesPath = filepath.Join(teamDir, "capabilities.yaml")
	}
	var caps teamruntime.CapabilitySnapshot
	if err := readYAMLFile(capabilitiesPath, &caps); err == nil {
		status.CapabilitiesGeneratedAt = caps.GeneratedAt
		status.CapabilitiesTTLSeconds = caps.TTLSeconds
		status.Capabilities = append([]coding.DiscoveredCLICapability(nil), caps.Capabilities...)
	}

	roleRegistryPath := strings.TrimSpace(bootstrap.RoleRegistryPath)
	if roleRegistryPath == "" {
		roleRegistryPath = filepath.Join(teamDir, "role_registry.yaml")
	}
	var roles teamruntime.RoleRegistry
	if err := readYAMLFile(roleRegistryPath, &roles); err == nil {
		status.Roles = append([]teamruntime.RoleBinding(nil), roles.Roles...)
	}

	runtimeStatePath := strings.TrimSpace(bootstrap.RuntimeStatePath)
	if runtimeStatePath == "" {
		runtimeStatePath = filepath.Join(teamDir, "runtime_state.yaml")
	}
	_ = readYAMLFile(runtimeStatePath, &status.RuntimeState)

	eventLogPath := strings.TrimSpace(bootstrap.EventLogPath)
	if eventLogPath == "" {
		eventLogPath = filepath.Join(teamDir, "events.jsonl")
	}
	events, _ := readJSONLinesTail(eventLogPath, eventsTail)
	status.RecentEvents = events
	return status, nil
}

func renderTeamStatusReport(report teamStatusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Team runtime entries: %d\n", report.Count)
	for idx, entry := range report.Entries {
		if idx > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[%d] team=%s session=%s template=%s initialized=%s\n",
			idx+1,
			nonEmpty(entry.TeamID, "(unknown)"),
			nonEmpty(entry.SessionID, "(unknown)"),
			nonEmpty(entry.Template, "(unknown)"),
			formatStatusTime(entry.InitializedAt),
		)
		fmt.Fprintf(&b, "  dir: %s\n", entry.BaseDir)
		if entry.TmuxSession != "" {
			fmt.Fprintf(&b, "  tmux_session: %s\n", entry.TmuxSession)
		}
		fmt.Fprintf(&b, "  capabilities: generated_at=%s ttl=%ds count=%d\n",
			formatStatusTime(entry.CapabilitiesGeneratedAt),
			entry.CapabilitiesTTLSeconds,
			len(entry.Capabilities),
		)
		for _, cap := range entry.Capabilities {
			fmt.Fprintf(&b, "    - %s path=%s auth_ready=%t plan=%t exec=%t stream=%t fs=%t net=%t",
				nonEmpty(cap.ID, cap.Binary),
				nonEmpty(cap.Path, cap.Binary),
				cap.AuthReady,
				cap.SupportsPlan,
				cap.SupportsExecute,
				cap.SupportsStream,
				cap.SupportsFilesystem,
				cap.SupportsNetwork,
			)
			if cap.Version != "" {
				fmt.Fprintf(&b, " version=%s", cap.Version)
			}
			if cap.FailureReason != "" {
				fmt.Fprintf(&b, " failure=%s", cap.FailureReason)
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  roles: %d\n", len(entry.Roles))
		for _, role := range entry.Roles {
			fmt.Fprintf(&b, "    - %s target=%s selected=%s agent=%s pane=%s fallback=%s\n",
				nonEmpty(role.RoleID, "(unknown)"),
				nonEmpty(role.TargetCLI, "-"),
				nonEmpty(role.SelectedCLI, "-"),
				nonEmpty(role.SelectedAgentType, "-"),
				nonEmpty(role.TmuxPane, "-"),
				nonEmpty(strings.Join(role.FallbackCLIs, ","), "-"),
			)
		}
		if len(entry.RecentEvents) > 0 {
			fmt.Fprintf(&b, "  recent_events (%d):\n", len(entry.RecentEvents))
			for _, ev := range entry.RecentEvents {
				fmt.Fprintf(&b, "    - %s\n", formatRuntimeEventSummary(ev))
			}
		}
	}
	return b.String()
}

func formatRuntimeEventSummary(ev map[string]any) string {
	if len(ev) == 0 {
		return "(empty event)"
	}
	parts := make([]string, 0, 8)
	if ts := stringValue(ev["timestamp"]); ts != "" {
		parts = append(parts, ts)
	}
	if et := stringValue(ev["type"]); et != "" {
		parts = append(parts, et)
	}
	if role := stringValue(ev["role_id"]); role != "" {
		parts = append(parts, "role="+role)
	}
	if task := stringValue(ev["task_id"]); task != "" {
		parts = append(parts, "task="+task)
	}
	if tool := stringValue(ev["tool_name"]); tool != "" {
		parts = append(parts, "tool="+tool)
	}
	if pane := stringValue(ev["pane"]); pane != "" {
		parts = append(parts, "pane="+pane)
	}
	if msg := stringValue(ev["message"]); msg != "" {
		parts = append(parts, "msg="+msg)
	}
	if errMsg := stringValue(ev["error"]); errMsg != "" {
		parts = append(parts, "error="+errMsg)
	}
	if len(parts) == 0 {
		raw, _ := json.Marshal(ev)
		return string(raw)
	}
	return strings.Join(parts, " | ")
}

func readJSONLinesTail(path string, tail int) ([]map[string]any, error) {
	if tail <= 0 || utils.IsBlank(path) {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines := make([]string, 0, tail)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > tail {
			lines = lines[len(lines)-tail:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		out = append(out, event)
	}
	return out, nil
}

func readYAMLFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}

func isDir(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && !info.IsDir()
}

func formatStatusTime(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func nonEmpty(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func stringValue(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	default:
		return ""
	}
}
