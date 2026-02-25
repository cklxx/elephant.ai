package rl

import (
	"fmt"
	"strings"
	"time"

	"alex/evaluation/swe_bench"
)

// Extractor converts WorkerResult traces into RLTrajectory records.
type Extractor struct{}

// NewExtractor creates a new Extractor.
func NewExtractor() *Extractor {
	return &Extractor{}
}

// Extract converts a WorkerResult into an RLTrajectory.
// autoScore should be a 0-100 value from the evaluation pipeline.
func (e *Extractor) Extract(result swe_bench.WorkerResult, evalJobID string, autoScore float64, grade string) (*RLTrajectory, error) {
	if len(result.Trace) == 0 {
		return nil, fmt.Errorf("result %s has no trace steps", result.TaskID)
	}

	steps := make([]TrajectoryStep, len(result.Trace))
	toolSet := make(map[string]struct{})

	for i, ts := range result.Trace {
		reward := computeStepReward(ts, i, len(result.Trace), result.Status)
		steps[i] = TrajectoryStep{
			StepIndex:   ts.Step,
			Thought:     ts.Thought,
			Action:      ts.Action,
			Observation: ts.Observation,
			ToolCall:    ts.ToolCall,
			Timestamp:   ts.Timestamp,
			Reward:      reward,
		}
		if ts.ToolCall != nil && ts.ToolCall.Name != "" {
			toolSet[ts.ToolCall.Name] = struct{}{}
		}
	}

	tools := make([]string, 0, len(toolSet))
	for t := range toolSet {
		tools = append(tools, t)
	}

	outcome := outcomeFromStatus(result.Status)

	id := fmt.Sprintf("traj_%s_%s", evalJobID, result.TaskID)
	if id == "traj__" {
		id = fmt.Sprintf("traj_%s_%s", evalJobID, result.InstanceID)
	}

	return &RLTrajectory{
		ID:          id,
		EvalJobID:   evalJobID,
		TaskID:      result.TaskID,
		InstanceID:  result.InstanceID,
		AutoScore:   autoScore,
		Grade:       grade,
		Steps:       steps,
		Metadata: TrajectoryMeta{
			TotalSteps: len(steps),
			Duration:   result.Duration,
			TokensUsed: result.TokensUsed,
			Cost:       result.Cost,
			ToolsUsed:  tools,
			Outcome:    outcome,
		},
		ExtractedAt: time.Now(),
	}, nil
}

// computeStepReward assigns a per-step reward based on position and result status.
// Terminal steps get higher reward for successful outcomes, negative for failures.
func computeStepReward(step swe_bench.TraceStep, idx, total int, status swe_bench.ResultStatus) float64 {
	isLast := idx == total-1

	// Base reward: small positive for forward progress
	base := 0.1

	// Tool call error penalty
	if step.ToolCall != nil && step.ToolCall.Error != "" {
		base = -0.2
	}

	// Terminal step adjustment
	if isLast {
		switch status {
		case swe_bench.StatusCompleted:
			return 1.0
		case swe_bench.StatusFailed:
			return -0.5
		case swe_bench.StatusTimeout:
			return -0.3
		}
	}

	return base
}

func outcomeFromStatus(status swe_bench.ResultStatus) string {
	switch status {
	case swe_bench.StatusCompleted:
		return "completed"
	case swe_bench.StatusFailed:
		return "failed"
	case swe_bench.StatusTimeout:
		return "timeout"
	case swe_bench.StatusCanceled:
		return "canceled"
	default:
		return strings.ToLower(string(status))
	}
}
