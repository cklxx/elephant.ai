package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/internal/infra/teamruntime"
	"alex/internal/shared/utils"
)

const (
	defaultTeamTimelineLimit = 8
	defaultArtifactScanDepth = 4
)

type teamRunView struct {
	Goal           string             `json:"goal,omitempty"`
	OverallStatus  string             `json:"overall_status"`
	StartedAt      time.Time          `json:"started_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	LastActivityAt time.Time          `json:"last_activity_at,omitempty"`
	SessionID      string             `json:"session_id"`
	TeamID         string             `json:"team_id"`
	FocusRoleID    string             `json:"focus_role_id,omitempty"`
	Roles          []teamRoleView     `json:"roles,omitempty"`
	Artifacts      []teamArtifactView `json:"artifacts,omitempty"`
	RecentEvents   []teamActivityView `json:"recent_events,omitempty"`
}

type teamRoleView struct {
	RoleID            string    `json:"role_id"`
	SelectedAgent     string    `json:"selected_agent"`
	Status            string    `json:"status"`
	ShortSummary      string    `json:"short_summary,omitempty"`
	LastActivityAt    time.Time `json:"last_activity_at,omitempty"`
	TerminalAvailable bool      `json:"terminal_available"`
	FollowUpAvailable bool      `json:"follow_up_available"`
}

type teamActivityView struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	RoleID    string    `json:"role_id,omitempty"`
	Type      string    `json:"type,omitempty"`
	Summary   string    `json:"summary"`
}

type teamArtifactView struct {
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	RoleID string `json:"role_id,omitempty"`
}

type teamTerminalSnapshotView struct {
	Title                string    `json:"title"`
	RoleID               string    `json:"role_id"`
	SelectedAgent        string    `json:"selected_agent"`
	Status               string    `json:"status"`
	Summary              string    `json:"summary,omitempty"`
	LastActivityAt       time.Time `json:"last_activity_at,omitempty"`
	Mode                 string    `json:"mode"`
	Lines                int       `json:"lines"`
	Content              string    `json:"content,omitempty"`
	OpenInteractiveLabel string    `json:"open_interactive_label,omitempty"`
	OpenInteractiveHint  string    `json:"open_interactive_hint,omitempty"`
	FollowUpHint         string    `json:"follow_up_hint,omitempty"`
}

func buildTeamRunView(entry teamRuntimeStatus) teamRunView {
	roleViews := buildRoleViews(entry)
	activity := buildActivityTimeline(entry.RecentEvents, defaultTeamTimelineLimit)
	focusRole := selectPreferredRoleView(roleViews)

	view := teamRunView{
		Goal:          strings.TrimSpace(entry.Goal),
		OverallStatus: normalizeTeamViewStatus(entry.RuntimeState.Status),
		StartedAt:     entry.InitializedAt,
		UpdatedAt:     entry.RuntimeState.UpdatedAt,
		SessionID:     strings.TrimSpace(entry.SessionID),
		TeamID:        strings.TrimSpace(entry.TeamID),
		Roles:         roleViews,
		Artifacts:     collectTeamArtifacts(entry),
		RecentEvents:  activity,
	}
	if !focusRole.LastActivityAt.IsZero() {
		view.LastActivityAt = focusRole.LastActivityAt
	}
	if strings.TrimSpace(focusRole.RoleID) != "" {
		view.FocusRoleID = focusRole.RoleID
	}
	if view.UpdatedAt.IsZero() {
		view.UpdatedAt = view.LastActivityAt
	}
	if view.UpdatedAt.IsZero() {
		view.UpdatedAt = view.StartedAt
	}
	return view
}

func buildRoleViews(entry teamRuntimeStatus) []teamRoleView {
	eventByRole := latestRoleEvents(entry.RecentEvents)
	out := make([]teamRoleView, 0, len(entry.Roles))
	for _, role := range entry.Roles {
		state := entry.RuntimeState.Roles[strings.TrimSpace(role.RoleID)]
		ev := eventByRole[strings.TrimSpace(role.RoleID)]
		status := normalizeRoleViewStatus(state.Status, ev)
		summary := summarizeRoleState(status, ev)
		lastActivity := state.UpdatedAt
		if ts := parseEventTimestamp(stringValue(ev["timestamp"])); !ts.IsZero() {
			lastActivity = ts
		}
		out = append(out, teamRoleView{
			RoleID:            strings.TrimSpace(role.RoleID),
			SelectedAgent:     roleViewAgent(role),
			Status:            status,
			ShortSummary:      summary,
			LastActivityAt:    lastActivity,
			TerminalAvailable: strings.TrimSpace(role.TmuxPane) != "",
			FollowUpAvailable: strings.TrimSpace(role.TmuxPane) != "",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi := rolePriority(out[i].Status)
		pj := rolePriority(out[j].Status)
		if pi != pj {
			return pi < pj
		}
		if !out[i].LastActivityAt.Equal(out[j].LastActivityAt) {
			return out[i].LastActivityAt.After(out[j].LastActivityAt)
		}
		return out[i].RoleID < out[j].RoleID
	})
	return out
}

func buildActivityTimeline(events []map[string]any, limit int) []teamActivityView {
	if len(events) == 0 || limit <= 0 {
		return nil
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	out := make([]teamActivityView, 0, len(events))
	for _, ev := range events {
		summary := summarizeActivityEvent(ev)
		if summary == "" {
			continue
		}
		out = append(out, teamActivityView{
			Timestamp: parseEventTimestamp(stringValue(ev["timestamp"])),
			RoleID:    strings.TrimSpace(stringValue(ev["role_id"])),
			Type:      strings.TrimSpace(stringValue(ev["type"])),
			Summary:   summary,
		})
	}
	return out
}

func latestRoleEvents(events []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any)
	for _, ev := range events {
		roleID := strings.TrimSpace(stringValue(ev["role_id"]))
		if roleID == "" {
			continue
		}
		out[roleID] = ev
	}
	return out
}

func selectPreferredRoleView(roles []teamRoleView) teamRoleView {
	if len(roles) == 0 {
		return teamRoleView{}
	}
	return roles[0]
}

func selectPreferredTerminalRole(entry teamRuntimeStatus, requestedRoleID string) (teamruntime.RoleBinding, bool, error) {
	if strings.TrimSpace(requestedRoleID) != "" {
		role, err := resolveInjectRole(entry, requestedRoleID)
		return role, false, err
	}
	if len(entry.Roles) == 0 {
		return teamruntime.RoleBinding{}, false, fmt.Errorf("no role bindings found in runtime")
	}

	viewByRole := make(map[string]teamRoleView, len(entry.View.Roles))
	for _, role := range entry.View.Roles {
		viewByRole[role.RoleID] = role
	}
	bestIdx := 0
	bestPriority := 99
	bestActivity := time.Time{}
	for i, binding := range entry.Roles {
		roleID := strings.TrimSpace(binding.RoleID)
		roleView := viewByRole[roleID]
		priority := rolePriority(roleView.Status)
		if strings.TrimSpace(binding.TmuxPane) == "" {
			priority += 10
		}
		if priority < bestPriority || (priority == bestPriority && roleView.LastActivityAt.After(bestActivity)) {
			bestIdx = i
			bestPriority = priority
			bestActivity = roleView.LastActivityAt
		}
	}
	return entry.Roles[bestIdx], len(entry.Roles) > 1, nil
}

func buildTerminalSnapshotView(
	entry teamRuntimeStatus,
	role teamruntime.RoleBinding,
	mode string,
	lines int,
	content string,
	runtimeRoot string,
) teamTerminalSnapshotView {
	roleID := strings.TrimSpace(role.RoleID)
	var selected teamRoleView
	for _, candidate := range entry.View.Roles {
		if strings.EqualFold(candidate.RoleID, roleID) {
			selected = candidate
			break
		}
	}
	title := "Live Terminal"
	if mode == "capture" {
		title = "Recent Output"
	}
	return teamTerminalSnapshotView{
		Title:                title,
		RoleID:               roleID,
		SelectedAgent:        selected.SelectedAgent,
		Status:               selected.Status,
		Summary:              selected.ShortSummary,
		LastActivityAt:       selected.LastActivityAt,
		Mode:                 mode,
		Lines:                lines,
		Content:              strings.TrimSpace(content),
		OpenInteractiveLabel: "Open Interactive View",
		OpenInteractiveHint:  buildTeamTerminalCommandHint(entry, roleID, "attach", runtimeRoot, 0),
		FollowUpHint:         buildTeamInjectCommandHint(entry, roleID, runtimeRoot),
	}
}

func renderTerminalSnapshotView(view teamTerminalSnapshotView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", nonEmpty(view.Title, "Live Terminal"))
	fmt.Fprintf(&b, "Role: %s\n", nonEmpty(view.RoleID, "(unknown)"))
	if strings.TrimSpace(view.SelectedAgent) != "" {
		fmt.Fprintf(&b, "Agent: %s\n", view.SelectedAgent)
	}
	if strings.TrimSpace(view.Status) != "" {
		fmt.Fprintf(&b, "Status: %s\n", view.Status)
	}
	if strings.TrimSpace(view.Summary) != "" {
		fmt.Fprintf(&b, "Summary: %s\n", view.Summary)
	}
	if !view.LastActivityAt.IsZero() {
		fmt.Fprintf(&b, "Last activity: %s\n", formatStatusTime(view.LastActivityAt))
	}
	fmt.Fprintf(&b, "Window: last %d lines (%s)\n", view.Lines, view.Mode)
	b.WriteString("\n")
	if strings.TrimSpace(view.Content) == "" {
		b.WriteString("(no terminal output captured)\n")
	} else {
		b.WriteString(view.Content)
		b.WriteString("\n")
	}
	if strings.TrimSpace(view.OpenInteractiveHint) != "" {
		fmt.Fprintf(&b, "\n%s: %s\n", nonEmpty(view.OpenInteractiveLabel, "Open Interactive View"), view.OpenInteractiveHint)
	}
	if strings.TrimSpace(view.FollowUpHint) != "" {
		fmt.Fprintf(&b, "Send Follow-up: %s\n", view.FollowUpHint)
	}
	return b.String()
}

func renderTeamRunView(view teamRunView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal: %s\n", nonEmpty(view.Goal, "(not recorded)"))
	fmt.Fprintf(&b, "Team status: %s\n", nonEmpty(view.OverallStatus, "initialized"))
	fmt.Fprintf(&b, "Session ID: %s\n", nonEmpty(view.SessionID, "(unknown)"))
	fmt.Fprintf(&b, "Team ID: %s\n", nonEmpty(view.TeamID, "(unknown)"))
	fmt.Fprintf(&b, "Start time: %s\n", formatStatusTime(view.StartedAt))
	fmt.Fprintf(&b, "Updated: %s\n", formatStatusTime(view.UpdatedAt))
	if strings.TrimSpace(view.FocusRoleID) != "" {
		fmt.Fprintf(&b, "Focus role: %s\n", view.FocusRoleID)
	}
	b.WriteString("\nRoles:\n")
	if len(view.Roles) == 0 {
		b.WriteString("  (no roles)\n")
	} else {
		for _, role := range view.Roles {
			fmt.Fprintf(&b, "  - %s [%s] %s\n",
				nonEmpty(role.RoleID, "(unknown)"),
				nonEmpty(role.Status, "pending"),
				nonEmpty(role.SelectedAgent, "-"),
			)
			if strings.TrimSpace(role.ShortSummary) != "" {
				fmt.Fprintf(&b, "    Summary: %s\n", role.ShortSummary)
			}
			if !role.LastActivityAt.IsZero() {
				fmt.Fprintf(&b, "    Last activity: %s\n", formatStatusTime(role.LastActivityAt))
			}
			if role.TerminalAvailable {
				fmt.Fprintf(&b, "    Recent Output: %s\n", buildTeamTerminalCommandHintFromIDs(view.SessionID, view.TeamID, role.RoleID, "capture", "", 120))
				fmt.Fprintf(&b, "    Open Interactive View: %s\n", buildTeamTerminalCommandHintFromIDs(view.SessionID, view.TeamID, role.RoleID, "attach", "", 0))
			}
			if role.FollowUpAvailable {
				fmt.Fprintf(&b, "    Send Follow-up: %s\n", buildTeamInjectCommandHintFromIDs(view.SessionID, view.TeamID, role.RoleID, ""))
			}
		}
	}

	b.WriteString("\nActivity Timeline:\n")
	if len(view.RecentEvents) == 0 {
		b.WriteString("  (no recent activity)\n")
	} else {
		for _, event := range view.RecentEvents {
			ts := formatStatusTime(event.Timestamp)
			if ts == "-" {
				ts = "(time unknown)"
			}
			if strings.TrimSpace(event.RoleID) != "" {
				fmt.Fprintf(&b, "  - %s | %s | %s\n", ts, event.RoleID, event.Summary)
				continue
			}
			fmt.Fprintf(&b, "  - %s | %s\n", ts, event.Summary)
		}
	}

	b.WriteString("\nArtifacts:\n")
	if len(view.Artifacts) == 0 {
		b.WriteString("  (no runtime artifacts)\n")
	} else {
		for _, artifact := range view.Artifacts {
			label := nonEmpty(artifact.Label, artifact.Kind)
			if strings.TrimSpace(artifact.RoleID) != "" {
				fmt.Fprintf(&b, "  - %s | %s | %s\n", label, artifact.RoleID, artifact.Path)
			} else {
				fmt.Fprintf(&b, "  - %s | %s\n", label, artifact.Path)
			}
		}
	}
	return b.String()
}

func collectTeamArtifacts(entry teamRuntimeStatus) []teamArtifactView {
	artifacts := make([]teamArtifactView, 0, len(entry.Roles)+4)
	seen := make(map[string]struct{})
	add := func(kind, label, path, roleID string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		artifacts = append(artifacts, teamArtifactView{
			Kind:   kind,
			Label:  label,
			Path:   trimmed,
			RoleID: strings.TrimSpace(roleID),
		})
	}

	baseDir := strings.TrimSpace(entry.BaseDir)
	add("timeline", "Team activity log", filepath.Join(baseDir, "events.jsonl"), "")
	for _, role := range entry.Roles {
		add("terminal_log", fmt.Sprintf("%s recent output", role.RoleID), role.RoleLogPath, role.RoleID)
	}

	for _, subdir := range []string{"artifacts", "artifact", "outputs", "output", "reports"} {
		root := filepath.Join(baseDir, subdir)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		baseDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if strings.Count(filepath.Clean(path), string(os.PathSeparator))-baseDepth > defaultArtifactScanDepth {
					return filepath.SkipDir
				}
				return nil
			}
			label := strings.TrimSpace(strings.TrimPrefix(path, baseDir+string(os.PathSeparator)))
			add("artifact", label, path, "")
			return nil
		})
	}

	sort.SliceStable(artifacts, func(i, j int) bool {
		if artifacts[i].Kind != artifacts[j].Kind {
			return artifacts[i].Kind < artifacts[j].Kind
		}
		if artifacts[i].RoleID != artifacts[j].RoleID {
			return artifacts[i].RoleID < artifacts[j].RoleID
		}
		return artifacts[i].Path < artifacts[j].Path
	})
	return artifacts
}

func normalizeRoleViewStatus(raw string, ev map[string]any) string {
	status := utils.TrimLower(raw)
	switch status {
	case "", "initialized":
		status = "pending"
	case "running", "completed", "failed", "blocked", "waiting_input", "waiting-input":
	default:
		status = strings.TrimSpace(raw)
	}
	switch status {
	case "waiting_input", "waiting-input":
		return "blocked"
	}
	eventType := utils.TrimLower(stringValue(ev["type"]))
	switch eventType {
	case "role_failed", "error", "tmux_input_inject_failed", "tmux_pane_bootstrap_failed":
		return "failed"
	case "role_completed":
		return "completed"
	case "role_started", "tool_call", "result":
		if status == "" || status == "pending" {
			return "running"
		}
	case "tmux_unavailable":
		if status == "" || status == "pending" {
			return "blocked"
		}
	}
	if status == "" {
		return "pending"
	}
	return status
}

func normalizeTeamViewStatus(raw string) string {
	status := utils.TrimLower(raw)
	switch status {
	case "", "initialized":
		return "pending"
	case "waiting_input", "waiting-input":
		return "blocked"
	default:
		return strings.TrimSpace(raw)
	}
}

func summarizeRoleState(status string, ev map[string]any) string {
	if summary := summarizeActivityEvent(ev); summary != "" {
		return summary
	}
	switch status {
	case "running":
		return "Running"
	case "blocked":
		return "Waiting for input"
	case "completed":
		return "Completed"
	case "failed":
		return "Failed"
	default:
		return "Pending"
	}
}

func summarizeActivityEvent(ev map[string]any) string {
	if len(ev) == 0 {
		return ""
	}
	switch utils.TrimLower(stringValue(ev["type"])) {
	case "bootstrap_started":
		return "Bootstrap started"
	case "bootstrap_completed":
		return "Bootstrap completed"
	case "tmux_pane_ready":
		return "Interactive view ready"
	case "tmux_unavailable":
		return "Interactive view unavailable"
	case "tmux_pane_bootstrap_failed":
		return prependError("Interactive view setup failed", stringValue(ev["error"]))
	case "role_started":
		return "Role started"
	case "tool_call":
		tool := stringValue(ev["tool_name"])
		summary := stringValue(ev["summary"])
		if summary != "" && tool != "" {
			return fmt.Sprintf("Used %s: %s", tool, summary)
		}
		if tool != "" {
			return fmt.Sprintf("Used %s", tool)
		}
		return "Used a tool"
	case "result":
		if boolValue(ev["is_error"]) {
			return "Produced an error result"
		}
		return "Produced a result"
	case "role_completed":
		return "Role completed"
	case "role_failed":
		return prependError("Role failed", stringValue(ev["error"]))
	case "error":
		return prependError("Runtime error", stringValue(ev["message"]))
	case "tmux_input_injected":
		return "Follow-up sent"
	case "tmux_input_inject_failed":
		return prependError("Follow-up failed", stringValue(ev["error"]))
	default:
		if msg := stringValue(ev["message"]); msg != "" {
			return msg
		}
		return formatRuntimeEventSummary(ev)
	}
}

func prependError(prefix, detail string) string {
	trimmed := strings.TrimSpace(detail)
	if trimmed == "" {
		return prefix
	}
	return prefix + ": " + trimmed
}

func roleViewAgent(role teamruntime.RoleBinding) string {
	if trimmed := strings.TrimSpace(role.SelectedAgentType); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(role.SelectedCLI); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(role.TargetCLI); trimmed != "" {
		return trimmed
	}
	return "agent"
}

func rolePriority(status string) int {
	switch utils.TrimLower(status) {
	case "failed":
		return 0
	case "blocked":
		return 1
	case "running":
		return 2
	case "pending":
		return 3
	case "completed":
		return 4
	default:
		return 5
	}
}

func buildTeamTerminalCommandHint(entry teamRuntimeStatus, roleID, mode, runtimeRoot string, lines int) string {
	return buildTeamTerminalCommandHintFromIDs(entry.SessionID, entry.TeamID, roleID, mode, runtimeRoot, lines)
}

func buildTeamTerminalCommandHintFromIDs(sessionID, teamID, roleID, mode, runtimeRoot string, lines int) string {
	args := []string{"alex", "team", "terminal"}
	if trimmed := strings.TrimSpace(runtimeRoot); trimmed != "" {
		args = append(args, "--runtime-root", trimmed)
	}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		args = append(args, "--session-id", trimmed)
	}
	if trimmed := strings.TrimSpace(teamID); trimmed != "" {
		args = append(args, "--team-id", trimmed)
	}
	if trimmed := strings.TrimSpace(roleID); trimmed != "" {
		args = append(args, "--role-id", trimmed)
	}
	if trimmed := strings.TrimSpace(mode); trimmed != "" {
		args = append(args, "--mode", trimmed)
	}
	if lines > 0 && !strings.EqualFold(strings.TrimSpace(mode), "attach") {
		args = append(args, "--lines", fmt.Sprintf("%d", lines))
	}
	return strings.Join(args, " ")
}

func buildTeamInjectCommandHint(entry teamRuntimeStatus, roleID, runtimeRoot string) string {
	return buildTeamInjectCommandHintFromIDs(entry.SessionID, entry.TeamID, roleID, runtimeRoot)
}

func buildTeamInjectCommandHintFromIDs(sessionID, teamID, roleID, runtimeRoot string) string {
	args := []string{"alex", "team", "inject"}
	if trimmed := strings.TrimSpace(runtimeRoot); trimmed != "" {
		args = append(args, "--runtime-root", trimmed)
	}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		args = append(args, "--session-id", trimmed)
	}
	if trimmed := strings.TrimSpace(teamID); trimmed != "" {
		args = append(args, "--team-id", trimmed)
	}
	if trimmed := strings.TrimSpace(roleID); trimmed != "" {
		args = append(args, "--role-id", trimmed)
	}
	args = append(args, "--message", "\"<follow-up>\"")
	return strings.Join(args, " ")
}
