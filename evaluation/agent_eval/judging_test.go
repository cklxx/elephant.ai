package agent_eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/evaluation/swe_bench"
)

type fixedJudge struct {
	result AgentJudgement
}

func (f fixedJudge) Judge(_ context.Context, _ EvalTask, _ swe_bench.WorkerResult, _ JudgeRubric) (AgentJudgement, error) {
	return f.result, nil
}

func TestAutoJudgeTaskScoresCompletionFormatConstraints(t *testing.T) {
	rubric := JudgeRubric{
		Name:          "baseline",
		PassThreshold: 0.7,
		FailOnZero:    []string{"completion", "format", "constraints"},
		Dimensions: []JudgeDimension{
			{ID: "completion", Weight: 0.2, Auto: true},
			{ID: "format", Weight: 0.3, Auto: true},
			{ID: "constraints", Weight: 0.5, Auto: true},
		},
	}

	task := EvalTask{
		ID:           "t1",
		Title:        "Plan",
		Goal:         "Plan work",
		Difficulty:   TierMedium,
		Domain:       DomainPlanning,
		PassCriteria: []string{"List owners", "Include risks", "expected_output: A markdown table with owners and risks."},
	}

	result := swe_bench.WorkerResult{
		TaskID:   "t1",
		Status:   swe_bench.StatusCompleted,
		Solution: "| week | owner | risk |\n|---|---|---|\n| w1 | Nora | latency |",
	}

	auto := AutoJudgeTask(task, result, rubric)
	if auto.Status != JudgementStatusPassed {
		t.Fatalf("expected auto status passed, got %s", auto.Status)
	}
	if auto.Score < rubric.PassThreshold {
		t.Fatalf("expected auto score >= threshold, got %.2f", auto.Score)
	}
	if len(auto.Dimensions) != 3 {
		t.Fatalf("expected 3 auto dimensions, got %d", len(auto.Dimensions))
	}
}

func TestRunJudgementPipelineNeedsAgentWhenRubricHasAgentDims(t *testing.T) {
	rubric := JudgeRubric{
		Name:          "baseline",
		PassThreshold: 0.7,
		FailOnZero:    []string{"completion"},
		Dimensions: []JudgeDimension{
			{ID: "completion", Weight: 0.4, Auto: true},
			{ID: "format", Weight: 0.2, Auto: true},
			{ID: "correctness", Weight: 0.4, Auto: false},
		},
	}

	set := &EvalSet{
		Config: EvalSetConfig{Name: "baseline", Version: "1.0.0"},
		Tasks: []EvalTask{
			{ID: "t1", PassCriteria: []string{"expected_output: list"}, Difficulty: TierMedium, Domain: DomainReasoning},
		},
	}

	results := []swe_bench.WorkerResult{
		{TaskID: "t1", Status: swe_bench.StatusCompleted, Solution: "- item"},
	}

	summary, judged, err := RunJudgementPipeline(context.Background(), set, results, rubric, NoopAgentJudge{})
	if err != nil {
		t.Fatalf("RunJudgementPipeline: %v", err)
	}
	if summary.Status != JudgementStatusNeedsAgent {
		t.Fatalf("expected summary status needs_agent, got %s", summary.Status)
	}
	if judged[0].Final.Status != JudgementStatusNeedsAgent {
		t.Fatalf("expected final status needs_agent, got %s", judged[0].Final.Status)
	}
}

func TestRunJudgementPipelineCombinesAgentScores(t *testing.T) {
	rubric := JudgeRubric{
		Name:          "baseline",
		PassThreshold: 0.7,
		Dimensions: []JudgeDimension{
			{ID: "completion", Weight: 0.5, Auto: true},
			{ID: "correctness", Weight: 0.5, Auto: false},
		},
	}

	set := &EvalSet{
		Config: EvalSetConfig{Name: "baseline", Version: "1.0.0"},
		Tasks: []EvalTask{
			{ID: "t1", PassCriteria: []string{"expected_output: list"}, Difficulty: TierMedium, Domain: DomainReasoning},
		},
	}

	results := []swe_bench.WorkerResult{
		{TaskID: "t1", Status: swe_bench.StatusCompleted, Solution: "- item"},
	}

	agent := fixedJudge{result: AgentJudgement{Status: JudgementStatusPassed, Score: 1.0}}
	summary, judged, err := RunJudgementPipeline(context.Background(), set, results, rubric, agent)
	if err != nil {
		t.Fatalf("RunJudgementPipeline: %v", err)
	}
	if summary.Status != JudgementStatusPassed {
		t.Fatalf("expected summary status passed, got %s", summary.Status)
	}
	if judged[0].Final.Status != JudgementStatusPassed {
		t.Fatalf("expected final status passed, got %s", judged[0].Final.Status)
	}
	if judged[0].Final.Score < rubric.PassThreshold {
		t.Fatalf("expected final score >= threshold, got %.2f", judged[0].Final.Score)
	}
}

func TestLoadJudgeRubric(t *testing.T) {
	dir := t.TempDir()
	rubricPath := filepath.Join(dir, "rubric.yaml")

	content := `version: "1.0"
name: "baseline"
pass_threshold: 0.7
dimensions:
  - id: completion
    name: Completion
    weight: 0.5
    auto: true
  - id: correctness
    name: Correctness
    weight: 0.5
    auto: false
`
	if err := os.WriteFile(rubricPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write rubric: %v", err)
	}

	rubric, err := LoadJudgeRubric(rubricPath)
	if err != nil {
		t.Fatalf("LoadJudgeRubric: %v", err)
	}
	if rubric.Name != "baseline" {
		t.Fatalf("expected name baseline, got %s", rubric.Name)
	}
	if len(rubric.Dimensions) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(rubric.Dimensions))
	}
}
