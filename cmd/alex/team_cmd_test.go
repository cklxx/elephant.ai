package main

import (
	"os"
	"testing"

	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/teamruntime"
	"gopkg.in/yaml.v3"
)

func TestValidateTeamRunOptions_RequiresExactlyOneInputMode(t *testing.T) {
	if err := validateTeamRunOptions(teamRunOptions{}); err == nil {
		t.Fatal("expected error when no run mode is selected")
	}
	if err := validateTeamRunOptions(teamRunOptions{template: "claude_research", goal: "x", prompt: "y"}); err == nil {
		t.Fatal("expected error when multiple run modes are selected")
	}
}

func TestValidateTeamRunOptions_RequiresGoalExceptTemplateList(t *testing.T) {
	if err := validateTeamRunOptions(teamRunOptions{template: "claude_research"}); err == nil {
		t.Fatal("expected goal validation error")
	}
	if err := validateTeamRunOptions(teamRunOptions{template: "list"}); err != nil {
		t.Fatalf("template=list should be allowed without goal: %v", err)
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
