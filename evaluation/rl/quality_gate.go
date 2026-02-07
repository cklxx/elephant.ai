package rl

import (
	"context"
	"fmt"
)

// Judge is an optional LLM-based quality judge for borderline trajectories.
type Judge interface {
	Score(ctx context.Context, traj *RLTrajectory) (float64, error)
}

// QualityGate classifies trajectories into quality tiers using auto scores
// and optionally an LLM judge for borderline cases.
type QualityGate struct {
	config QualityConfig
	judge  Judge
}

// NewQualityGate creates a QualityGate with the given config and optional judge.
func NewQualityGate(config QualityConfig, judge Judge) *QualityGate {
	return &QualityGate{config: config, judge: judge}
}

// Classify assigns a QualityTier to the trajectory. If the score is in the
// borderline zone and a judge is configured, the judge is consulted.
func (g *QualityGate) Classify(ctx context.Context, traj *RLTrajectory) (QualityTier, error) {
	score := traj.AutoScore

	// Fast path: clearly above gold or below bronze
	if score >= g.config.GoldMinScore {
		return TierGold, nil
	}
	if score < g.config.BronzeMinScore {
		return TierReject, nil
	}

	// Borderline zone: consult judge if available
	if g.config.IsBorderline(score) && g.judge != nil {
		judgeScore, err := g.judge.Score(ctx, traj)
		if err != nil {
			// Fall back to auto classification on judge failure
			return g.config.ClassifyTier(score), fmt.Errorf("judge failed (falling back to auto): %w", err)
		}
		traj.JudgeScore = &judgeScore

		// Re-classify with averaged score
		combined := (score + judgeScore) / 2
		return g.config.ClassifyTier(combined), nil
	}

	return g.config.ClassifyTier(score), nil
}

// ClassifyBatch classifies a slice of trajectories and returns stats.
func (g *QualityGate) ClassifyBatch(ctx context.Context, trajectories []*RLTrajectory) (ExtractionStats, error) {
	stats := ExtractionStats{
		Total:  len(trajectories),
		ByTier: make(map[QualityTier]int),
	}

	for _, traj := range trajectories {
		tier, err := g.Classify(ctx, traj)
		if err != nil {
			// Logged but not fatal — classification still happened
			_ = err
		}
		traj.QualityTier = tier
		stats.ByTier[tier]++
		if tier == TierReject {
			stats.Rejected++
		}
		if traj.JudgeScore != nil {
			stats.Judged++
		}
	}

	return stats, nil
}
