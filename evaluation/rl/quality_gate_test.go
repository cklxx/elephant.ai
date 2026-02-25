package rl

import (
	"context"
	"fmt"
	"testing"
)

type mockJudge struct {
	score float64
	err   error
}

func (m *mockJudge) Score(_ context.Context, _ *RLTrajectory) (float64, error) {
	return m.score, m.err
}

func TestQualityGate_ClassifyAutoOnly(t *testing.T) {
	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = false
	gate := NewQualityGate(cfg, nil)
	ctx := context.Background()

	tests := []struct {
		score float64
		want  QualityTier
	}{
		{95, TierGold},
		{80, TierGold},
		{79.9, TierSilver},
		{60, TierSilver},
		{59.9, TierBronze},
		{40, TierBronze},
		{39.9, TierReject},
		{0, TierReject},
	}

	for _, tt := range tests {
		traj := &RLTrajectory{AutoScore: tt.score}
		tier, err := gate.Classify(ctx, traj)
		if err != nil {
			t.Errorf("score=%f: unexpected error: %v", tt.score, err)
		}
		if tier != tt.want {
			t.Errorf("score=%f: got %s, want %s", tt.score, tier, tt.want)
		}
	}
}

func TestQualityGate_ClassifyWithJudge(t *testing.T) {
	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = true
	cfg.BorderlineLower = 55
	cfg.BorderlineUpper = 75

	judge := &mockJudge{score: 80}
	gate := NewQualityGate(cfg, judge)
	ctx := context.Background()

	// Score 60 is in borderline zone [55, 75), judge returns 80
	// Combined = (60 + 80) / 2 = 70 → Silver
	traj := &RLTrajectory{AutoScore: 60}
	tier, err := gate.Classify(ctx, traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierSilver {
		t.Errorf("expected silver, got %s", tier)
	}
	if traj.JudgeScore == nil || *traj.JudgeScore != 80 {
		t.Errorf("expected judge score 80, got %v", traj.JudgeScore)
	}
}

func TestQualityGate_JudgePromotesToGold(t *testing.T) {
	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = true
	cfg.BorderlineLower = 55
	cfg.BorderlineUpper = 75

	judge := &mockJudge{score: 100}
	gate := NewQualityGate(cfg, judge)
	ctx := context.Background()

	// Score 70 in borderline, judge returns 100
	// Combined = (70 + 100) / 2 = 85 → Gold
	traj := &RLTrajectory{AutoScore: 70}
	tier, err := gate.Classify(ctx, traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierGold {
		t.Errorf("expected gold, got %s", tier)
	}
}

func TestQualityGate_JudgeFailureFallsBack(t *testing.T) {
	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = true
	cfg.BorderlineLower = 55
	cfg.BorderlineUpper = 75

	judge := &mockJudge{err: fmt.Errorf("network error")}
	gate := NewQualityGate(cfg, judge)
	ctx := context.Background()

	// Score 60 is borderline, judge fails → falls back to auto (silver)
	traj := &RLTrajectory{AutoScore: 60}
	tier, err := gate.Classify(ctx, traj)
	if err == nil {
		t.Fatal("expected error for judge failure")
	}
	if tier != TierSilver {
		t.Errorf("expected silver fallback, got %s", tier)
	}
	if traj.JudgeScore != nil {
		t.Errorf("expected nil judge score on failure, got %v", traj.JudgeScore)
	}
}

func TestQualityGate_SkipsJudgeForClearScores(t *testing.T) {
	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = true
	cfg.BorderlineLower = 55
	cfg.BorderlineUpper = 75

	callCount := 0
	judge := &mockJudge{score: 90}
	gate := NewQualityGate(cfg, &countingJudge{judge: judge, count: &callCount})
	ctx := context.Background()

	// Score 90 is clearly gold — judge should NOT be called
	traj := &RLTrajectory{AutoScore: 90}
	tier, _ := gate.Classify(ctx, traj)
	if tier != TierGold {
		t.Errorf("expected gold, got %s", tier)
	}
	if callCount != 0 {
		t.Errorf("expected 0 judge calls, got %d", callCount)
	}

	// Score 30 is clearly reject — judge should NOT be called
	traj2 := &RLTrajectory{AutoScore: 30}
	tier2, _ := gate.Classify(ctx, traj2)
	if tier2 != TierReject {
		t.Errorf("expected reject, got %s", tier2)
	}
	if callCount != 0 {
		t.Errorf("expected 0 judge calls, got %d", callCount)
	}
}

type countingJudge struct {
	judge *mockJudge
	count *int
}

func (c *countingJudge) Score(ctx context.Context, traj *RLTrajectory) (float64, error) {
	*c.count++
	return c.judge.Score(ctx, traj)
}

func TestQualityGate_ClassifyBatch(t *testing.T) {
	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = false
	gate := NewQualityGate(cfg, nil)
	ctx := context.Background()

	trajectories := []*RLTrajectory{
		{AutoScore: 90},
		{AutoScore: 70},
		{AutoScore: 50},
		{AutoScore: 30},
	}

	stats, err := gate.ClassifyBatch(ctx, trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Total != 4 {
		t.Errorf("expected 4 total, got %d", stats.Total)
	}
	if stats.ByTier[TierGold] != 1 {
		t.Errorf("expected 1 gold, got %d", stats.ByTier[TierGold])
	}
	if stats.ByTier[TierSilver] != 1 {
		t.Errorf("expected 1 silver, got %d", stats.ByTier[TierSilver])
	}
	if stats.ByTier[TierBronze] != 1 {
		t.Errorf("expected 1 bronze, got %d", stats.ByTier[TierBronze])
	}
	if stats.Rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", stats.Rejected)
	}

	// Verify tiers were assigned to trajectories
	if trajectories[0].QualityTier != TierGold {
		t.Errorf("traj[0] expected gold, got %s", trajectories[0].QualityTier)
	}
	if trajectories[3].QualityTier != TierReject {
		t.Errorf("traj[3] expected reject, got %s", trajectories[3].QualityTier)
	}
}
