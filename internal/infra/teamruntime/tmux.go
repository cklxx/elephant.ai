package teamruntime

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

const defaultTmuxSocket = "elephant"

type TmuxManager struct {
	socket string
}

func NewTmuxManager(socket string) *TmuxManager {
	trimmed := strings.TrimSpace(socket)
	if trimmed == "" {
		trimmed = defaultTmuxSocket
	}
	return &TmuxManager{socket: trimmed}
}

func (m *TmuxManager) Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// EnsureRolePanes ensures a team session exists and returns role->pane mapping.
func (m *TmuxManager) EnsureRolePanes(
	ctx context.Context,
	teamID string,
	bindings map[string]RoleBinding,
	recorder *EventRecorder,
) (string, map[string]string, error) {
	if m == nil || !m.Available() {
		return "", nil, fmt.Errorf("tmux not available")
	}
	sessionName := tmuxTeamSessionName(teamID)

	// Reuse existing panes when session already exists.
	if hasTmuxSession(ctx, m.socket, sessionName) {
		panes, err := listTmuxPanes(ctx, m.socket, sessionName)
		if err == nil && len(panes) > 0 {
			roleToPane := assignRolePanes(bindings, panes)
			return sessionName, roleToPane, nil
		}
	}

	if err := runTmux(ctx, m.socket, "new-session", "-d", "-s", sessionName, "bash"); err != nil {
		return "", nil, err
	}

	roleIDs := sortedRoleIDs(bindings)
	roleToPane := make(map[string]string, len(roleIDs))
	if len(roleIDs) == 0 {
		return sessionName, roleToPane, nil
	}

	firstPane, err := tmuxPaneID(ctx, m.socket, sessionName, "0.0")
	if err != nil {
		return "", nil, err
	}
	roleToPane[roleIDs[0]] = firstPane

	for idx := 1; idx < len(roleIDs); idx++ {
		paneID, splitErr := splitTmuxPane(ctx, m.socket, sessionName)
		if splitErr != nil {
			return "", nil, splitErr
		}
		roleToPane[roleIDs[idx]] = paneID
	}
	_ = runTmux(ctx, m.socket, "select-layout", "-t", sessionName, "tiled")

	for _, roleID := range roleIDs {
		pane := roleToPane[roleID]
		b := bindings[roleID]
		cmd := paneBootstrapCommand(roleID, teamID, b)
		if sendErr := runTmux(ctx, m.socket, "send-keys", "-t", pane, cmd, "C-m"); sendErr != nil {
			if recorder != nil {
				_ = recorder.Record("tmux_pane_bootstrap_failed", map[string]any{
					"team_id": teamID,
					"role_id": roleID,
					"pane":    pane,
					"error":   sendErr.Error(),
				})
			}
			continue
		}
		if recorder != nil {
			_ = recorder.Record("tmux_pane_ready", map[string]any{
				"team_id": teamID,
				"role_id": roleID,
				"pane":    pane,
			})
		}
	}
	return sessionName, roleToPane, nil
}

func (m *TmuxManager) Inject(ctx context.Context, paneID string, input string) error {
	if m == nil {
		return fmt.Errorf("tmux manager is nil")
	}
	pane := strings.TrimSpace(paneID)
	if pane == "" {
		return fmt.Errorf("pane_id is required")
	}
	data := strings.TrimSpace(input)
	if data == "" {
		return fmt.Errorf("input is required")
	}
	return runTmux(ctx, m.socket, "send-keys", "-t", pane, data, "C-m")
}

func paneBootstrapCommand(roleID, teamID string, binding RoleBinding) string {
	role := shellSafeEnv(roleID)
	team := shellSafeEnv(teamID)
	target := shellSafeEnv(binding.SelectedCLI)
	profile := shellSafeEnv(binding.CapabilityProfile)
	logPath := strings.TrimSpace(binding.RoleLogPath)
	if logPath == "" {
		return fmt.Sprintf("export ROLE_ID=%s TEAM_ID=%s TARGET_CLI=%s CAP_PROFILE=%s", role, team, target, profile)
	}
	return fmt.Sprintf(
		"export ROLE_ID=%s TEAM_ID=%s TARGET_CLI=%s CAP_PROFILE=%s; mkdir -p %s; touch %s; tail -n +1 -f %s",
		role, team, target, profile,
		shellSafePathDir(logPath), shellSafePath(logPath), shellSafePath(logPath),
	)
}

func shellSafeEnv(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "\"\""
	}
	return strings.ReplaceAll(trimmed, " ", "_")
}

func shellSafePath(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "'\\''") + "'"
}

func shellSafePathDir(raw string) string {
	path := strings.TrimSpace(raw)
	if idx := strings.LastIndex(path, "/"); idx > 0 {
		path = path[:idx]
	}
	if path == "" {
		path = "."
	}
	return shellSafePath(path)
}

func tmuxTeamSessionName(teamID string) string {
	safe := strings.NewReplacer(".", "-", ":", "-", "/", "-", " ", "-").Replace(strings.TrimSpace(teamID))
	if safe == "" {
		safe = "unknown"
	}
	return "elephant-team-" + safe
}

func hasTmuxSession(ctx context.Context, socket, session string) bool {
	return runTmux(ctx, socket, "has-session", "-t", session) == nil
}

func listTmuxPanes(ctx context.Context, socket, session string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "tmux", "-L", socket, "list-panes", "-t", session, "-F", "#{pane_id}").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	outPanes := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			outPanes = append(outPanes, trimmed)
		}
	}
	return outPanes, nil
}

func assignRolePanes(bindings map[string]RoleBinding, panes []string) map[string]string {
	roleIDs := sortedRoleIDs(bindings)
	out := make(map[string]string, len(roleIDs))
	for idx, roleID := range roleIDs {
		if idx >= len(panes) {
			break
		}
		out[roleID] = panes[idx]
	}
	return out
}

func sortedRoleIDs(bindings map[string]RoleBinding) []string {
	out := make([]string, 0, len(bindings))
	for roleID := range bindings {
		out = append(out, roleID)
	}
	sort.Strings(out)
	return out
}

func tmuxPaneID(ctx context.Context, socket, session, paneRef string) (string, error) {
	out, err := exec.CommandContext(ctx, "tmux", "-L", socket, "display-message", "-p", "-t", session+":"+paneRef, "#{pane_id}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func splitTmuxPane(ctx context.Context, socket, session string) (string, error) {
	out, err := exec.CommandContext(ctx, "tmux", "-L", socket, "split-window", "-d", "-t", session, "-P", "-F", "#{pane_id}", "bash").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runTmux(ctx context.Context, socket string, args ...string) error {
	cmd := exec.CommandContext(ctx, "tmux", append([]string{"-L", socket}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}
