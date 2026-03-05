package agent_eval

import (
	"context"
	"fmt"
	"os"
	"strings"

	"alex/evaluation/swe_bench"

	"gopkg.in/yaml.v3"
)

// JudgementStatus represents the lifecycle state of a judgement.
type JudgementStatus string

const (
	JudgementStatusPending    JudgementStatus = "pending"
	JudgementStatusNeedsAgent JudgementStatus = "needs_agent"
	JudgementStatusPassed     JudgementStatus = "passed"
	JudgementStatusFailed     JudgementStatus = "failed"
)

// JudgeRubric describes the weighted dimensions used to assess a task.
type JudgeRubric struct {
	Version       string           `yaml:"version" json:"version"`
	Name          string           `yaml:"name" json:"name"`
	Description   string           `yaml:"description,omitempty" json:"description,omitempty"`
	PassThreshold float64          `yaml:"pass_threshold" json:"pass_threshold"`
	FailOnZero    []string         `yaml:"fail_on_zero,omitempty" json:"fail_on_zero,omitempty"`
	Dimensions    []JudgeDimension `yaml:"dimensions" json:"dimensions"`
}

// JudgeDimension defines a single scoring dimension.
type JudgeDimension struct {
	ID          string  `yaml:"id" json:"id"`
	Name        string  `yaml:"name" json:"name"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Weight      float64 `yaml:"weight" json:"weight"`
	Auto        bool    `yaml:"auto" json:"auto"`
	Guidance    string  `yaml:"guidance,omitempty" json:"guidance,omitempty"`
}

// DimensionScore captures a per-dimension judgement result.
type DimensionScore struct {
	ID       string  `json:"id"`
	Score    int     `json:"score"` // 0-2
	Weight   float64 `json:"weight"`
	Source   string  `json:"source"` // auto | agent
	Notes    string  `json:"notes,omitempty"`
	Evidence string  `json:"evidence,omitempty"`
}

// AutoJudgement holds auto-evaluated dimension scores.
type AutoJudgement struct {
	Status     JudgementStatus  `json:"status"`
	Score      float64          `json:"score"` // 0-1 normalized
	Dimensions []DimensionScore `json:"dimensions"`
}

// AgentJudgement holds agent-evaluated dimension scores.
type AgentJudgement struct {
	Status     JudgementStatus  `json:"status"`
	Score      float64          `json:"score"` // 0-1 normalized
	Dimensions []DimensionScore `json:"dimensions"`
	Notes      string           `json:"notes,omitempty"`
}

// JudgementOutcome is the combined result after auto + agent judgement.
type JudgementOutcome struct {
	Status JudgementStatus `json:"status"`
	Score  float64         `json:"score"` // 0-1 normalized
	Notes  string          `json:"notes,omitempty"`
}

// JudgementResult contains all judgement outputs for a task.
type JudgementResult struct {
	TaskID     string           `json:"task_id"`
	InstanceID string           `json:"instance_id"`
	Auto       AutoJudgement    `json:"auto"`
	Agent      AgentJudgement   `json:"agent"`
	Final      JudgementOutcome `json:"final"`
}

// JudgementSummary aggregates judgement outcomes across tasks.
type JudgementSummary struct {
	Total    int             `json:"total"`
	Passed   int             `json:"passed"`
	Failed   int             `json:"failed"`
	Pending  int             `json:"pending"`
	PassRate float64         `json:"pass_rate"`
	Status   JudgementStatus `json:"status"`
}

// AgentJudge defines the interface for LLM/Human judgement.
type AgentJudge interface {
	Judge(ctx context.Context, task EvalTask, result swe_bench.WorkerResult, rubric JudgeRubric) (AgentJudgement, error)
}

// NoopAgentJudge returns pending judgements without scoring.
type NoopAgentJudge struct{}

func (NoopAgentJudge) Judge(_ context.Context, _ EvalTask, _ swe_bench.WorkerResult, _ JudgeRubric) (AgentJudgement, error) {
	return AgentJudgement{Status: JudgementStatusPending}, nil
}

// LoadJudgeRubric loads a rubric from a YAML file.
func LoadJudgeRubric(path string) (*JudgeRubric, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("rubric path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rubric: %w", err)
	}
	var rubric JudgeRubric
	if err := yaml.Unmarshal(data, &rubric); err != nil {
		return nil, fmt.Errorf("decode rubric: %w", err)
	}
	if err := rubric.Validate(); err != nil {
		return nil, err
	}
	return &rubric, nil
}

// Validate ensures the rubric is well-formed.
func (r JudgeRubric) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("rubric name is required")
	}
	if len(r.Dimensions) == 0 {
		return fmt.Errorf("rubric dimensions are required")
	}

	var totalWeight float64
	for _, d := range r.Dimensions {
		if strings.TrimSpace(d.ID) == "" {
			return fmt.Errorf("rubric dimension id is required")
		}
		if d.Weight <= 0 {
			return fmt.Errorf("rubric dimension %s weight must be positive", d.ID)
		}
		totalWeight += d.Weight
	}
	if totalWeight <= 0 {
		return fmt.Errorf("rubric total weight must be positive")
	}
	return nil
}

// RunJudgementPipeline evaluates all results against the eval set and rubric.
func RunJudgementPipeline(ctx context.Context, set *EvalSet, results []swe_bench.WorkerResult, rubric JudgeRubric, agentJudge AgentJudge) (*JudgementSummary, []JudgementResult, error) {
	if set == nil {
		return nil, nil, fmt.Errorf("eval set is required")
	}
	if err := rubric.Validate(); err != nil {
		return nil, nil, err
	}
	if agentJudge == nil {
		agentJudge = NoopAgentJudge{}
	}

	taskMap := make(map[string]EvalTask, len(set.Tasks))
	for _, task := range set.Tasks {
		taskMap[task.ID] = task
	}

	judgements := make([]JudgementResult, 0, len(results))
	summary := &JudgementSummary{Total: len(results)}

	for _, result := range results {
		taskID := strings.TrimSpace(result.TaskID)
		if taskID == "" {
			taskID = strings.TrimSpace(result.InstanceID)
		}

		task, ok := taskMap[taskID]
		if !ok {
			judgements = append(judgements, JudgementResult{
				TaskID:     taskID,
				InstanceID: result.InstanceID,
				Auto:       AutoJudgement{Status: JudgementStatusFailed},
				Agent:      AgentJudgement{Status: JudgementStatusPending},
				Final:      JudgementOutcome{Status: JudgementStatusFailed, Notes: "task not in eval set"},
			})
			summary.Failed++
			continue
		}

		auto := AutoJudgeTask(task, result, rubric)
		agent := AgentJudgement{Status: JudgementStatusPending}
		final := JudgementOutcome{Status: JudgementStatusNeedsAgent, Score: auto.Score}

		if auto.Status == JudgementStatusFailed {
			final.Status = JudgementStatusFailed
			final.Score = auto.Score
		} else if hasAgentDimensions(rubric) {
			agentResult, err := agentJudge.Judge(ctx, task, result, rubric)
			if err != nil {
				agentResult = AgentJudgement{Status: JudgementStatusPending, Notes: err.Error()}
			}
			agent = agentResult
			final = combineJudgement(auto, agent, rubric)
		} else {
			final.Status = auto.Status
		}

		judgements = append(judgements, JudgementResult{
			TaskID:     task.ID,
			InstanceID: result.InstanceID,
			Auto:       auto,
			Agent:      agent,
			Final:      final,
		})

		switch final.Status {
		case JudgementStatusPassed:
			summary.Passed++
		case JudgementStatusFailed:
			summary.Failed++
		default:
			summary.Pending++
		}
	}

	if summary.Total > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.Total)
	}
	switch {
	case summary.Pending > 0:
		summary.Status = JudgementStatusNeedsAgent
	case summary.Failed > 0:
		summary.Status = JudgementStatusFailed
	default:
		summary.Status = JudgementStatusPassed
	}

	return summary, judgements, nil
}

// AutoJudgeTask scores auto-evaluable dimensions defined in the rubric.
func AutoJudgeTask(task EvalTask, result swe_bench.WorkerResult, rubric JudgeRubric) AutoJudgement {
	dimensions := make([]DimensionScore, 0)
	failOnZero := make(map[string]struct{}, len(rubric.FailOnZero))
	for _, id := range rubric.FailOnZero {
		failOnZero[strings.TrimSpace(id)] = struct{}{}
	}

	var totalWeight float64
	var weightedScore float64
	failed := false

	for _, dim := range rubric.Dimensions {
		if !dim.Auto {
			continue
		}
		score := autoScoreDimension(dim.ID, task, result)
		dimensions = append(dimensions, DimensionScore{
			ID:     dim.ID,
			Score:  score,
			Weight: dim.Weight,
			Source: "auto",
		})

		totalWeight += dim.Weight
		weightedScore += float64(score) / 2.0 * dim.Weight
		if score == 0 {
			if _, mustFail := failOnZero[dim.ID]; mustFail {
				failed = true
			}
		}
	}

	normalized := 0.0
	if totalWeight > 0 {
		normalized = weightedScore / totalWeight
	}

	status := JudgementStatusPassed
	if failed {
		status = JudgementStatusFailed
	} else if normalized < rubric.PassThreshold {
		status = JudgementStatusFailed
	}

	return AutoJudgement{
		Status:     status,
		Score:      normalized,
		Dimensions: dimensions,
	}
}

func autoScoreDimension(id string, task EvalTask, result swe_bench.WorkerResult) int {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "completion":
		if result.Status == swe_bench.StatusCompleted {
			return 2
		}
		if result.Status == swe_bench.StatusTimeout || result.Status == swe_bench.StatusFailed {
			return 0
		}
		return 1
	case "format":
		return formatScore(task, result)
	case "constraints":
		return constraintScore(task, result)
	default:
		return 1
	}
}

func formatScore(task EvalTask, result swe_bench.WorkerResult) int {
	text := pickResultText(result)
	expected := strings.ToLower(task.ExpectedOutput())
	if expected == "" {
		return 1
	}
	switch {
	case strings.Contains(expected, "table"):
		if strings.Contains(text, "|") {
			return 2
		}
		return 0
	case strings.Contains(expected, "ranked list"), strings.Contains(expected, "list"):
		trimmed := strings.TrimSpace(text)
		if strings.Contains(text, "\n-") || strings.Contains(text, "\n1.") ||
			strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "1.") || strings.HasPrefix(trimmed, "1)") {
			return 2
		}
		return 0
	case strings.Contains(expected, "json"):
		if strings.Contains(text, "{") && strings.Contains(text, "}") {
			return 2
		}
		return 0
	default:
		return 1
	}
}

func constraintScore(task EvalTask, result swe_bench.WorkerResult) int {
	constraints := task.ConstraintCriteria()
	if len(constraints) == 0 {
		return 1
	}
	text := pickResultText(result)
	covered := 0
	for _, c := range constraints {
		if constraintMentioned(text, c) {
			covered++
		}
	}
	ratio := float64(covered) / float64(len(constraints))
	switch {
	case ratio >= 0.8:
		return 2
	case ratio >= 0.5:
		return 1
	default:
		return 0
	}
}

func pickResultText(result swe_bench.WorkerResult) string {
	if strings.TrimSpace(result.Solution) != "" {
		return strings.ToLower(result.Solution)
	}
	return strings.ToLower(result.Explanation)
}

func constraintMentioned(text, constraint string) bool {
	words := strings.Fields(strings.ToLower(constraint))
	for _, word := range words {
		word = strings.Trim(word, ".,;:()[]\"'")
		if len(word) < 4 {
			continue
		}
		if stopword(word) {
			continue
		}
		if strings.Contains(text, word) {
			return true
		}
		if strings.HasSuffix(word, "es") {
			base := strings.TrimSuffix(word, "es")
			if len(base) >= 4 && strings.Contains(text, base) {
				return true
			}
		} else if strings.HasSuffix(word, "s") {
			base := strings.TrimSuffix(word, "s")
			if len(base) >= 4 && strings.Contains(text, base) {
				return true
			}
		}
	}
	return false
}

func stopword(word string) bool {
	switch word {
	case "the", "with", "that", "this", "from", "into", "over", "your", "you", "and", "for", "are", "not", "all", "out", "per", "must", "should":
		return true
	default:
		return false
	}
}

func hasAgentDimensions(rubric JudgeRubric) bool {
	for _, dim := range rubric.Dimensions {
		if !dim.Auto {
			return true
		}
	}
	return false
}

func combineJudgement(auto AutoJudgement, agent AgentJudgement, rubric JudgeRubric) JudgementOutcome {
	if auto.Status == JudgementStatusFailed {
		return JudgementOutcome{Status: JudgementStatusFailed, Score: auto.Score}
	}
	if agent.Status == JudgementStatusPending || agent.Status == JudgementStatusNeedsAgent {
		return JudgementOutcome{Status: JudgementStatusNeedsAgent, Score: auto.Score}
	}
	if agent.Status == JudgementStatusFailed {
		return JudgementOutcome{Status: JudgementStatusFailed, Score: agent.Score}
	}

	autoWeight, agentWeight := rubricWeights(rubric)
	total := autoWeight + agentWeight
	score := 0.0
	if total > 0 {
		score = (auto.Score*autoWeight + agent.Score*agentWeight) / total
	}
	status := JudgementStatusPassed
	if score < rubric.PassThreshold {
		status = JudgementStatusFailed
	}
	return JudgementOutcome{Status: status, Score: score}
}

func rubricWeights(rubric JudgeRubric) (autoWeight float64, agentWeight float64) {
	for _, dim := range rubric.Dimensions {
		if dim.Auto {
			autoWeight += dim.Weight
		} else {
			agentWeight += dim.Weight
		}
	}
	return autoWeight, agentWeight
}

// ExpectedOutput returns the expected output hints from pass criteria.
func (t EvalTask) ExpectedOutput() string {
	for _, c := range t.PassCriteria {
		if strings.HasPrefix(c, "expected_output:") {
			return strings.TrimSpace(strings.TrimPrefix(c, "expected_output:"))
		}
	}
	return ""
}

// ConstraintCriteria returns constraints extracted from the pass criteria list.
func (t EvalTask) ConstraintCriteria() []string {
	var constraints []string
	for _, c := range t.PassCriteria {
		if strings.HasPrefix(c, "expected_output:") {
			continue
		}
		if strings.TrimSpace(c) != "" {
			constraints = append(constraints, c)
		}
	}
	return constraints
}
