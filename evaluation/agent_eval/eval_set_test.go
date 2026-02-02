package agent_eval

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sampleTasks() []EvalTask {
	return []EvalTask{
		{ID: "t1", Title: "Easy Reasoning", Difficulty: TierEasy, Domain: DomainReasoning, Tags: []string{"basic"}, Weight: 1.0},
		{ID: "t2", Title: "Medium ToolUse", Difficulty: TierMedium, Domain: DomainToolUse, Tags: []string{"multi-step"}, Weight: 2.0},
		{ID: "t3", Title: "Hard CodeGen", Difficulty: TierHard, Domain: DomainCodeGen, Tags: []string{"error-handling", "multi-step"}, Weight: 1.5},
		{ID: "t4", Title: "Expert Planning", Difficulty: TierExpert, Domain: DomainPlanning, Tags: []string{"multi-step"}, Weight: 3.0},
		{ID: "t5", Title: "Easy Retrieval", Difficulty: TierEasy, Domain: DomainRetrieval, Tags: []string{"basic"}, Weight: 1.0},
		{ID: "t6", Title: "Medium Conversation", Difficulty: TierMedium, Domain: DomainConversation, Tags: []string{"error-handling"}, Weight: 0},
	}
}

func mustBuild(t *testing.T, b *EvalSetBuilder) *EvalSet {
	t.Helper()
	set, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected Build error: %v", err)
	}
	return set
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEvalSet_BuilderCreatesSetWithCorrectConfig(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("core-eval", "1.0.0").
			WithDescription("Core evaluation set").
			AddTasks(sampleTasks()),
	)

	if set.Config.Name != "core-eval" {
		t.Fatalf("expected name core-eval, got %s", set.Config.Name)
	}
	if set.Config.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", set.Config.Version)
	}
	if set.Config.Description != "Core evaluation set" {
		t.Fatalf("expected description 'Core evaluation set', got %s", set.Config.Description)
	}
	if set.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestEvalSet_AddTaskAndAddTasks(t *testing.T) {
	single := mustBuild(t,
		NewEvalSetBuilder("single", "0.1.0").
			AddTask(EvalTask{ID: "s1", Difficulty: TierEasy, Domain: DomainReasoning}),
	)
	if len(single.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(single.Tasks))
	}

	multi := mustBuild(t,
		NewEvalSetBuilder("multi", "0.1.0").
			AddTask(EvalTask{ID: "m0", Difficulty: TierEasy, Domain: DomainReasoning}).
			AddTasks(sampleTasks()),
	)
	if len(multi.Tasks) != 1+len(sampleTasks()) {
		t.Fatalf("expected %d tasks, got %d", 1+len(sampleTasks()), len(multi.Tasks))
	}
}

func TestEvalSet_FilterByDifficulty(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("filtered", "0.1.0").
			AddTasks(sampleTasks()).
			FilterByDifficulty(TierEasy),
	)

	for _, task := range set.Tasks {
		if task.Difficulty != TierEasy {
			t.Fatalf("expected all tasks to be TierEasy, got %s for task %s", task.Difficulty, task.ID)
		}
	}
	if len(set.Tasks) != 2 {
		t.Fatalf("expected 2 easy tasks, got %d", len(set.Tasks))
	}
}

func TestEvalSet_FilterByDomain(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("domain-filter", "0.1.0").
			AddTasks(sampleTasks()).
			FilterByDomain(DomainCodeGen),
	)

	for _, task := range set.Tasks {
		if task.Domain != DomainCodeGen {
			t.Fatalf("expected all tasks to be DomainCodeGen, got %s for task %s", task.Domain, task.ID)
		}
	}
	if len(set.Tasks) != 1 {
		t.Fatalf("expected 1 code_gen task, got %d", len(set.Tasks))
	}
}

func TestEvalSet_FilterByTags(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("tag-filter", "0.1.0").
			AddTasks(sampleTasks()).
			FilterByTags([]string{"error-handling"}),
	)

	// Tasks with "error-handling": t3, t6
	if len(set.Tasks) != 2 {
		t.Fatalf("expected 2 tasks with error-handling tag, got %d", len(set.Tasks))
	}
	for _, task := range set.Tasks {
		found := false
		for _, tag := range task.Tags {
			if tag == "error-handling" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("task %s does not have error-handling tag", task.ID)
		}
	}
}

func TestEvalSet_Limit(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("limited", "0.1.0").
			AddTasks(sampleTasks()).
			Limit(3),
	)

	if len(set.Tasks) != 3 {
		t.Fatalf("expected 3 tasks after limit, got %d", len(set.Tasks))
	}
}

