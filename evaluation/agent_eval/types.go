package agent_eval

import (
	"time"

	"alex/evaluation/swe_bench"
)

// EvaluationResults 评估结果
type EvaluationResults struct {
	JobID           string                   `json:"job_id"`
	AgentID         string                   `json:"agent_id,omitempty"`
	Config          *EvaluationConfig        `json:"config,omitempty"`
	Results         []swe_bench.WorkerResult `json:"results"`
	AutoScores      []AutoScore              `json:"auto_scores,omitempty"`
	Judgements      *JudgementSummary        `json:"judgements,omitempty"`
	JudgeRuns       []JudgementResult        `json:"judgement_results,omitempty"`
	Metrics         *EvaluationMetrics       `json:"metrics"`
	Analysis        *AnalysisResult          `json:"analysis"`
	Agent           *AgentProfile            `json:"agent,omitempty"`
	ReportPath      string                   `json:"report_path,omitempty"`
	ReportArtifacts []EvaluationArtifact     `json:"report_artifacts,omitempty"`
	Timestamp       time.Time                `json:"timestamp"`
}

// EvaluationArtifact describes a generated artifact for an evaluation run.
type EvaluationArtifact struct {
	Type   string `json:"type"`
	Format string `json:"format,omitempty"`
	Name   string `json:"name"`
	Path   string `json:"path"`
}

// EvaluationQuery 描述查询评估记录的过滤器
type EvaluationQuery struct {
	AgentID     string
	After       time.Time
	Before      time.Time
	MinScore    float64
	Limit       int
	DatasetPath string
	DatasetType string
	Tags        []string
}

// HasFilters returns true if any filtering field is set.
func (q EvaluationQuery) HasFilters() bool {
	return q.AgentID != "" || !q.After.IsZero() || !q.Before.IsZero() || q.MinScore > 0 || q.Limit > 0 || q.DatasetPath != "" || q.DatasetType != "" || len(q.Tags) > 0
}

// AutoScore 为单个任务的自动评分结果
type AutoScore struct {
	TaskID     string  `json:"task_id"`
	InstanceID string  `json:"instance_id"`
	Score      float64 `json:"score"`
	Grade      string  `json:"grade"`
	Reason     string  `json:"reason"`
}

// ComparisonResult 比较结果（用于A/B测试）
type ComparisonResult struct {
	BaselineResults   *EvaluationResults `json:"baseline_results"`
	ExperimentResults *EvaluationResults `json:"experiment_results"`
	ComparisonMetrics *ComparisonMetrics `json:"comparison_metrics"`
	Improvements      []Improvement      `json:"improvements"`
	Regressions       []Regression       `json:"regressions"`
	Timestamp         time.Time          `json:"timestamp"`
}

// ComparisonMetrics 比较指标
type ComparisonMetrics struct {
	SuccessRateDelta      float64 `json:"success_rate_delta"`
	PerformanceDelta      float64 `json:"performance_delta"`
	CostDelta             float64 `json:"cost_delta"`
	QualityDelta          float64 `json:"quality_delta"`
	SignificanceLevel     float64 `json:"significance_level"`
	StatisticalConfidence float64 `json:"statistical_confidence"`
}

// Improvement 改进项
type Improvement struct {
	Category    string  `json:"category"`
	Metric      string  `json:"metric"`
	Improvement float64 `json:"improvement"`
	Description string  `json:"description"`
}

// Regression 回归项
type Regression struct {
	Category    string  `json:"category"`
	Metric      string  `json:"metric"`
	Regression  float64 `json:"regression"`
	Description string  `json:"description"`
	Impact      string  `json:"impact"`
}

// HistoricalTrend 历史趋势数据
type HistoricalTrend struct {
	EvaluationID string                 `json:"evaluation_id"`
	Timestamp    time.Time              `json:"timestamp"`
	Metrics      *EvaluationMetrics     `json:"metrics"`
	Config       map[string]interface{} `json:"config"`
}

// TrendData 趋势数据
type TrendData struct {
	TimePoints    []time.Time `json:"time_points"`
	SuccessRates  []float64   `json:"success_rates"`
	AvgExecTimes  []float64   `json:"avg_exec_times"`
	QualityScores []float64   `json:"quality_scores"`
	CostPerTask   []float64   `json:"cost_per_task"`
}

