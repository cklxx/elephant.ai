package rl

import (
	"time"

	"alex/evaluation/swe_bench"
)

// QualityTier categorizes RL trajectories by quality.
type QualityTier string

const (
	TierGold   QualityTier = "gold"
	TierSilver QualityTier = "silver"
	TierBronze QualityTier = "bronze"
	TierReject QualityTier = "reject"
)

// ValidTiers lists all valid quality tiers in descending order.
var ValidTiers = []QualityTier{TierGold, TierSilver, TierBronze, TierReject}

// RLTrajectory represents a complete agent trajectory suitable for RL training.
type RLTrajectory struct {
	ID          string      `json:"id"`
	EvalJobID   string      `json:"eval_job_id"`
	TaskID      string      `json:"task_id"`
	InstanceID  string      `json:"instance_id"`
	QualityTier QualityTier `json:"quality_tier"`
	AutoScore   float64     `json:"auto_score"`
	JudgeScore  *float64    `json:"judge_score,omitempty"`
	Grade       string      `json:"grade"`
	Steps       []TrajectoryStep `json:"steps"`
	Metadata    TrajectoryMeta   `json:"metadata"`
	ExtractedAt time.Time        `json:"extracted_at"`
}

// TrajectoryStep represents a single ReAct step within a trajectory.
type TrajectoryStep struct {
	StepIndex   int              `json:"step_index"`
	Thought     string           `json:"thought,omitempty"`
	Action      string           `json:"action"`
	Observation string           `json:"observation,omitempty"`
	ToolCall    *swe_bench.ToolCall `json:"tool_call,omitempty"`
	Timestamp   time.Time        `json:"timestamp"`
	Reward      float64          `json:"reward"`
}

// TrajectoryMeta holds aggregate metadata for a trajectory.
type TrajectoryMeta struct {
	TotalSteps int           `json:"total_steps"`
	Duration   time.Duration `json:"duration"`
	TokensUsed int           `json:"tokens_used"`
	Cost       float64       `json:"cost"`
	ToolsUsed  []string      `json:"tools_used,omitempty"`
	Outcome    string        `json:"outcome"` // "completed", "failed", "timeout"
}

// QualityConfig holds configurable thresholds for tier classification.
type QualityConfig struct {
	GoldMinScore   float64 `yaml:"gold_min_score" json:"gold_min_score"`
	SilverMinScore float64 `yaml:"silver_min_score" json:"silver_min_score"`
	BronzeMinScore float64 `yaml:"bronze_min_score" json:"bronze_min_score"`

	// Judge escalation boundaries
	JudgeEnabled      bool    `yaml:"judge_enabled" json:"judge_enabled"`
	BorderlineLower   float64 `yaml:"borderline_lower" json:"borderline_lower"`
	BorderlineUpper   float64 `yaml:"borderline_upper" json:"borderline_upper"`
	JudgeProvider     string  `yaml:"judge_provider" json:"judge_provider"`
	JudgeModel        string  `yaml:"judge_model" json:"judge_model"`
}

// DefaultQualityConfig returns sensible defaults for quality gating.
func DefaultQualityConfig() QualityConfig {
	return QualityConfig{
		GoldMinScore:   80,
		SilverMinScore: 60,
		BronzeMinScore: 40,
		JudgeEnabled:   false,
		BorderlineLower: 55,
		BorderlineUpper: 75,
		JudgeProvider:   "claude",
		JudgeModel:      "claude-sonnet-4-5-20250929",
	}
}

// ClassifyTier returns the quality tier for a given auto score.
func (c QualityConfig) ClassifyTier(score float64) QualityTier {
	switch {
	case score >= c.GoldMinScore:
		return TierGold
	case score >= c.SilverMinScore:
		return TierSilver
	case score >= c.BronzeMinScore:
		return TierBronze
	default:
		return TierReject
	}
}

// IsBorderline returns true if the score falls in the borderline zone for judge escalation.
func (c QualityConfig) IsBorderline(score float64) bool {
	return c.JudgeEnabled && score >= c.BorderlineLower && score < c.BorderlineUpper
}

// ExtractionStats holds counters for an RL extraction run.
type ExtractionStats struct {
	Total    int            `json:"total"`
	ByTier   map[QualityTier]int `json:"by_tier"`
	Rejected int            `json:"rejected"`
	Judged   int            `json:"judged"`
}
