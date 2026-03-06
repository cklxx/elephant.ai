package main

import (
	"os"
	"strings"
	"testing"

	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/teamruntime"
	"gopkg.in/yaml.v3"
)

func TestValidateTeamRunOptions_RequiresExactlyOneInputMode(t *testing.T) {
	if err := validateTeamRunOptions(teamRunOptions{}, nil); err == nil {
		t.Fatal("expected error when no run mode is selected")
	}
	if err := validateTeamRunOptions(teamRunOptions{template: "claude_research", goal: "x", prompt: "y"}, nil); err == nil {
		t.Fatal("expected error when multiple run modes are selected")
	}
}

func TestValidateTeamRunOptions_RequiresGoalExceptTemplateList(t *testing.T) {
	if err := validateTeamRunOptions(teamRunOptions{template: "claude_research"}, nil); err == nil {
		t.Fatal("expected goal validation error")
	}
	if err := validateTeamRunOptions(teamRunOptions{template: "list"}, nil); err != nil {
		t.Fatalf("template=list should be allowed without goal: %v", err)
	}
}

func TestValidateTeamRunOptions_RejectsUnexpectedArgsAndUnsupportedFlagMixes(t *testing.T) {
	if err := validateTeamRunOptions(teamRunOptions{template: "claude_research", goal: "x"}, []string{"extra"}); err == nil {
		t.Fatal("expected unexpected positional args to fail")
	}
	if err := validateTeamRunOptions(teamRunOptions{file: "tasks.yaml", goal: "x"}, nil); err == nil {
		t.Fatal("expected --goal without template to fail")
	}
	if err := validateTeamRunOptions(teamRunOptions{prompt: "ship it", rolePrompts: map[string]string{"reviewer": "focus"}}, nil); err == nil {
		t.Fatal("expected --role-prompt without template to fail")
	}
	if err := validateTeamRunOptions(teamRunOptions{template: "list", goal: "x"}, nil); err == nil {
		t.Fatal("expected --template list with goal to fail")
	}
}

func TestWriteSinglePromptTaskFile_CreatesTaskSpec(t *testing.T) {
	path, err := writeSinglePromptTaskFile(teamRunOptions{
		prompt:        "hello team",
		agentType:     "codex",
		executionMode: "execute",
		autonomyLevel: "full",
		workspaceMode: "worktree",
	})
	if err != nil {
		t.Fatalf("writeSinglePromptTaskFile failed: %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp taskfile failed: %v", err)
	}

	var tf taskfile.TaskFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		t.Fatalf("unmarshal taskfile failed: %v", err)
	}
	if len(tf.Tasks) != 1 {
		t.Fatalf("expected one generated task, got %d", len(tf.Tasks))
	}
	task := tf.Tasks[0]
	if task.Prompt != "hello team" {
		t.Fatalf("unexpected prompt: %q", task.Prompt)
	}
	if task.AgentType != "codex" {
		t.Fatalf("unexpected agent type: %q", task.AgentType)
	}
	if task.ExecutionMode != "execute" || task.AutonomyLevel != "full" || task.WorkspaceMode != "worktree" {
		t.Fatalf("unexpected execution fields: %+v", task)
	}
}

func TestParseRolePrompts(t *testing.T) {
	prompts, err := parseRolePrompts([]string{"analyst=focus on correctness", "reviewer=focus on risk"})
	if err != nil {
		t.Fatalf("parseRolePrompts failed: %v", err)
	}
	if prompts["analyst"] != "focus on correctness" {
		t.Fatalf("unexpected analyst prompt: %q", prompts["analyst"])
	}
	if prompts["reviewer"] != "focus on risk" {
		t.Fatalf("unexpected reviewer prompt: %q", prompts["reviewer"])
	}

	if _, err := parseRolePrompts([]string{"invalid"}); err == nil {
		t.Fatal("expected invalid prompt format to fail")
	}
}

