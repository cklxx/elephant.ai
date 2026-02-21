package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

var testTeam = agent.TeamDefinition{
	Name:        "execute_and_report",
	Description: "Codex executes, Claude reports",
	Roles: []agent.TeamRoleDefinition{
		{
			Name:           "executor",
			AgentType:      "codex",
			PromptTemplate: "[Executor] Goal: {GOAL}",
			ExecutionMode:  "execute",
			AutonomyLevel:  "full",
			WorkspaceMode:  "worktree",
			Config:         map[string]string{"task_kind": "coding"},
		},
		{
			Name:           "reporter",
			AgentType:      "claude_code",
			PromptTemplate: "[Reporter] Summarize: {GOAL} (Team: {TEAM}, Role: {ROLE})",
			ExecutionMode:  "execute",
			AutonomyLevel:  "full",
			WorkspaceMode:  "shared",
			InheritContext: true,
		},
	},
	Stages: []agent.TeamStageDefinition{
		{Name: "execution", Roles: []string{"executor"}},
		{Name: "reporting", Roles: []string{"reporter"}},
	},
}

func TestTeamDispatch_Success(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-team-1",
		Arguments: map[string]any{
			"team": "execute_and_report",
			"goal": "implement feature X",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	// Should dispatch exactly 2 tasks (executor + reporter).
	if len(d.dispatched) != 2 {
		t.Fatalf("expected 2 dispatched tasks, got %d", len(d.dispatched))
	}

	exec := d.dispatched[0].Req
	report := d.dispatched[1].Req

	// Verify executor task.
	if !strings.HasPrefix(exec.TaskID, "team-executor-") {
		t.Errorf("expected executor task_id prefix 'team-executor-', got %s", exec.TaskID)
	}
	if exec.AgentType != "codex" {
		t.Errorf("expected executor agent_type=codex, got %s", exec.AgentType)
	}
	if exec.AutonomyLevel != "full" {
		t.Errorf("expected executor autonomy=full, got %s", exec.AutonomyLevel)
	}
	if len(exec.DependsOn) != 0 {
		t.Errorf("expected executor to have no dependencies, got %v", exec.DependsOn)
	}
	if exec.WorkspaceMode != agent.WorkspaceModeWorktree {
		t.Errorf("expected executor workspace=worktree, got %s", exec.WorkspaceMode)
	}

	// Verify prompt template rendering.
	if exec.Prompt != "[Executor] Goal: implement feature X" {
		t.Errorf("expected rendered prompt, got %q", exec.Prompt)
	}

	// Verify reporter task.
	if !strings.HasPrefix(report.TaskID, "team-reporter-") {
		t.Errorf("expected reporter task_id prefix 'team-reporter-', got %s", report.TaskID)
	}
	if report.AgentType != "claude_code" {
		t.Errorf("expected reporter agent_type=claude_code, got %s", report.AgentType)
	}
	if !report.InheritContext {
		t.Error("expected reporter inherit_context=true")
	}
	if report.WorkspaceMode != agent.WorkspaceModeShared {
		t.Errorf("expected reporter workspace=shared, got %s", report.WorkspaceMode)
	}

	// Reporter should depend on executor.
	if len(report.DependsOn) != 1 || report.DependsOn[0] != exec.TaskID {
		t.Errorf("expected reporter to depend on executor %s, got %v", exec.TaskID, report.DependsOn)
	}

	// Verify reporter prompt rendering (all variables).
	if !strings.Contains(report.Prompt, "implement feature X") {
		t.Errorf("expected reporter prompt to contain goal, got %q", report.Prompt)
	}
	if !strings.Contains(report.Prompt, "execute_and_report") {
		t.Errorf("expected reporter prompt to contain team name, got %q", report.Prompt)
	}
	if !strings.Contains(report.Prompt, "reporter") {
		t.Errorf("expected reporter prompt to contain role name, got %q", report.Prompt)
	}

	// Verify causation ID.
	if exec.CausationID != "call-team-1" {
		t.Errorf("expected causation_id=call-team-1, got %s", exec.CausationID)
	}

	// Verify metadata.
	if result.Metadata == nil {
		t.Fatal("expected metadata")
	}
	if result.Metadata["team"] != "execute_and_report" {
		t.Errorf("expected metadata team, got %v", result.Metadata["team"])
	}
}

func TestTeamDispatch_ListTeams(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-list",
		Arguments: map[string]any{
			"team": "list",
			"goal": "ignored",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "execute_and_report") {
		t.Errorf("expected team name in list output, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "executor(codex)") {
		t.Errorf("expected role details in list output, got %q", result.Content)
	}
	// Should not dispatch anything.
	if len(d.dispatched) != 0 {
		t.Errorf("expected 0 dispatches for list, got %d", len(d.dispatched))
	}
}

func TestTeamDispatch_ListTeamsEmpty(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-list-empty",
		Arguments: map[string]any{
			"team": "list",
			"goal": "ignored",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No teams configured") {
		t.Errorf("expected empty teams message, got %q", result.Content)
	}
}

func TestTeamDispatch_UnknownTeam(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-unknown",
		Arguments: map[string]any{
			"team": "nonexistent",
			"goal": "some goal",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unknown team")
	}
	if !strings.Contains(result.Content, "unknown team") {
		t.Errorf("expected 'unknown team' message, got %q", result.Content)
	}
}

func TestTeamDispatch_NoDispatcher(t *testing.T) {
	// Context with team definitions but no BackgroundTaskDispatcher.
	ctx := agent.WithTeamDefinitions(context.Background(), []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-no-dispatch",
		Arguments: map[string]any{
			"team": "execute_and_report",
			"goal": "some goal",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when no dispatcher")
	}
	if !strings.Contains(result.Content, "not available") {
		t.Errorf("expected 'not available' message, got %q", result.Content)
	}
}

func TestTeamDispatch_PromptOverride(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-override",
		Arguments: map[string]any{
			"team": "execute_and_report",
			"goal": "feature Y",
			"prompts": map[string]any{
				"executor": "Custom executor prompt for feature Y",
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 2 {
		t.Fatalf("expected 2 dispatched tasks, got %d", len(d.dispatched))
	}

	// Executor should use the override prompt.
	if d.dispatched[0].Req.Prompt != "Custom executor prompt for feature Y" {
		t.Errorf("expected overridden prompt, got %q", d.dispatched[0].Req.Prompt)
	}
	// Reporter should still use template rendering.
	if !strings.Contains(d.dispatched[1].Req.Prompt, "feature Y") {
		t.Errorf("expected reporter prompt to use template with goal, got %q", d.dispatched[1].Req.Prompt)
	}
}

func TestTeamDispatch_DispatchError(t *testing.T) {
	d := &mockDispatcher{dispatchErr: fmt.Errorf("executor busy")}
	ctx := ctxWithDispatcher(d)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-fail",
		Arguments: map[string]any{
			"team": "execute_and_report",
			"goal": "fail task",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error on dispatch failure")
	}
	if !strings.Contains(result.Content, "executor busy") {
		t.Errorf("expected dispatch error message, got %q", result.Content)
	}
}

func TestTeamDispatch_ConfigOverride(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{testTeam})
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-cfg",
		Arguments: map[string]any{
			"team": "execute_and_report",
			"goal": "config test",
			"config": map[string]any{
				"custom_key": "custom_value",
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	// Both tasks should have the global config override.
	for i, call := range d.dispatched {
		if call.Req.Config["custom_key"] != "custom_value" {
			t.Errorf("task %d: expected custom_key=custom_value in config, got %v", i, call.Req.Config)
		}
	}

	// Executor's role config should also be present.
	if d.dispatched[0].Req.Config["task_kind"] != "coding" {
		t.Errorf("expected executor role config task_kind=coding, got %v", d.dispatched[0].Req.Config)
	}
}

func TestTeamDispatch_UnsupportedParam(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewTeamDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-bad-param",
		Arguments: map[string]any{
			"team":    "execute_and_report",
			"goal":    "test",
			"unknown": "bad",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported parameter")
	}
}

func TestValidateTeam_EmptyRoles(t *testing.T) {
	team := agent.TeamDefinition{
		Name:   "empty",
		Stages: []agent.TeamStageDefinition{{Name: "s1", Roles: []string{"r1"}}},
	}
	if err := validateTeam(&team); err == nil {
		t.Fatal("expected error for empty roles")
	}
}

func TestValidateTeam_EmptyStages(t *testing.T) {
	team := agent.TeamDefinition{
		Name:  "no-stages",
		Roles: []agent.TeamRoleDefinition{{Name: "r1", AgentType: "codex"}},
	}
	if err := validateTeam(&team); err == nil {
		t.Fatal("expected error for empty stages")
	}
}

func TestValidateTeam_UnknownRoleInStage(t *testing.T) {
	team := agent.TeamDefinition{
		Name:   "bad-ref",
		Roles:  []agent.TeamRoleDefinition{{Name: "r1", AgentType: "codex"}},
		Stages: []agent.TeamStageDefinition{{Name: "s1", Roles: []string{"r1", "r2"}}},
	}
	if err := validateTeam(&team); err == nil {
		t.Fatal("expected error for unknown role in stage")
	}
}

func TestBuildStageDeps(t *testing.T) {
	roleTaskIDs := map[string]string{
		"a": "task-a",
		"b": "task-b",
		"c": "task-c",
	}
	stages := []agent.TeamStageDefinition{
		{Name: "s1", Roles: []string{"a", "b"}},
		{Name: "s2", Roles: []string{"c"}},
	}
	deps := buildStageDeps(stages, roleTaskIDs)

	// Stage 1 roles have no deps.
	if len(deps["a"]) != 0 {
		t.Errorf("expected no deps for 'a', got %v", deps["a"])
	}
	if len(deps["b"]) != 0 {
		t.Errorf("expected no deps for 'b', got %v", deps["b"])
	}

	// Stage 2 role depends on both stage 1 task IDs.
	if len(deps["c"]) != 2 {
		t.Fatalf("expected 2 deps for 'c', got %v", deps["c"])
	}
	depSet := map[string]bool{}
	for _, d := range deps["c"] {
		depSet[d] = true
	}
	if !depSet["task-a"] || !depSet["task-b"] {
		t.Errorf("expected deps on task-a and task-b, got %v", deps["c"])
	}
}

func TestRenderTeamPrompt(t *testing.T) {
	tests := []struct {
		name     string
		template string
		override string
		goal     string
		expected string
	}{
		{
			name:     "basic template",
			template: "Do {GOAL}",
			goal:     "feature X",
			expected: "Do feature X",
		},
		{
			name:     "all variables",
			template: "{ROLE} in {TEAM}: {GOAL}",
			goal:     "test",
			expected: "myRole in myTeam: test",
		},
		{
			name:     "override wins",
			template: "template ignored",
			override: "custom prompt",
			goal:     "test",
			expected: "custom prompt",
		},
		{
			name:     "empty template uses goal",
			template: "",
			goal:     "fallback goal",
			expected: "fallback goal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overrides := map[string]string{}
			if tt.override != "" {
				overrides["myRole"] = tt.override
			}
			result := renderTeamPrompt(tt.template, overrides, "myRole", "myTeam", tt.goal)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTruncateGoal(t *testing.T) {
	if truncateGoal("short", 100) != "short" {
		t.Error("short goal should not be truncated")
	}
	long := strings.Repeat("x", 100)
	truncated := truncateGoal(long, 20)
	if len(truncated) != 20 {
		t.Errorf("expected length 20, got %d", len(truncated))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Error("expected ... suffix")
	}
}
