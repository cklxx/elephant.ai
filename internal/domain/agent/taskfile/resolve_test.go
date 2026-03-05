package taskfile

import (
	"strings"
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
	tf := &TaskFile{
		Version: "1",
		PlanID:  "override-test",
		Tasks: []TaskSpec{
			{
				ID:            "a",
				Prompt:        "do A",
				AgentType:     "codex",
				ExecutionMode: "execute",
				Config: map[string]string{
					"verify":             "false",
					"retry_max_attempts": "5",
					"merge_on_success":   "false",
				},
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

func TestSpecToDispatchRequest_DoesNotOverrideInternalAgentType(t *testing.T) {
	spec := TaskSpec{
		ID:          "t1",
		Description: "internal task",
		Prompt:      "do internal work",
		AgentType:   "internal",
		RuntimeMeta: TeamRuntimeMeta{
			SelectedAgentType: "codex",
		},
	}

	req := SpecToDispatchRequest(spec, "cause-123")

	if req.AgentType != agent.AgentTypeInternal {
		t.Fatalf("AgentType: got %q, want %q", req.AgentType, agent.AgentTypeInternal)
	}
}

func TestContextPreamblePrependedInDispatch(t *testing.T) {
	preamble := "Project: elephant.ai (Go). Key packages: internal/domain/agent."

	// Preamble set on spec directly.
	spec := TaskSpec{
		ID:              "t1",
		Description:     "task",
		Prompt:          "do something",
		ContextPreamble: preamble,
	}
	req := SpecToDispatchRequest(spec, "")
	if !strings.HasPrefix(req.Prompt, preamble) {
		t.Errorf("prompt should start with preamble, got: %q", req.Prompt)
	}
	if !strings.Contains(req.Prompt, "do something") {
		t.Error("original prompt should be present")
	}

	// Empty preamble: prompt unchanged.
	spec2 := TaskSpec{
		ID:          "t2",
		Description: "task",
		Prompt:      "bare prompt",
	}
	req2 := SpecToDispatchRequest(spec2, "")
	if req2.Prompt != "bare prompt" {
		t.Errorf("no preamble: prompt should be unchanged, got %q", req2.Prompt)
	}
}

func TestContextPreambleInheritedFromDefaults(t *testing.T) {
	preamble := "Arch context."
	tf := &TaskFile{
		Version: "1",
		PlanID:  "preamble-test",
		Defaults: TaskDefaults{
			ContextPreamble: preamble,
		},
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "task A"},
			{ID: "b", Prompt: "task B", ContextPreamble: "override preamble"},
		},
	}

	resolved := ResolveDefaults(tf)

	// Task a inherits default preamble.
	if resolved[0].ContextPreamble != preamble {
		t.Errorf("task a: want preamble %q, got %q", preamble, resolved[0].ContextPreamble)
	}
	// Task b keeps its own preamble.
	if resolved[1].ContextPreamble != "override preamble" {
		t.Errorf("task b: want override preamble, got %q", resolved[1].ContextPreamble)
	}
}

func TestMaxBudgetPropagatedToConfig(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "budget-test",
		Tasks: []TaskSpec{
			{
				ID:        "a",
				Prompt:    "do A",
				AgentType: "codex",
				MaxBudget: 2.5,
			},
		},
	}

	resolved := ResolveDefaults(tf)

	if resolved[0].Config["max_budget_usd"] != "2.5" {
		t.Errorf("max_budget_usd: got %q, want %q", resolved[0].Config["max_budget_usd"], "2.5")
	}
}

func TestMaxBudgetInheritedFromDefaults(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "budget-default-test",
		Defaults: TaskDefaults{
			MaxBudget: 5.0,
		},
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A", AgentType: "codex"},
			{ID: "b", Prompt: "do B", AgentType: "codex", MaxBudget: 1.0},
		},
	}

	resolved := ResolveDefaults(tf)

	// Task a inherits default.
	if resolved[0].Config["max_budget_usd"] != "5" {
		t.Errorf("task a: want max_budget_usd=5, got %q", resolved[0].Config["max_budget_usd"])
	}
	// Task b keeps its own budget.
	if resolved[1].Config["max_budget_usd"] != "1" {
		t.Errorf("task b: want max_budget_usd=1, got %q", resolved[1].Config["max_budget_usd"])
	}
}

func TestResolveDefaults_ConfigMergeConflict_TaskWins(t *testing.T) {
	// When both defaults.Config and task.Config have the same key, the
	// task-level value must win.
	tf := &TaskFile{
		Version: "1",
		PlanID:  "config-merge-test",
		Defaults: TaskDefaults{
			AgentType: "codex",
			Config: map[string]string{
				"task_kind":    "coding",
				"env":          "production",
				"log_level":    "info",
				"default_only": "from-defaults",
			},
		},
		Tasks: []TaskSpec{
			{
				ID:     "a",
				Prompt: "do A",
				Config: map[string]string{
					"env":        "staging",    // overrides default "production"
					"log_level":  "debug",      // overrides default "info"
					"task_extra": "task-value", // unique to task
				},
			},
		},
	}

	resolved := ResolveDefaults(tf)

	// Task-level values must override defaults.
	if resolved[0].Config["env"] != "staging" {
		t.Errorf("task config 'env' should be 'staging' (task wins), got %q", resolved[0].Config["env"])
	}
	if resolved[0].Config["log_level"] != "debug" {
		t.Errorf("task config 'log_level' should be 'debug' (task wins), got %q", resolved[0].Config["log_level"])
	}

	// Default-only keys should still be present.
	if resolved[0].Config["default_only"] != "from-defaults" {
		t.Errorf("task config 'default_only' should be inherited from defaults, got %q", resolved[0].Config["default_only"])
	}

	// Task-only keys should be preserved.
	if resolved[0].Config["task_extra"] != "task-value" {
		t.Errorf("task config 'task_extra' should be preserved, got %q", resolved[0].Config["task_extra"])
	}

	// task_kind from defaults should be present (task didn't override it).
	if resolved[0].Config["task_kind"] != "coding" {
		t.Errorf("task config 'task_kind' should be inherited from defaults, got %q", resolved[0].Config["task_kind"])
	}

	// Verify the original TaskFile was not modified.
	if tf.Tasks[0].Config["default_only"] != "" {
		t.Error("original task should not have been modified by ResolveDefaults")
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
		got := agent.CanonicalAgentType(tc.input)
		if got != tc.want {
			t.Errorf("CanonicalAgentType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