func TestEvalSet_BuildWithNoTasksReturnsError(t *testing.T) {
	_, err := NewEvalSetBuilder("empty", "0.1.0").Build()
	if err == nil {
		t.Fatal("expected error when building with no tasks")
	}
}

func TestEvalSet_BuildFilteredToEmptyReturnsError(t *testing.T) {
	_, err := NewEvalSetBuilder("empty-filter", "0.1.0").
		AddTask(EvalTask{ID: "x", Difficulty: TierEasy, Domain: DomainReasoning}).
		FilterByDifficulty(TierExpert).
		Build()
	if err == nil {
		t.Fatal("expected error when all tasks are filtered out")
	}
}

func TestEvalSet_TasksByDifficulty(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("by-diff", "0.1.0").AddTasks(sampleTasks()),
	)

	grouped := set.TasksByDifficulty()

	if len(grouped[TierEasy]) != 2 {
		t.Fatalf("expected 2 easy tasks, got %d", len(grouped[TierEasy]))
	}
	if len(grouped[TierMedium]) != 2 {
		t.Fatalf("expected 2 medium tasks, got %d", len(grouped[TierMedium]))
	}
	if len(grouped[TierHard]) != 1 {
		t.Fatalf("expected 1 hard task, got %d", len(grouped[TierHard]))
	}
	if len(grouped[TierExpert]) != 1 {
		t.Fatalf("expected 1 expert task, got %d", len(grouped[TierExpert]))
	}
}

func TestEvalSet_TasksByDomain(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("by-domain", "0.1.0").AddTasks(sampleTasks()),
	)

	grouped := set.TasksByDomain()

	if len(grouped[DomainReasoning]) != 1 {
		t.Fatalf("expected 1 reasoning task, got %d", len(grouped[DomainReasoning]))
	}
	if len(grouped[DomainToolUse]) != 1 {
		t.Fatalf("expected 1 tool_use task, got %d", len(grouped[DomainToolUse]))
	}
	if len(grouped[DomainCodeGen]) != 1 {
		t.Fatalf("expected 1 code_gen task, got %d", len(grouped[DomainCodeGen]))
	}
	if len(grouped[DomainPlanning]) != 1 {
		t.Fatalf("expected 1 planning task, got %d", len(grouped[DomainPlanning]))
	}
	if len(grouped[DomainRetrieval]) != 1 {
		t.Fatalf("expected 1 retrieval task, got %d", len(grouped[DomainRetrieval]))
	}
	if len(grouped[DomainConversation]) != 1 {
		t.Fatalf("expected 1 conversation task, got %d", len(grouped[DomainConversation]))
	}
}

func TestEvalSet_SummaryComputesCorrectStats(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("summary", "2.0.0").AddTasks(sampleTasks()),
	)

	summary := set.Summary()

	if summary.TotalTasks != 6 {
		t.Fatalf("expected 6 total tasks, got %d", summary.TotalTasks)
	}
	if summary.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", summary.Version)
	}
	if summary.ByDifficulty[TierEasy] != 2 {
		t.Fatalf("expected 2 easy in summary, got %d", summary.ByDifficulty[TierEasy])
	}
	if summary.ByDomain[DomainCodeGen] != 1 {
		t.Fatalf("expected 1 code_gen in summary, got %d", summary.ByDomain[DomainCodeGen])
	}

	// Weights: 1.0 + 2.0 + 1.5 + 3.0 + 1.0 + 1.0 (zero-weight normalized to 1.0) = 9.5
	// Average = 9.5 / 6 = 1.5833...
	expectedAvg := 9.5 / 6.0
	if math.Abs(summary.AvgWeight-expectedAvg) > 0.001 {
		t.Fatalf("expected avg weight ~%.4f, got %.4f", expectedAvg, summary.AvgWeight)
	}
}

func TestEvalSet_ValidateCompositionSatisfied(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("valid", "1.0.0").
			AddTasks(sampleTasks()).
			WithCompositionRule(CompositionRule{
				Difficulty: TierEasy,
				MinCount:   1,
			}).
			WithCompositionRule(CompositionRule{
				Domain:     DomainReasoning,
				Percentage: 0.1,
			}),
	)

	violations := ValidateComposition(set)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestEvalSet_ValidateCompositionMinCountViolation(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("min-fail", "1.0.0").
			AddTasks(sampleTasks()).
			WithCompositionRule(CompositionRule{
				Difficulty: TierExpert,
				MinCount:   5, // only 1 expert task
			}),
	)

	violations := ValidateComposition(set)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
}

