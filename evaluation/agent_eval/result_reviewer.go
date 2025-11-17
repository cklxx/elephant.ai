package agent_eval

import (
	"sort"
	"strings"
	"time"

	"alex/evaluation/swe_bench"
)

// AutoReviewOptions 控制自动评审与反工行为
type AutoReviewOptions struct {
	Enabled            bool    `json:"enabled"`
	MinPassingScore    float64 `json:"min_passing_score"`
	EnableAutoRework   bool    `json:"enable_auto_rework"`
	MaxReworkTasks     int     `json:"max_rework_tasks"`
	AlwaysReworkFailed bool    `json:"always_rework_failed"`
}

// AutoReviewOverrides 允许通过CLI覆盖部分配置
type AutoReviewOverrides struct {
	Enabled            *bool    `json:"enabled,omitempty"`
	MinPassingScore    *float64 `json:"min_passing_score,omitempty"`
	EnableAutoRework   *bool    `json:"enable_auto_rework,omitempty"`
	MaxReworkTasks     *int     `json:"max_rework_tasks,omitempty"`
	AlwaysReworkFailed *bool    `json:"always_rework_failed,omitempty"`
}

func defaultAutoReviewOptions() *AutoReviewOptions {
	return &AutoReviewOptions{
		Enabled:            true,
		MinPassingScore:    0.75,
		EnableAutoRework:   true,
		MaxReworkTasks:     5,
		AlwaysReworkFailed: true,
	}
}

func cloneAutoReviewOptions(options *AutoReviewOptions) *AutoReviewOptions {
	if options == nil {
		return nil
	}
	copy := *options
	return &copy
}

func applyAutoReviewOverrides(base *AutoReviewOptions, overrides *AutoReviewOverrides) *AutoReviewOptions {
	if overrides == nil {
		return cloneAutoReviewOptions(base)
	}

	if base == nil {
		base = defaultAutoReviewOptions()
	} else {
		base = cloneAutoReviewOptions(base)
	}

	if overrides.Enabled != nil {
		base.Enabled = *overrides.Enabled
	}
	if overrides.MinPassingScore != nil {
		base.MinPassingScore = *overrides.MinPassingScore
	}
	if overrides.EnableAutoRework != nil {
		base.EnableAutoRework = *overrides.EnableAutoRework
	}
	if overrides.MaxReworkTasks != nil {
		base.MaxReworkTasks = *overrides.MaxReworkTasks
	}
	if overrides.AlwaysReworkFailed != nil {
		base.AlwaysReworkFailed = *overrides.AlwaysReworkFailed
	}

	return base
}

// ResultAssessment 针对单个任务的自动评审结果
type ResultAssessment struct {
	InstanceID  string                 `json:"instance_id"`
	TaskID      string                 `json:"task_id"`
	Status      swe_bench.ResultStatus `json:"status"`
	Score       float64                `json:"score"`
	Grade       string                 `json:"grade"`
	Verdict     string                 `json:"verdict"`
	Issues      []string               `json:"issues"`
	NeedsRework bool                   `json:"needs_rework"`
	ReviewedAt  time.Time              `json:"reviewed_at"`
}

// AutoReviewReport 汇总自动评审结果以及反工反馈
type AutoReviewReport struct {
	Assessments []*ResultAssessment `json:"assessments"`
	Rework      *ReworkSummary      `json:"rework_summary,omitempty"`
}

// ReworkSummary 概览反工尝试
type ReworkSummary struct {
	Attempted   int      `json:"attempted"`
	Completed   int      `json:"completed"`
	Improved    int      `json:"improved"`
	StillFailed int      `json:"still_failed"`
	Notes       []string `json:"notes,omitempty"`
}

// ResultAutoReviewer 用于自动判别任务质量
type ResultAutoReviewer struct {
	options *AutoReviewOptions
}

// NewResultAutoReviewer 创建评审器
func NewResultAutoReviewer(options *AutoReviewOptions) *ResultAutoReviewer {
	if options == nil {
		options = defaultAutoReviewOptions()
	}
	return &ResultAutoReviewer{options: options}
}

// UpdateOptions 更新运行时配置
func (rar *ResultAutoReviewer) UpdateOptions(options *AutoReviewOptions) {
	if options == nil {
		options = defaultAutoReviewOptions()
	}
	rar.options = options
}

// Review 对每个任务执行打分
func (rar *ResultAutoReviewer) Review(results []swe_bench.WorkerResult) []*ResultAssessment {
	assessments := make([]*ResultAssessment, 0, len(results))
	for _, result := range results {
		assessment := rar.evaluate(result)
		assessments = append(assessments, assessment)
	}
	return assessments
}

func (rar *ResultAutoReviewer) evaluate(result swe_bench.WorkerResult) *ResultAssessment {
	score := 1.0
	issues := make([]string, 0)

	if result.Status != swe_bench.StatusCompleted {
		score -= 0.5
		issues = append(issues, "任务未成功完成")
	}

	if len(strings.TrimSpace(result.Solution)) < 50 {
		score -= 0.15
		issues = append(issues, "解决方案描述过短")
	}

	if len(result.FilesChanged) == 0 {
		score -= 0.1
		issues = append(issues, "缺少文件修改记录")
	}

	if len(result.Explanation) < 100 {
		score -= 0.1
		issues = append(issues, "推理过程过于简略")
	}

	if result.TokensUsed > 8000 {
		score -= 0.05
		issues = append(issues, "Token 使用偏高")
	}

	if result.Duration > 10*time.Minute {
		score -= 0.05
		issues = append(issues, "执行时间偏长")
	}

	if score < 0 {
		score = 0
	}

	grade := rar.scoreToGrade(score)
	needsRework := score < rar.options.MinPassingScore || (rar.options.AlwaysReworkFailed && result.Status != swe_bench.StatusCompleted)

	verdict := "通过"
	if needsRework {
		verdict = "需复查"
	}

	return &ResultAssessment{
		InstanceID:  result.InstanceID,
		TaskID:      result.TaskID,
		Status:      result.Status,
		Score:       score,
		Grade:       grade,
		Verdict:     verdict,
		Issues:      issues,
		NeedsRework: needsRework,
		ReviewedAt:  time.Now(),
	}
}

func (rar *ResultAutoReviewer) scoreToGrade(score float64) string {
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.8:
		return "B"
	case score >= 0.7:
		return "C"
	case score >= 0.6:
		return "D"
	default:
		return "E"
	}
}

// SelectReworkCandidates 根据评分挑选需要反工的任务
func (rar *ResultAutoReviewer) SelectReworkCandidates(assessments []*ResultAssessment) []string {
	if rar.options == nil || !rar.options.EnableAutoRework {
		return nil
	}

	type candidate struct {
		id    string
		score float64
	}

	cands := make([]candidate, 0)
	for _, assessment := range assessments {
		if assessment.NeedsRework {
			cands = append(cands, candidate{id: assessment.InstanceID, score: assessment.Score})
		}
	}

	sort.Slice(cands, func(i, j int) bool {
		return cands[i].score < cands[j].score
	})

	limit := rar.options.MaxReworkTasks
	if limit <= 0 || limit > len(cands) {
		limit = len(cands)
	}

	reworkIDs := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		reworkIDs = append(reworkIDs, cands[i].id)
	}

	return reworkIDs
}
