package taskfile

import (
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestResolveDefaults_MergesFileDefaults(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "test",
		Defaults: TaskDefaults{
			AgentType:     "codex",
			ExecutionMode: "execute",
			Config:        map[string]string{"task_kind": "coding"},
		},
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", AgentType: "kimi"},
		},
	}

	resolved := ResolveDefaults(tf)

	if resolved[0].AgentType != "codex" {
		t.Errorf("task a agent_type: got %q, want %q", resolved[0].AgentType, "codex")
	}
	if resolved[1].AgentType != "kimi" {
		t.Errorf("task b agent_type: got %q, want %q (should not be overridden)", resolved[1].AgentType, "kimi")
	}
	if resolved[0].ExecutionMode != "execute" {
		t.Errorf("task a execution_mode: got %q, want %q", resolved[0].ExecutionMode, "execute")
	}
}

func TestResolveDefaults_CodingDefaultsApplied(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "coding-test",
		Tasks: []TaskSpec{
			{
				ID:            "a",
				Prompt:        "implement X",
				AgentType:     "codex",
				ExecutionMode: "execute",
			},
		},
	}

	resolved := ResolveDefaults(tf)

	if resolved[0].Config["task_kind"] != "coding" {
		t.Errorf("expected task_kind=coding, got %q", resolved[0].Config["task_kind"])
	}
	if resolved[0].Config["verify"] != "true" {
		t.Errorf("expected verify=true for execute mode, got %q", resolved[0].Config["verify"])
	}
	if resolved[0].Config["merge_on_success"] != "true" {
		t.Errorf("expected merge_on_success=true for execute mode, got %q", resolved[0].Config["merge_on_success"])
	}
	if resolved[0].Config["retry_max_attempts"] != "3" {
		t.Errorf("expected retry_max_attempts=3, got %q", resolved[0].Config["retry_max_attempts"])
	}
	if resolved[0].WorkspaceMode != string(agent.WorkspaceModeWorktree) {
		t.Errorf("expected workspace worktree, got %q", resolved[0].WorkspaceMode)
	}
}

func TestResolveDefaults_PlanMode(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "plan-test",
		Tasks: []TaskSpec{
			{
				ID:            "a",
				Prompt:        "plan X",
				AgentType:     "claude_code",
				ExecutionMode: "plan",
			},
		},
	}

	resolved := ResolveDefaults(tf)

	if resolved[0].Config["verify"] != "false" {
		t.Errorf("expected verify=false for plan mode, got %q", resolved[0].Config["verify"])
	}
	if resolved[0].Config["merge_on_success"] != "false" {
		t.Errorf("expected merge_on_success=false for plan mode, got %q", resolved[0].Config["merge_on_success"])
	}
	if resolved[0].Config["retry_max_attempts"] != "1" {
		t.Errorf("expected retry_max_attempts=1 for plan mode, got %q", resolved[0].Config["retry_max_attempts"])
	}
	if resolved[0].WorkspaceMode != string(agent.WorkspaceModeShared) {
		t.Errorf("expected workspace shared for plan mode, got %q", resolved[0].WorkspaceMode)
	}
}

func TestResolveDefaults_ControlledToFull(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "ctrl-test",
		Tasks: []TaskSpec{
			{
				ID:            "a",
				Prompt:        "do A",
				AgentType:     "codex",
				AutonomyLevel: "controlled",
			},
		},
	}

	resolved := ResolveDefaults(tf)

	if resolved[0].AutonomyLevel != "full" {
		t.Errorf("expected autonomy_level=full, got %q", resolved[0].AutonomyLevel)
	}
}

func TestResolveDefaults_ExplicitOverridesDefaults(t *testing.T) {
	boolFalse := false
	retryMax := 5
	tf := &TaskFile{
		Version: "1",
		PlanID:  "override-test",
		Tasks: []TaskSpec{
			{
				ID:             "a",
				Prompt:         "do A",
				AgentType:      "codex",
				ExecutionMode:  "execute",
				Verify:         &boolFalse,
				RetryMax:       &retryMax,
				MergeOnSuccess: &boolFalse,
			},
		},
	}

	resolved := ResolveDefaults(tf)

	if resolved[0].Config["verify"] != "false" {
		t.Errorf("explicit verify=false should be preserved, got %q", resolved[0].Config["verify"])
	}
	if resolved[0].Config["retry_max_attempts"] != "5" {
		t.Errorf("explicit retry_max=5 should be preserved, got %q", resolved[0].Config["retry_max_attempts"])
	}
	if resolved[0].Config["merge_on_success"] != "false" {
		t.Errorf("explicit merge_on_success=false should be preserved, got %q", resolved[0].Config["merge_on_success"])
	}
}

func TestSpecToDispatchRequest(t *testing.T) {
	spec := TaskSpec{
		ID:            "t1",
		Description:   "test task",
		Prompt:        "do something",
		AgentType:     "claude_code",
		ExecutionMode: "execute",
		AutonomyLevel: "full",
		DependsOn:     []string{"t0"},
		WorkspaceMode: "worktree",
		FileScope:     []string{"internal/"},
		Config:        map[string]string{"task_kind": "coding"},
	}

	req := SpecToDispatchRequest(spec, "cause-123")

	if req.TaskID != "t1" {
		t.Errorf("TaskID: got %q, want %q", req.TaskID, "t1")
	}
	if req.AgentType != "claude_code" {
		t.Errorf("AgentType: got %q, want %q", req.AgentType, "claude_code")
	}
	if req.CausationID != "cause-123" {
		t.Errorf("CausationID: got %q, want %q", req.CausationID, "cause-123")
	}
	if req.WorkspaceMode != agent.WorkspaceModeWorktree {
		t.Errorf("WorkspaceMode: got %q, want %q", req.WorkspaceMode, agent.WorkspaceModeWorktree)
	}
}

func TestCanonicalAgentType(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"codex", "codex"},
		{"Codex", "codex"},
		{"claude_code", "claude_code"},
		{"claude-code", "claude_code"},
		{"Claude Code", "claude_code"},
		{"kimi", "kimi"},
		{"kimi_cli", "kimi"},
		{"k2", "kimi"},
		{"internal", "internal"},
		{"", ""},
		{"custom", "custom"},
	}
	for _, tc := range cases {
		got := canonicalAgentType(tc.input)
		if got != tc.want {
			t.Errorf("canonicalAgentType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