// Benchmark 基准测试结果
type Benchmark struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Results     *EvaluationResults     `json:"results"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
}

// EvaluationSession 评估会话
type EvaluationSession struct {
	SessionID      string            `json:"session_id"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        time.Time         `json:"end_time"`
	Status         SessionStatus     `json:"status"`
	Config         *EvaluationConfig `json:"config"`
	Jobs           []*EvaluationJob  `json:"jobs"`
	TotalTasks     int               `json:"total_tasks"`
	CompletedTasks int               `json:"completed_tasks"`
	FailedTasks    int               `json:"failed_tasks"`
}

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusPaused    SessionStatus = "paused"
)

// TaskComplexity 任务复杂性评估
type TaskComplexity struct {
	TaskID          string             `json:"task_id"`
	ComplexityScore float64            `json:"complexity_score"`
	Factors         []ComplexityFactor `json:"factors"`
	EstimatedTime   time.Duration      `json:"estimated_time"`
	EstimatedCost   float64            `json:"estimated_cost"`
}

// ComplexityFactor 复杂性因子
type ComplexityFactor struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// AgentProfile Agent性能档案
type AgentProfile struct {
	AgentID    string    `json:"agent_id"`
	ConfigHash string    `json:"config_hash"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Performance characteristics
	AvgSuccessRate  float64       `json:"avg_success_rate"`
	AvgExecTime     time.Duration `json:"avg_exec_time"`
	AvgCostPerTask  float64       `json:"avg_cost_per_task"`
	AvgQualityScore float64       `json:"avg_quality_score"`

	// Behavioral patterns
	PreferredTools map[string]int `json:"preferred_tools"`
	CommonErrors   map[string]int `json:"common_errors"`
	Strengths      []string       `json:"strengths"`
	Weaknesses     []string       `json:"weaknesses"`

	// Historical data
	EvaluationCount int        `json:"evaluation_count"`
	LastEvaluated   time.Time  `json:"last_evaluated"`
	TrendData       *TrendData `json:"trend_data"`

	// Metadata
	Tags        []string               `json:"tags"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// TaskCategory 任务分类
type TaskCategory string

const (
	CategoryCoding        TaskCategory = "coding"
	CategoryDebugging     TaskCategory = "debugging"
	CategoryRefactoring   TaskCategory = "refactoring"
	CategoryTesting       TaskCategory = "testing"
	CategoryAnalysis      TaskCategory = "analysis"
	CategoryDocumentation TaskCategory = "documentation"
)

// TaskMetadata 任务元数据
type TaskMetadata struct {
	TaskID        string        `json:"task_id"`
	Category      TaskCategory  `json:"category"`
	Difficulty    Difficulty    `json:"difficulty"`
	Language      string        `json:"language"`
	Framework     string        `json:"framework"`
	Domain        string        `json:"domain"`
	Tags          []string      `json:"tags"`
	EstimatedTime time.Duration `json:"estimated_time"`
	Requirements  []string      `json:"requirements"`
}

// Difficulty 难度等级
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
	DifficultyExpert Difficulty = "expert"
)

// EvaluationReport 评估报告
type EvaluationReport struct {
	ReportID    string       `json:"report_id"`
	GeneratedAt time.Time    `json:"generated_at"`
	ReportType  ReportType   `json:"report_type"`
	Format      ReportFormat `json:"format"`

	// Content
	Title    string          `json:"title"`
	Summary  string          `json:"summary"`
	Sections []ReportSection `json:"sections"`

	// Data
	Results     *EvaluationResults  `json:"results"`
	Analysis    *AnalysisResult     `json:"analysis"`
	Comparisons []*ComparisonResult `json:"comparisons,omitempty"`

	// Metadata
	GeneratedBy string                 `json:"generated_by"`
	Version     string                 `json:"version"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ReportType 报告类型
type ReportType string

const (
	ReportTypeSingle     ReportType = "single"
	ReportTypeComparison ReportType = "comparison"
	ReportTypeTrend      ReportType = "trend"
	ReportTypeBenchmark  ReportType = "benchmark"
)

// ReportFormat 报告格式
type ReportFormat string

const (
	ReportFormatMarkdown ReportFormat = "markdown"
	ReportFormatHTML     ReportFormat = "html"
	ReportFormatPDF      ReportFormat = "pdf"
	ReportFormatJSON     ReportFormat = "json"
)

// ReportSection 报告段落
type ReportSection struct {
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Type        SectionType            `json:"type"`
	Order       int                    `json:"order"`
	Subsections []ReportSection        `json:"subsections,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// SectionType 段落类型
type SectionType string

const (
	SectionTypeSummary         SectionType = "summary"
	SectionTypePerformance     SectionType = "performance"
	SectionTypeQuality         SectionType = "quality"
	SectionTypeRecommendations SectionType = "recommendations"
	SectionTypeInsights        SectionType = "insights"
	SectionTypeAlerts          SectionType = "alerts"
	SectionTypeChart           SectionType = "chart"
	SectionTypeTable           SectionType = "table"
)

// ConfigurationDrift 配置漂移检测
type ConfigurationDrift struct {
	ConfigID       string                 `json:"config_id"`
	BaselineConfig map[string]interface{} `json:"baseline_config"`
	CurrentConfig  map[string]interface{} `json:"current_config"`
	DetectedAt     time.Time              `json:"detected_at"`

	Changes   []ConfigChange `json:"changes"`
	RiskLevel string         `json:"risk_level"`
	Impact    string         `json:"impact"`

	Recommendations []string `json:"recommendations"`
}

// ConfigChange 配置变更
type ConfigChange struct {
	Path        string      `json:"path"`
	OldValue    interface{} `json:"old_value"`
	NewValue    interface{} `json:"new_value"`
	ChangeType  ChangeType  `json:"change_type"`
	Impact      ImpactLevel `json:"impact"`
	Description string      `json:"description"`
}

// ChangeType 变更类型
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeModified ChangeType = "modified"
	ChangeTypeRemoved  ChangeType = "removed"
)

