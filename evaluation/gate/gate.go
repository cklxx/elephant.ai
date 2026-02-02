// Package gate provides CI evaluation gating for PRs.
//
// It orchestrates an evaluation run via the existing agent_eval infrastructure,
// checks configurable thresholds (minimum score, minimum grade, max duration),
// and produces a Markdown summary suitable for PR comments.
package gate

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent_eval "alex/evaluation/agent_eval"
)

// gradeOrder defines the ordinal ranking for letter grades.
// Higher values represent better grades.
var gradeOrder = map[string]int{"A": 5, "B": 4, "C": 3, "D": 2, "F": 1}

// GateConfig holds the thresholds and parameters for an evaluation gate.
type GateConfig struct {
	// MinScore is the minimum pass-rate (0.0-1.0) required. Default 0.80.
	MinScore float64

	// MinGrade is the minimum letter grade required (A/B/C/D/F). Default "B".
	MinGrade string

	// MaxDuration is the wall-clock budget for the gate run. Default 5m for quick.
	MaxDuration time.Duration

	// RequiredDataset is the path to the dataset file used for evaluation.
	RequiredDataset string

	// InstanceLimit caps the number of evaluation instances. Default 3 for quick.
	InstanceLimit int

	// Workers is the number of concurrent evaluation workers. Default 2.
	Workers int
}

// GateResult captures the outcome of an evaluation gate check.
type GateResult struct {
	Passed         bool          `json:"passed"`
	Score          float64       `json:"score"`
	Grade          string        `json:"grade"`
	Duration       time.Duration `json:"duration"`
	Summary        string        `json:"summary"`
	FailureReasons []string      `json:"failure_reasons,omitempty"`
}

// EvalGate orchestrates evaluation runs and checks them against thresholds.
type EvalGate struct {
	// managerFactory builds an EvaluationManager for a given output directory.
	// Exposed for testing; production code uses the default factory.
	managerFactory func(outputDir string) (evalRunner, error)
}

// evalRunner is the subset of agent_eval.CLIManager that EvalGate needs.
type evalRunner interface {
	RunEvaluation(ctx context.Context, opts *agent_eval.EvaluationOptions) (*agent_eval.EvaluationJob, error)
}

// NewEvalGate creates a new EvalGate with the default manager factory.
func NewEvalGate() *EvalGate {
	return &EvalGate{
		managerFactory: func(outputDir string) (evalRunner, error) {
			return agent_eval.NewCLIManager(outputDir)
		},
	}
}

// DefaultQuickGateConfig returns a GateConfig tuned for fast PR gates.
func DefaultQuickGateConfig() GateConfig {
	return GateConfig{
		MinScore:        0.80,
		MinGrade:        "B",
		MaxDuration:     5 * time.Minute,
		RequiredDataset: "evaluation/swe_bench/real_instances.json",
		InstanceLimit:   3,
		Workers:         2,
	}
}

// DefaultFullGateConfig returns a GateConfig for comprehensive evaluation.
func DefaultFullGateConfig() GateConfig {
	return GateConfig{
		MinScore:        0.70,
		MinGrade:        "C",
		MaxDuration:     30 * time.Minute,
		RequiredDataset: "evaluation/swe_bench/real_instances.json",
		InstanceLimit:   0, // 0 means all instances
		Workers:         4,
	}
}

// Evaluate orchestrates an evaluation run and checks the results against the
// given config thresholds. It returns a GateResult summarizing the outcome.
func (g *EvalGate) Evaluate(ctx context.Context, config GateConfig) (*GateResult, error) {
	start := time.Now()

	// Apply duration budget via context deadline.
	if config.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.MaxDuration)
		defer cancel()
	}

	outputDir := "evaluation_results"
	manager, err := g.managerFactory(outputDir)
	if err != nil {
		return nil, fmt.Errorf("gate: failed to create evaluation manager: %w", err)
	}

	opts := agent_eval.DefaultEvaluationOptions()
	opts.InstanceLimit = config.InstanceLimit
	opts.MaxWorkers = config.Workers
	if config.RequiredDataset != "" {
		opts.DatasetPath = config.RequiredDataset
	}

	job, err := manager.RunEvaluation(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("gate: evaluation run failed: %w", err)
	}

	duration := time.Since(start)

	// Extract score and grade from results.
	score := 0.0
	grade := "F"
	if job.Results != nil && job.Results.Analysis != nil {
		score = job.Results.Analysis.Summary.OverallScore
		grade = job.Results.Analysis.Summary.PerformanceGrade
	}

	result := g.CheckThresholds(score, grade, config)
	result.Duration = duration
	result.Summary = g.FormatSummary(result)

	return result, nil
}

// CheckThresholds is a pure function that checks whether a score and grade
// meet the requirements specified in config. It returns a GateResult with
// Passed set accordingly and any failure reasons populated.
func (g *EvalGate) CheckThresholds(score float64, grade string, config GateConfig) *GateResult {
	result := &GateResult{
		Passed: true,
		Score:  score,
		Grade:  grade,
	}

	if score < config.MinScore {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("score %.1f%% is below minimum %.1f%%", score*100, config.MinScore*100))
	}

	if !gradeAtLeast(grade, config.MinGrade) {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("grade %s is below minimum %s", grade, config.MinGrade))
	}

	return result
}

// FormatSummary returns a Markdown-formatted summary suitable for posting as
// a PR comment.
func (g *EvalGate) FormatSummary(result *GateResult) string {
	var sb strings.Builder

	if result.Passed {
		sb.WriteString("## Eval Gate: PASSED\n\n")
	} else {
		sb.WriteString("## Eval Gate: FAILED\n\n")
	}

	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Score  | %.1f%% |\n", result.Score*100))
	sb.WriteString(fmt.Sprintf("| Grade  | %s    |\n", result.Grade))
	if result.Duration > 0 {
		sb.WriteString(fmt.Sprintf("| Duration | %s |\n", result.Duration.Round(time.Second)))
	}

	if len(result.FailureReasons) > 0 {
		sb.WriteString("\n### Failure Reasons\n\n")
		for _, reason := range result.FailureReasons {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
	}

	return sb.String()
}

// gradeAtLeast returns true if grade is at least as good as min.
// Unknown grades are treated as the lowest possible rank (0).
func gradeAtLeast(grade, min string) bool {
	return gradeOrder[grade] >= gradeOrder[min]
}