func TestResolveInjectRole(t *testing.T) {
	entry := teamRuntimeStatus{
		TeamID: "team-a",
		Roles: []teamruntime.RoleBinding{
			{RoleID: "analyst_a", TmuxPane: "%1"},
			{RoleID: "analyst_b", TmuxPane: "%2"},
		},
	}

	role, err := resolveInjectRole(entry, "")
	if err != nil {
		t.Fatalf("resolve default role failed: %v", err)
	}
	if role.RoleID != "analyst_a" {
		t.Fatalf("expected first role fallback, got %q", role.RoleID)
	}

	role, err = resolveInjectRole(entry, "analyst_b")
	if err != nil {
		t.Fatalf("resolve explicit role failed: %v", err)
	}
	if role.TmuxPane != "%2" {
		t.Fatalf("unexpected pane: %q", role.TmuxPane)
	}

	if _, err := resolveInjectRole(entry, "missing"); err == nil {
		t.Fatal("expected missing role to fail")
	}
}

func TestParseTeamTerminalMode(t *testing.T) {
	cases := map[string]string{
		"":         "stream",
		"stream":   "stream",
		" STREAM ": "stream",
		"attach":   "attach",
		"capture":  "capture",
		"bad":      "",
	}
	for input, want := range cases {
		if got := parseTeamTerminalMode(input); got != want {
			t.Fatalf("parseTeamTerminalMode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestRenderTeamRunCLIOutput_IncludesSessionID(t *testing.T) {
	output := renderTeamRunCLIOutput("completed", "session-123")
	if !strings.Contains(output, "completed") {
		t.Fatalf("expected content in output, got %q", output)
	}
	if !strings.Contains(output, "Session ID: session-123") {
		t.Fatalf("expected session id in output, got %q", output)
	}
}

func TestResolveRequestedRoleID(t *testing.T) {
	roleID, err := resolveRequestedRoleID(teamInjectOptions{roleID: "reviewer"})
	if err != nil {
		t.Fatalf("resolveRequestedRoleID explicit role failed: %v", err)
	}
	if roleID != "reviewer" {
		t.Fatalf("expected reviewer, got %q", roleID)
	}

	roleID, err = resolveRequestedRoleID(teamInjectOptions{taskID: "team-analyst"})
	if err != nil {
		t.Fatalf("resolveRequestedRoleID task id failed: %v", err)
	}
	if roleID != "analyst" {
		t.Fatalf("expected analyst, got %q", roleID)
	}

	if _, err := resolveRequestedRoleID(teamInjectOptions{taskID: "plain-task"}); err == nil {
		t.Fatal("expected non-team task id to fail")
	}
}

func TestSelectTeamRuntimeStatus(t *testing.T) {
	statuses := []teamRuntimeStatus{
		{
			SessionID: "session-a",
			TeamID:    "team-a",
			Roles: []teamruntime.RoleBinding{
				{RoleID: "planner"},
			},
		},
		{
			SessionID: "session-b",
			TeamID:    "team-b",
			Roles: []teamruntime.RoleBinding{
				{RoleID: "planner"},
			},
		},
	}

	if _, err := selectTeamRuntimeStatus(statuses, teamInjectOptions{}, ""); err == nil {
		t.Fatal("expected multiple statuses without filters to fail")
	}

	if _, err := selectTeamRuntimeStatus(statuses, teamInjectOptions{}, "planner"); err == nil {
		t.Fatal("expected ambiguous role match to fail")
	}

	selected, err := selectTeamRuntimeStatus(statuses[:1], teamInjectOptions{}, "planner")
	if err != nil {
		t.Fatalf("expected unique match, got %v", err)
	}
	if selected.TeamID != "team-a" {
		t.Fatalf("expected team-a, got %q", selected.TeamID)
	}
}

func TestRunTeamCommandWithContainer_HelpFlag(t *testing.T) {
	if err := runTeamCommandWithContainer([]string{"--help"}, nil); err != nil {
		t.Fatalf("expected --help to succeed, got %v", err)
	}
}