// PerformanceRegression 性能回归检测
type PerformanceRegression struct {
	DetectedAt    time.Time `json:"detected_at"`
	MetricName    string    `json:"metric_name"`
	BaselineValue float64   `json:"baseline_value"`
	CurrentValue  float64   `json:"current_value"`
	RegressionPct float64   `json:"regression_percentage"`

	Threshold  float64            `json:"threshold"`
	Severity   RegressionSeverity `json:"severity"`
	Confidence float64            `json:"confidence"`

	AffectedTasks  []string `json:"affected_tasks"`
	PossibleCauses []string `json:"possible_causes"`
}

// RegressionSeverity 回归严重程度
type RegressionSeverity string

const (
	SeverityCritical RegressionSeverity = "critical"
	SeverityMajor    RegressionSeverity = "major"
	SeverityMinor    RegressionSeverity = "minor"
)

// AnomalyDetection 异常检测结果
type AnomalyDetection struct {
	TaskID      string          `json:"task_id"`
	DetectedAt  time.Time       `json:"detected_at"`
	AnomalyType AnomalyType     `json:"anomaly_type"`
	Severity    AnomalySeverity `json:"severity"`
	Confidence  float64         `json:"confidence"`

	Description   string             `json:"description"`
	MetricValues  map[string]float64 `json:"metric_values"`
	ExpectedRange map[string]float64 `json:"expected_range"`

	Investigation []string `json:"investigation_steps"`
	AutoResolved  bool     `json:"auto_resolved"`
}

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyPerformance AnomalyType = "performance"
	AnomalyQuality     AnomalyType = "quality"
	AnomalyBehavior    AnomalyType = "behavior"
	AnomalyCost        AnomalyType = "cost"
)

// AnomalySeverity 异常严重程度
type AnomalySeverity string

const (
	AnomalySeverityCritical AnomalySeverity = "critical"
	AnomalySeverityHigh     AnomalySeverity = "high"
	AnomalySeverityMedium   AnomalySeverity = "medium"
	AnomalySeverityLow      AnomalySeverity = "low"
)

// CacheStats 缓存统计
type CacheStats struct {
	HitCount    int64     `json:"hit_count"`
	MissCount   int64     `json:"miss_count"`
	HitRate     float64   `json:"hit_rate"`
	CacheSize   int64     `json:"cache_size_bytes"`
	EntryCount  int       `json:"entry_count"`
	LastCleared time.Time `json:"last_cleared"`
}

// SystemHealth 系统健康状态
type SystemHealth struct {
	Status    HealthStatus `json:"status"`
	CheckedAt time.Time    `json:"checked_at"`

	Components   map[string]ComponentHealth `json:"components"`
	OverallScore float64                    `json:"overall_score"`

	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`

	Recommendations []string `json:"recommendations"`
}

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Name         string        `json:"name"`
	Status       HealthStatus  `json:"status"`
	LastChecked  time.Time     `json:"last_checked"`
	ResponseTime time.Duration `json:"response_time"`
	ErrorRate    float64       `json:"error_rate"`
	Message      string        `json:"message,omitempty"`
}
