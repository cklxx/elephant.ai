package agent_eval

import (
	"testing"
	"time"

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

func TestAttentionCollectorCollectsProxyMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	results := []swe_bench.WorkerResult{
		{
			TaskID:       "task-1",
			InstanceID:   "inst-1",
			Status:       swe_bench.StatusCompleted,
			Duration:     70 * time.Second,
			RetryCount:   0,
			Explanation:  "Detailed summary with implementation choices and verification notes that should imply moderate confidence.",
			Solution:     "Patched flow and produced validated output with evidence.",
			FilesChanged: []string{"a.go", "b.go"},
			Commands:     []string{"request_user approval for deployment", "bash test.sh"},
		},
		{
			TaskID:       "task-2",
			InstanceID:   "inst-2",
			Status:       swe_bench.StatusFailed,
			Duration:     120 * time.Second,
			RetryCount:   2,
			Error:        "permission denied on write",
			ErrorType:    "permission_error",
			Commands:     []string{"clarify missing permission scope"},
			FilesChanged: []string{"c.go"},
			Workflow: &workflow.WorkflowSnapshot{
				Phase: workflow.PhaseFailed,
			},
		},
	}

	metrics, err := collector.Collect(results)
	if err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	if metrics.Attention.HAMAgentMinutes <= 0 {
		t.Fatalf("expected HAM agent minutes > 0, got %.2f", metrics.Attention.HAMAgentMinutes)
	}
	if metrics.Attention.HAMBaselineMinutes <= metrics.Attention.HAMAgentMinutes {
		t.Fatalf("expected baseline HAM > agent HAM, got baseline=%.2f agent=%.2f", metrics.Attention.HAMBaselineMinutes, metrics.Attention.HAMAgentMinutes)
	}
	if metrics.Attention.AttentionSaving <= 0 {
		t.Fatalf("expected positive attention saving ratio, got %.3f", metrics.Attention.AttentionSaving)
	}
	if metrics.Attention.TotalInterruptions == 0 {
		t.Fatalf("expected interruptions to be counted")
	}
	if metrics.Attention.SevereFailureRate <= 0 {
		t.Fatalf("expected severe failure rate > 0, got %.3f", metrics.Attention.SevereFailureRate)
	}
	if metrics.Attention.TrustCalibrationErr <= 0 {
		t.Fatalf("expected trust calibration error > 0, got %.3f", metrics.Attention.TrustCalibrationErr)
	}
}

func TestAttentionCollectorBoundsTrustCalibrationError(t *testing.T) {
	result := swe_bench.WorkerResult{
		Status:       swe_bench.StatusTimeout,
		Duration:     3 * time.Minute,
		RetryCount:   3,
		Explanation:  "long confident explanation",
		FilesChanged: []string{"a.go", "b.go", "c.go"},
		Error:        "timeout",
	}

	err := estimateTrustCalibrationError(result)
	if err < 0 || err > 1 {
		t.Fatalf("expected trust calibration error in [0,1], got %.4f", err)
	}
}
