package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/infra/coding"
	"alex/internal/infra/teamruntime"
	"gopkg.in/yaml.v3"
)

func TestLoadTeamRuntimeStatus_SortsAndLoads(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_team_runtime")
	now := time.Now().UTC().Truncate(time.Second)
	writeTeamRuntimeFixture(t, root, "session-old", "team-old", now.Add(-2*time.Hour))
	writeTeamRuntimeFixture(t, root, "session-new", "team-new", now)

	statuses, err := loadTeamRuntimeStatus(teamStatusOptions{
		runtimeRoot: root,
		eventsTail:  2,
	})
	if err != nil {
		t.Fatalf("loadTeamRuntimeStatus failed: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].TeamID != "team-new" {
		t.Fatalf("expected newest team first, got %q", statuses[0].TeamID)
	}
	if len(statuses[0].Capabilities) == 0 || statuses[0].Capabilities[0].ID != "codex" {
		t.Fatalf("expected capabilities loaded, got %+v", statuses[0].Capabilities)
	}
	if len(statuses[0].Roles) == 0 || statuses[0].Roles[0].TmuxPane == "" {
		t.Fatalf("expected roles loaded with tmux pane, got %+v", statuses[0].Roles)
	}
	if len(statuses[0].RecentEvents) != 2 {
		t.Fatalf("expected tail=2 events, got %d", len(statuses[0].RecentEvents))
	}
	if got := statuses[0].RuntimeState.Status; got != "completed" {
		t.Fatalf("expected derived team status completed, got %q", got)
	}
	if got := statuses[0].RuntimeState.Roles["executor"].Status; got != "completed" {
		t.Fatalf("expected derived role status completed, got %q", got)
	}
}

func TestLoadTeamRuntimeStatus_FiltersSessionAndTeam(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_team_runtime")
	now := time.Now().UTC().Truncate(time.Second)
	writeTeamRuntimeFixture(t, root, "session-a", "team-a", now)
	writeTeamRuntimeFixture(t, root, "session-b", "team-b", now.Add(-time.Hour))

	statuses, err := loadTeamRuntimeStatus(teamStatusOptions{
		runtimeRoot: root,
		sessionID:   "session-a",
		teamID:      "team-a",
		eventsTail:  1,
	})
	if err != nil {
		t.Fatalf("loadTeamRuntimeStatus failed: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 filtered status, got %d", len(statuses))
	}
	if statuses[0].SessionID != "session-a" || statuses[0].TeamID != "team-a" {
		t.Fatalf("unexpected filtered status: %+v", statuses[0])
	}
}

func TestRunTeamStatus_NoRuntimeFoundReturnsExitCode1(t *testing.T) {
	err := runTeamStatus([]string{"--runtime-root", filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
}

func TestRunTeamStatus_JSONOutput(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_team_runtime")
	writeTeamRuntimeFixture(t, root, "session-json", "team-json", time.Now().UTC().Truncate(time.Second))

	if err := runTeamStatus([]string{"--runtime-root", root, "--json", "--all"}); err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

func TestRunTeamCommand_RejectsUnknownSubcommand(t *testing.T) {
	err := runTeamCommand([]string{"unknown"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
}

func writeTeamRuntimeFixture(t *testing.T, root, sessionID, teamID string, initializedAt time.Time) {
	t.Helper()
	teamDir := filepath.Join(root, sessionID, "teams", teamID)
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}

	bootstrap := teamruntime.BootstrapState{
		SessionID:        sessionID,
		TeamID:           teamID,
		Template:         "claude_analysis",
		Goal:             "fixture goal",
		InitializedAt:    initializedAt,
		CapabilitiesPath: filepath.Join(teamDir, "capabilities.yaml"),
		RoleRegistryPath: filepath.Join(teamDir, "role_registry.yaml"),
		RuntimeStatePath: filepath.Join(teamDir, "runtime_state.yaml"),
		EventLogPath:     filepath.Join(teamDir, "events.jsonl"),
		TmuxSession:      "elephant-team-" + teamID,
	}
	writeYAMLFixture(t, filepath.Join(teamDir, "bootstrap.yaml"), bootstrap)

	capabilities := teamruntime.CapabilitySnapshot{
		GeneratedAt: initializedAt.Add(time.Minute),
		TTLSeconds:  300,
		Capabilities: []coding.DiscoveredCLICapability{
			{
				ID:                 "codex",
				Binary:             "codex",
				Path:               "/usr/local/bin/codex",
				Executable:         true,
				Version:            "codex 1.0.0",
				AgentType:          "codex",
				AdapterSupport:     true,
				SupportsPlan:       true,
				SupportsExecute:    true,
				SupportsStream:     true,
				SupportsFilesystem: true,
				SupportsNetwork:    true,
				AuthReady:          true,
				ProbedAt:           initializedAt.Add(time.Minute),
			},
		},
	}
	writeYAMLFixture(t, filepath.Join(teamDir, "capabilities.yaml"), capabilities)

	roleRegistry := teamruntime.RoleRegistry{
		Roles: []teamruntime.RoleBinding{
			{
				RoleID:            "executor",
				CapabilityProfile: "execution",
				TargetCLI:         "codex",
				SelectedCLI:       "codex",
				SelectedPath:      "/usr/local/bin/codex",
				SelectedAgentType: "codex",
				FallbackCLIs:      []string{"claude_code"},
				TmuxPane:          "%11",
				RoleLogPath:       filepath.Join(teamDir, "logs", "executor.log"),
			},
		},
	}
	writeYAMLFixture(t, filepath.Join(teamDir, "role_registry.yaml"), roleRegistry)

	runtimeState := teamruntime.RuntimeState{
		SessionID: sessionID,
		TeamID:    teamID,
		Status:    "initialized",
		UpdatedAt: initializedAt.Add(2 * time.Minute),
		Roles: map[string]teamruntime.RoleRuntimeState{
			"executor": {
				RoleID:       "executor",
				Status:       "running",
				UpdatedAt:    initializedAt.Add(2 * time.Minute),
				SelectedCLI:  "codex",
				FallbackCLIs: []string{"claude_code"},
			},
		},
	}
	writeYAMLFixture(t, filepath.Join(teamDir, "runtime_state.yaml"), runtimeState)

	eventsPath := filepath.Join(teamDir, "events.jsonl")
	events := []map[string]any{
		{
			"timestamp": initializedAt.Add(3 * time.Minute).Format(time.RFC3339Nano),
			"type":      "tmux_pane_ready",
			"team_id":   teamID,
			"role_id":   "executor",
			"pane":      "%11",
		},
		{
			"timestamp": initializedAt.Add(4 * time.Minute).Format(time.RFC3339Nano),
			"type":      "tool_call",
			"team_id":   teamID,
			"role_id":   "executor",
			"tool_name": "shell_exec",
		},
		{
			"timestamp": initializedAt.Add(5 * time.Minute).Format(time.RFC3339Nano),
			"type":      "role_completed",
			"team_id":   teamID,
			"role_id":   "executor",
		},
	}
	lines := make([]string, 0, len(events))
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(eventsPath, []byte(stringsJoin(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
}

func writeYAMLFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("marshal yaml %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir yaml dir %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write yaml %s: %v", path, err)
	}
}

func stringsJoin(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += sep + items[i]
	}
	return out
}