func TestEvalSet_ValidateCompositionPercentageViolation(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("pct-fail", "1.0.0").
			AddTasks(sampleTasks()).
			WithCompositionRule(CompositionRule{
				Domain:     DomainPlanning,
				Percentage: 0.5, // only 1/6 = ~16.7%
			}),
	)

	violations := ValidateComposition(set)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
}

func TestEvalSet_WeightDefaultsToOneWhenZero(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("weight-default", "0.1.0").
			AddTask(EvalTask{ID: "w1", Difficulty: TierEasy, Domain: DomainReasoning, Weight: 0}),
	)

	if set.Tasks[0].Weight != 1.0 {
		t.Fatalf("expected weight to default to 1.0, got %f", set.Tasks[0].Weight)
	}
}

func TestEvalSet_MultipleFiltersCompose(t *testing.T) {
	tasks := []EvalTask{
		{ID: "a", Difficulty: TierMedium, Domain: DomainToolUse, Tags: []string{"multi-step"}},
		{ID: "b", Difficulty: TierMedium, Domain: DomainToolUse, Tags: []string{"basic"}},
		{ID: "c", Difficulty: TierMedium, Domain: DomainReasoning, Tags: []string{"multi-step"}},
		{ID: "d", Difficulty: TierHard, Domain: DomainToolUse, Tags: []string{"multi-step"}},
		{ID: "e", Difficulty: TierEasy, Domain: DomainToolUse, Tags: []string{"multi-step"}},
	}

	set := mustBuild(t,
		NewEvalSetBuilder("composed", "0.1.0").
			AddTasks(tasks).
			FilterByDifficulty(TierMedium).
			FilterByDomain(DomainToolUse).
			FilterByTags([]string{"multi-step"}),
	)

	// Only task "a" matches all three filters:
	// Medium + ToolUse + multi-step
	if len(set.Tasks) != 1 {
		t.Fatalf("expected 1 task matching all filters, got %d", len(set.Tasks))
	}
	if set.Tasks[0].ID != "a" {
		t.Fatalf("expected task a, got %s", set.Tasks[0].ID)
	}
}

func TestEvalSet_LimitWithFilter(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("limit-filter", "0.1.0").
			AddTasks(sampleTasks()).
			FilterByTags([]string{"multi-step"}).
			Limit(2),
	)

	// 3 tasks have "multi-step" (t2, t3, t4), limit to 2
	if len(set.Tasks) != 2 {
		t.Fatalf("expected 2 tasks after filter+limit, got %d", len(set.Tasks))
	}
}

func TestEvalSet_ValidateCompositionNilSet(t *testing.T) {
	violations := ValidateComposition(nil)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for nil set, got %d", len(violations))
	}
}

func TestEvalSet_ValidateCompositionMultipleViolations(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("multi-violation", "1.0.0").
			AddTasks(sampleTasks()).
			WithCompositionRule(CompositionRule{
				Difficulty: TierExpert,
				MinCount:   10,
			}).
			WithCompositionRule(CompositionRule{
				Domain:     DomainPlanning,
				Percentage: 0.9,
			}),
	)

	violations := ValidateComposition(set)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(violations), violations)
	}
}

func TestEvalSet_CompositionRuleMatchesWithTags(t *testing.T) {
	set := mustBuild(t,
		NewEvalSetBuilder("tag-rule", "1.0.0").
			AddTasks(sampleTasks()).
			WithCompositionRule(CompositionRule{
				Tags:     []string{"error-handling"},
				MinCount: 2,
			}),
	)

	violations := ValidateComposition(set)
	if len(violations) != 0 {
		t.Fatalf("expected no violations for tag rule with 2 matching tasks, got %v", violations)
	}
}

func TestEvalSet_EmptySummary(t *testing.T) {
	// Build a set with one task, then verify summary works for minimal case.
	set := mustBuild(t,
		NewEvalSetBuilder("minimal", "0.0.1").
			AddTask(EvalTask{ID: "only", Difficulty: TierHard, Domain: DomainRetrieval, Weight: 2.5}),
	)

	summary := set.Summary()
	if summary.TotalTasks != 1 {
		t.Fatalf("expected 1 total task, got %d", summary.TotalTasks)
	}
	if summary.AvgWeight != 2.5 {
		t.Fatalf("expected avg weight 2.5, got %f", summary.AvgWeight)
	}
}
