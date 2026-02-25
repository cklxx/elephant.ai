package taskfile

import (
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestRenderTaskFile_BasicTeam(t *testing.T) {
	tmpl := &TeamTemplate{
		Name:        "test-team",
		Description: "A test team",
		Roles: []TeamTemplateRole{
			{Name: "coder", AgentType: "codex", PromptTemplate: "Implement: {GOAL}"},
			{Name: "reviewer", AgentType: "claude_code", PromptTemplate: "Review work by {TEAM}: {GOAL}", InheritContext: true},
		},
		Stages: []TeamTemplateStage{
			{Name: "execute", Roles: []string{"coder"}},
			{Name: "review", Roles: []string{"reviewer"}},
		},
	}

	tf := RenderTaskFile(tmpl, "build feature X", nil)

	if tf.Version != "1" {
		t.Errorf("version: got %q, want %q", tf.Version, "1")
	}
	if len(tf.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tf.Tasks))
	}

	coder := tf.Tasks[0]
	reviewer := tf.Tasks[1]

	if coder.Prompt != "Implement: build feature X" {
		t.Errorf("coder prompt: got %q", coder.Prompt)
	}
	if len(coder.DependsOn) != 0 {
		t.Errorf("coder should have no deps, got %v", coder.DependsOn)
	}

	if reviewer.Prompt != "Review work by test-team: build feature X" {
		t.Errorf("reviewer prompt: got %q", reviewer.Prompt)
	}
	if len(reviewer.DependsOn) != 1 || reviewer.DependsOn[0] != "team-coder" {
		t.Errorf("reviewer depends_on: got %v, want [team-coder]", reviewer.DependsOn)
	}
	if !reviewer.InheritContext {
		t.Error("reviewer should inherit context")
	}
}

func TestRenderTaskFile_WithOverrides(t *testing.T) {
	tmpl := &TeamTemplate{
		Name: "override-team",
		Roles: []TeamTemplateRole{
			{Name: "worker", AgentType: "codex", PromptTemplate: "Default: {GOAL}"},
		},
		Stages: []TeamTemplateStage{
			{Name: "do", Roles: []string{"worker"}},
		},
	}

	overrides := map[string]string{
		"worker": "Custom prompt for worker",
	}

	tf := RenderTaskFile(tmpl, "goal", overrides)

	if tf.Tasks[0].Prompt != "Custom prompt for worker" {
		t.Errorf("expected override prompt, got %q", tf.Tasks[0].Prompt)
	}
}

func TestBuildStageDeps_ThreeStages(t *testing.T) {
	stages := []TeamTemplateStage{
		{Name: "s1", Roles: []string{"a", "b"}},
		{Name: "s2", Roles: []string{"c"}},
		{Name: "s3", Roles: []string{"d"}},
	}
	taskIDs := map[string]string{
		"a": "t-a", "b": "t-b", "c": "t-c", "d": "t-d",
	}

	deps := buildStageDeps(stages, taskIDs)

	if len(deps["a"]) != 0 || len(deps["b"]) != 0 {
		t.Error("stage 1 roles should have no deps")
	}
	if len(deps["c"]) != 2 {
		t.Errorf("c should depend on 2 tasks, got %v", deps["c"])
	}
	if len(deps["d"]) != 1 || deps["d"][0] != "t-c" {
		t.Errorf("d should depend on [t-c], got %v", deps["d"])
	}
}

func TestTeamTemplateFromDefinition(t *testing.T) {
	def := agent.TeamDefinition{
		Name:        "my-team",
		Description: "desc",
		Roles: []agent.TeamRoleDefinition{
			{Name: "worker", AgentType: "codex", ExecutionMode: "execute"},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "do", Roles: []string{"worker"}},
		},
	}

	tmpl := TeamTemplateFromDefinition(def)

	if tmpl.Name != "my-team" {
		t.Errorf("name: got %q", tmpl.Name)
	}
	if len(tmpl.Roles) != 1 || tmpl.Roles[0].AgentType != "codex" {
		t.Error("roles mismatch")
	}
	if len(tmpl.Stages) != 1 || tmpl.Stages[0].Roles[0] != "worker" {
		t.Error("stages mismatch")
	}
}
