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

	stageOutputIDs := computeStageOutputIDs(stages, taskIDs)
	deps := buildStageDeps(stages, stageOutputIDs)

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

func TestRenderTaskFile_DebateMode(t *testing.T) {
	tmpl := &TeamTemplate{
		Name: "debate-team",
		Roles: []TeamTemplateRole{
			{Name: "analyst", AgentType: "codex"},
			{Name: "reviewer", AgentType: "claude_code", InheritContext: true},
		},
		Stages: []TeamTemplateStage{
			{Name: "analyze", Roles: []string{"analyst"}, DebateMode: true},
			{Name: "review", Roles: []string{"reviewer"}},
		},
	}

	tf := RenderTaskFile(tmpl, "evaluate proposal X", nil)

	// Expect 3 tasks: analyst (primary), analyst-debate (challenger), reviewer.
	if len(tf.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d: %v", len(tf.Tasks), taskIDs(tf))
	}

	byID := make(map[string]TaskSpec)
	for _, spec := range tf.Tasks {
		byID[spec.ID] = spec
	}

	// Challenger task exists with correct metadata.
	debater, ok := byID["team-analyst-debate"]
	if !ok {
		t.Fatalf("expected team-analyst-debate task, got: %v", taskIDs(tf))
	}
	if !debater.InheritContext {
		t.Error("debate challenger should have InheritContext=true")
	}
	if len(debater.DependsOn) != 1 || debater.DependsOn[0] != "team-analyst" {
		t.Errorf("debate challenger DependsOn: got %v, want [team-analyst]", debater.DependsOn)
	}

	// Reviewer (next stage) depends on BOTH primary and debate IDs.
	reviewer, ok := byID["team-reviewer"]
	if !ok {
		t.Fatal("expected team-reviewer task")
	}
	wantDeps := map[string]bool{"team-analyst": true, "team-analyst-debate": true}
	if len(reviewer.DependsOn) != 2 {
		t.Fatalf("reviewer should depend on 2 tasks, got %v", reviewer.DependsOn)
	}
	for _, dep := range reviewer.DependsOn {
		if !wantDeps[dep] {
			t.Errorf("unexpected dep %q for reviewer", dep)
		}
	}
}

func TestComputeStageOutputIDs_DebateMode(t *testing.T) {
	stages := []TeamTemplateStage{
		{Name: "s1", Roles: []string{"a"}, DebateMode: true},
		{Name: "s2", Roles: []string{"b"}},
	}
	taskIDs := map[string]string{"a": "t-a", "b": "t-b"}

	out := computeStageOutputIDs(stages, taskIDs)

	// Stage 0 with debate should include primary + debate IDs.
	if len(out[0]) != 2 {
		t.Fatalf("stage 0 output IDs: want 2, got %v", out[0])
	}
	wantS0 := map[string]bool{"t-a": true, "t-a-debate": true}
	for _, id := range out[0] {
		if !wantS0[id] {
			t.Errorf("unexpected stage 0 output ID: %q", id)
		}
	}

	// Stage 1 without debate should include only primary.
	if len(out[1]) != 1 || out[1][0] != "t-b" {
		t.Errorf("stage 1 output IDs: want [t-b], got %v", out[1])
	}
}

func TestTeamTemplateFromDefinition_DebateMode(t *testing.T) {
	def := agent.TeamDefinition{
		Name: "debate-team",
		Roles: []agent.TeamRoleDefinition{
			{Name: "worker", AgentType: "codex"},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "do", Roles: []string{"worker"}, DebateMode: true},
		},
	}

	tmpl := TeamTemplateFromDefinition(def)

	if len(tmpl.Stages) != 1 || !tmpl.Stages[0].DebateMode {
		t.Error("DebateMode should be propagated from TeamStageDefinition")
	}
}

// taskIDs returns all task IDs from a TaskFile for debugging.
func taskIDs(tf *TaskFile) []string {
	var ids []string
	for _, t := range tf.Tasks {
		ids = append(ids, t.ID)
	}
	return ids
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
