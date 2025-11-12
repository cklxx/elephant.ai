package gate

import (
	"testing"
	"time"
)

func TestEvaluatorRecordsAndSummarisesOutcomes(t *testing.T) {
	evaluator := NewEvaluator(5)

	evaluator.RecordOutcome(Outcome{Mode: ModeRetrieve, Satisfied: true, RetrievedChunks: 3, ExternalCalls: 0, CostUSD: 0.05, Latency: 120 * time.Millisecond})
	evaluator.RecordOutcome(Outcome{Mode: ModeRetrieveSearch, Satisfied: false, FreshnessImproved: true, RetrievedChunks: 6, ExternalCalls: 2, CostUSD: 0.45, Latency: 340 * time.Millisecond})
	evaluator.RecordOutcome(Outcome{Mode: ModeFullLoop, Satisfied: true, FreshnessImproved: true, RetrievedChunks: 8, ExternalCalls: 4, CostUSD: 0.9, Latency: 520 * time.Millisecond})

	summary := evaluator.Snapshot()

	if summary.TotalOutcomes != 3 {
		t.Fatalf("expected 3 outcomes, got %d", summary.TotalOutcomes)
	}
	if summary.RollingWindow != 5 {
		t.Fatalf("expected window 5, got %d", summary.RollingWindow)
	}
	if summary.OverallSatisfaction <= 0.66 || summary.OverallSatisfaction > 0.67 {
		t.Fatalf("unexpected overall satisfaction %.4f", summary.OverallSatisfaction)
	}
	if summary.OverallFreshnessGainRate <= 0.66 || summary.OverallFreshnessGainRate > 0.67 {
		t.Fatalf("unexpected freshness gain rate %.4f", summary.OverallFreshnessGainRate)
	}
	if summary.AverageRetrievedChunks != 17.0/3.0 {
		t.Fatalf("unexpected average chunks %.4f", summary.AverageRetrievedChunks)
	}
	if summary.AverageExternalCalls != 6.0/3.0 {
		t.Fatalf("unexpected average external calls %.4f", summary.AverageExternalCalls)
	}
	if summary.AverageCostUSD <= 0.46 || summary.AverageCostUSD >= 0.47 {
		t.Fatalf("unexpected average cost %.4f", summary.AverageCostUSD)
	}
	if summary.AverageLatency < 326*time.Millisecond || summary.AverageLatency > 327*time.Millisecond {
		t.Fatalf("unexpected average latency %s", summary.AverageLatency)
	}

	retrieve := summary.Modes[ModeRetrieve]
	if retrieve.Count != 1 {
		t.Fatalf("expected 1 retrieve outcome, got %d", retrieve.Count)
	}
	if retrieve.SatisfactionRate != 1 {
		t.Fatalf("expected retrieve satisfaction 1, got %.2f", retrieve.SatisfactionRate)
	}
	if retrieve.AverageLatency != 120*time.Millisecond {
		t.Fatalf("unexpected retrieve latency %s", retrieve.AverageLatency)
	}

	fullLoop := summary.Modes[ModeFullLoop]
	if fullLoop.Count != 1 {
		t.Fatalf("expected 1 full-loop outcome, got %d", fullLoop.Count)
	}
	if fullLoop.FreshnessImprovementRate != 1 {
		t.Fatalf("expected full-loop freshness rate 1, got %.2f", fullLoop.FreshnessImprovementRate)
	}
	if fullLoop.AverageExternalCalls != 4 {
		t.Fatalf("expected full-loop external calls 4, got %.2f", fullLoop.AverageExternalCalls)
	}
}

func TestEvaluatorRespectsWindow(t *testing.T) {
	evaluator := NewEvaluator(3)
	evaluator.RecordOutcome(Outcome{Mode: ModeRetrieve})
	evaluator.RecordOutcome(Outcome{Mode: ModeRetrieveSearch})
	evaluator.RecordOutcome(Outcome{Mode: ModeFullLoop})
	evaluator.RecordOutcome(Outcome{Mode: ModeSkip, Satisfied: true})

	summary := evaluator.Snapshot()
	if summary.TotalOutcomes != 3 {
		t.Fatalf("expected windowed total 3, got %d", summary.TotalOutcomes)
	}
	if _, ok := summary.Modes[ModeRetrieve]; ok {
		t.Fatalf("oldest outcome should have been evicted")
	}
	if summary.Modes[ModeSkip].Count != 1 {
		t.Fatalf("expected latest outcome to remain")
	}
}

func TestEvaluatorReset(t *testing.T) {
	evaluator := NewEvaluator(2)
	evaluator.RecordOutcome(Outcome{Mode: ModeRetrieve, Satisfied: true})
	evaluator.Reset()
	summary := evaluator.Snapshot()
	if summary.TotalOutcomes != 0 {
		t.Fatalf("expected no outcomes after reset, got %d", summary.TotalOutcomes)
	}
	if len(summary.Modes) != 0 {
		t.Fatalf("expected mode map to be empty")
	}
}
