package agent_eval

import (
	"testing"

	"alex/evaluation/swe_bench"
	"alex/internal/domain/workflow"
)

func TestAssessSolutionQualityPenalizesWorkflowFailureSignals(t *testing.T) {
	collector := &QualityCollector{}

	baseline := swe_bench.WorkerResult{
		Status:      swe_bench.StatusCompleted,
		Solution:    "Implemented a concrete patch with validation and edge-case notes.",
		Explanation: "Detailed explanation that clearly exceeds one hundred characters so quality heuristics can include rationale and implementation tradeoffs.",
		FilesChanged: []string{
			"core.py",
		},
		Workflow: &workflow.WorkflowSnapshot{
			Phase: workflow.PhaseSucceeded,
		},
	}
	baselineScore := collector.assessSolutionQuality(baseline)
	if baselineScore <= 0.5 {
		t.Fatalf("expected healthy baseline quality score, got %.2f", baselineScore)
	}

	failedWorkflow := baseline
	failedWorkflow.Workflow = &workflow.WorkflowSnapshot{
		Phase: workflow.PhaseFailed,
	}
	failedWorkflowScore := collector.assessSolutionQuality(failedWorkflow)
	if failedWorkflowScore >= baselineScore {
		t.Fatalf("expected workflow-failed score < baseline: failed=%.2f baseline=%.2f", failedWorkflowScore, baselineScore)
	}
	if failedWorkflowScore > 0.3 {
		t.Fatalf("expected strong penalty for failed workflow, got %.2f", failedWorkflowScore)
	}

	maxIterStop := baseline
	maxIterStop.Workflow = &workflow.WorkflowSnapshot{
		Phase: workflow.PhaseSucceeded,
		Nodes: []workflow.NodeSnapshot{
			{
				ID:     "execute",
				Status: workflow.NodeStatusSucceeded,
				Output: map[string]any{"stop": "max_iterations"},
			},
		},
	}
	maxIterScore := collector.assessSolutionQuality(maxIterStop)
	if maxIterScore >= baselineScore {
		t.Fatalf("expected max-iterations score < baseline: max_iter=%.2f baseline=%.2f", maxIterScore, baselineScore)
	}
	if maxIterScore > 0.3 {
		t.Fatalf("expected strong penalty for max_iterations stop, got %.2f", maxIterScore)
	}
}

func TestAssessConsistencyPenalizesWorkflowFailureSignals(t *testing.T) {
	collector := &QualityCollector{}

	baseline := swe_bench.WorkerResult{
		Status:   swe_bench.StatusCompleted,
		Solution: "Compact but complete response body with enough technical detail to exceed the fifty character threshold.",
		FilesChanged: []string{
			"fix.py",
		},
		Workflow: &workflow.WorkflowSnapshot{
			Phase: workflow.PhaseSucceeded,
		},
	}

	baselineConsistency := collector.assessConsistency(baseline)
	if baselineConsistency < 0.7 {
		t.Fatalf("expected healthy baseline consistency, got %.2f", baselineConsistency)
	}

	failed := baseline
	failed.Workflow = &workflow.WorkflowSnapshot{Phase: workflow.PhaseFailed}
	failedConsistency := collector.assessConsistency(failed)
	if failedConsistency >= baselineConsistency {
		t.Fatalf("expected failed consistency < baseline: failed=%.2f baseline=%.2f", failedConsistency, baselineConsistency)
	}
}
