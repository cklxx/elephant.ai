package agent_eval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/evaluation/swe_bench"
	"alex/internal/domain/workflow"
)

// EvaluationMetrics 评估指标
type EvaluationMetrics struct {
	// Performance Metrics
	Performance PerformanceMetrics `json:"performance"`

	// Quality Metrics
	Quality QualityMetrics `json:"quality"`

	// Resource Usage
	Resources ResourceMetrics `json:"resources"`

	// Behavioral Metrics
	Behavior BehaviorMetrics `json:"behavior"`

	// Metadata
	Timestamp    time.Time `json:"timestamp"`
	TotalTasks   int       `json:"total_tasks"`
	EvaluationID string    `json:"evaluation_id"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	SuccessRate      float64       `json:"success_rate"`
	AvgExecutionTime time.Duration `json:"avg_execution_time"`
	MedianTime       time.Duration `json:"median_time"`
	P95Time          time.Duration `json:"p95_time"`
	TimeoutRate      float64       `json:"timeout_rate"`
	RetryRate        float64       `json:"retry_rate"`
}

// QualityMetrics 质量指标
type QualityMetrics struct {
	SolutionQuality    float64 `json:"solution_quality"`
	ErrorRecoveryRate  float64 `json:"error_recovery_rate"`
	ConsistencyScore   float64 `json:"consistency_score"`
	ComplexityHandling float64 `json:"complexity_handling"`
}

// ResourceMetrics 资源指标
type ResourceMetrics struct {
	AvgTokensUsed  int     `json:"avg_tokens_used"`
	TotalTokens    int     `json:"total_tokens"`
	AvgCostPerTask float64 `json:"avg_cost_per_task"`
	TotalCost      float64 `json:"total_cost"`
	MemoryUsage    int64   `json:"memory_usage_mb"`
}

// BehaviorMetrics 行为指标
type BehaviorMetrics struct {
	AvgToolCalls     float64        `json:"avg_tool_calls"`
	ToolUsagePattern map[string]int `json:"tool_usage_pattern"`
	CommonFailures   map[string]int `json:"common_failures"`
	ErrorPatterns    []string       `json:"error_patterns"`
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	collectors []MetricCollector
}

// MetricCollector 指标收集器接口
type MetricCollector interface {
	Collect(results []swe_bench.WorkerResult) (interface{}, error)
	Name() string
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		collectors: []MetricCollector{
			&PerformanceCollector{},
			&QualityCollector{},
			&ResourceCollector{},
			&BehaviorCollector{},
		},
	}
}

// Collect 收集指标
func (mc *MetricsCollector) Collect(results []swe_bench.WorkerResult) (*EvaluationMetrics, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results to collect metrics from")
	}

	metrics := &EvaluationMetrics{
		Timestamp:  time.Now(),
		TotalTasks: len(results),
	}

	// 性能指标
	perfCollector := &PerformanceCollector{}
	perfData, err := perfCollector.Collect(results)
	if err != nil {
		return nil, fmt.Errorf("failed to collect performance metrics: %w", err)
	}
	metrics.Performance = perfData.(PerformanceMetrics)

	// 质量指标
	qualityCollector := &QualityCollector{}
	qualityData, err := qualityCollector.Collect(results)
	if err != nil {
		return nil, fmt.Errorf("failed to collect quality metrics: %w", err)
	}
	metrics.Quality = qualityData.(QualityMetrics)

	// 资源指标
	resourceCollector := &ResourceCollector{}
	resourceData, err := resourceCollector.Collect(results)
	if err != nil {
		return nil, fmt.Errorf("failed to collect resource metrics: %w", err)
	}
	metrics.Resources = resourceData.(ResourceMetrics)

	// 行为指标
	behaviorCollector := &BehaviorCollector{}
	behaviorData, err := behaviorCollector.Collect(results)
	if err != nil {
		return nil, fmt.Errorf("failed to collect behavior metrics: %w", err)
	}
	metrics.Behavior = behaviorData.(BehaviorMetrics)

	return metrics, nil
}

// PerformanceCollector 性能指标收集器
type PerformanceCollector struct{}

func (pc *PerformanceCollector) Name() string { return "performance" }

func (pc *PerformanceCollector) Collect(results []swe_bench.WorkerResult) (interface{}, error) {
	var successCount, timeoutCount, retryCount int
	var durations []time.Duration

	for _, result := range results {
		durations = append(durations, result.Duration)

		switch result.Status {
		case swe_bench.StatusCompleted:
			successCount++
		case swe_bench.StatusTimeout:
			timeoutCount++
		}

		// 假设RetryCount字段存在
		if result.RetryCount > 0 {
			retryCount++
		}
	}

	// 排序以计算统计值
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	var totalDuration time.Duration
	for _, d := range durations {
		totalDuration += d
	}

	metrics := PerformanceMetrics{
		SuccessRate:      float64(successCount) / float64(len(results)),
		AvgExecutionTime: totalDuration / time.Duration(len(results)),
		TimeoutRate:      float64(timeoutCount) / float64(len(results)),
		RetryRate:        float64(retryCount) / float64(len(results)),
	}

	if len(durations) > 0 {
		medianIdx := len(durations) / 2
		metrics.MedianTime = durations[medianIdx]

		p95Idx := int(float64(len(durations)) * 0.95)
		if p95Idx >= len(durations) {
			p95Idx = len(durations) - 1
		}
		metrics.P95Time = durations[p95Idx]
	}

	return metrics, nil
}

// QualityCollector 质量指标收集器
type QualityCollector struct{}

func (qc *QualityCollector) Name() string { return "quality" }

func (qc *QualityCollector) Collect(results []swe_bench.WorkerResult) (interface{}, error) {
	var solutionScores []float64
	var errorRecoveryCount int
	var consistencyScores []float64

	for _, result := range results {
		// 简化的解决方案质量评估
		qualityScore := qc.assessSolutionQuality(result)
		solutionScores = append(solutionScores, qualityScore)

		// 错误恢复评估
		if result.RetryCount > 0 && result.Status == swe_bench.StatusCompleted {
			errorRecoveryCount++
		}

		// 一致性评估（基于解决方案长度和复杂性）
		consistencyScore := qc.assessConsistency(result)
		consistencyScores = append(consistencyScores, consistencyScore)
	}

	metrics := QualityMetrics{
		SolutionQuality:    average(solutionScores),
		ErrorRecoveryRate:  float64(errorRecoveryCount) / float64(len(results)),
		ConsistencyScore:   average(consistencyScores),
		ComplexityHandling: qc.assessComplexityHandling(results),
	}

	return metrics, nil
}

// assessSolutionQuality 评估解决方案质量
func (qc *QualityCollector) assessSolutionQuality(result swe_bench.WorkerResult) float64 {
	if result.Status != swe_bench.StatusCompleted {
		return 0.0
	}

	// 基于简单规则的质量评分
	score := 0.5 // 基础分

	// 有解决方案说明加分
	if len(result.Explanation) > 100 {
		score += 0.2
	}

	// 文件修改合理性
	if len(result.FilesChanged) > 0 && len(result.FilesChanged) < 10 {
		score += 0.2
	}

	// 解决方案不为空
	if len(result.Solution) > 50 {
		score += 0.1
	}

	// Workflow-level failure signals should heavily cap quality.
	if workflowFailureSignal(result) {
		score = math.Min(score, 0.25)
	}

	// Completed-with-error should not appear high quality.
	if strings.TrimSpace(result.Error) != "" {
		score = math.Min(score, 0.35)
	}

	return math.Min(score, 1.0)
}

// assessConsistency 评估一致性
func (qc *QualityCollector) assessConsistency(result swe_bench.WorkerResult) float64 {
	// 基于解决方案特征的简单一致性评估
	if result.Status != swe_bench.StatusCompleted {
		return 0.0
	}

	if workflowFailureSignal(result) {
		return 0.2
	}

	// 基于解决方案长度和复杂性的评估
	solutionLen := len(result.Solution)
	filesChanged := len(result.FilesChanged)

	// 合理的解决方案长度范围
	if solutionLen >= 50 && solutionLen <= 5000 && filesChanged > 0 && filesChanged <= 5 {
		return 0.8
	}
	return 0.4
}

func workflowFailureSignal(result swe_bench.WorkerResult) bool {
	if strings.EqualFold(strings.TrimSpace(result.ErrorType), "max_iterations_error") {
		return true
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(result.Error)), "max iterations") {
		return true
	}
	if result.Workflow == nil {
		return false
	}
	if result.Workflow.Phase == workflow.PhaseFailed {
		return true
	}
	for _, node := range result.Workflow.Nodes {
		if node.ID != "execute" {
			continue
		}
		output, ok := node.Output.(map[string]any)
		if !ok {
			continue
		}
		stop, ok := output["stop"].(string)
		if ok && strings.EqualFold(strings.TrimSpace(stop), "max_iterations") {
			return true
		}
	}
	return false
}

// assessComplexityHandling 评估复杂性处理能力
func (qc *QualityCollector) assessComplexityHandling(results []swe_bench.WorkerResult) float64 {
	var complexTasks, successfulComplexTasks int

	for _, result := range results {
		// 简单的复杂性判断（基于任务描述长度）
		if len(result.InstanceID) > 50 { // 假设长ID代表复杂任务
			complexTasks++
			if result.Status == swe_bench.StatusCompleted {
				successfulComplexTasks++
			}
		}
	}

	if complexTasks == 0 {
		return 0.5 // 无复杂任务时的默认分数
	}

	return float64(successfulComplexTasks) / float64(complexTasks)
}

// ResourceCollector 资源指标收集器
type ResourceCollector struct{}

func (rc *ResourceCollector) Name() string { return "resource" }

func (rc *ResourceCollector) Collect(results []swe_bench.WorkerResult) (interface{}, error) {
	var totalTokens int
	var totalCost float64

	for _, result := range results {
		totalTokens += result.TokensUsed
		totalCost += result.Cost
	}

	metrics := ResourceMetrics{
		TotalTokens: totalTokens,
		TotalCost:   totalCost,
		MemoryUsage: 50, // 简化的固定值，实际应该从系统获取
	}

	if len(results) > 0 {
		metrics.AvgTokensUsed = totalTokens / len(results)
		metrics.AvgCostPerTask = totalCost / float64(len(results))
	}

	return metrics, nil
}

// BehaviorCollector 行为指标收集器
type BehaviorCollector struct{}

func (bc *BehaviorCollector) Name() string { return "behavior" }

func (bc *BehaviorCollector) Collect(results []swe_bench.WorkerResult) (interface{}, error) {
	toolUsage := make(map[string]int)
	failures := make(map[string]int)
	var errorPatterns []string
	var totalToolCalls int

	for _, result := range results {
		// 工具调用统计（基于命令）
		toolCallCount := len(result.Commands)
		totalToolCalls += toolCallCount

		for _, cmd := range result.Commands {
			if len(cmd) > 0 {
				// 提取工具名（假设命令格式为 "tool_name args..."）
				toolName := extractToolName(cmd)
				toolUsage[toolName]++
			}
		}

		// 失败模式统计
		if result.Status != swe_bench.StatusCompleted {
			failureType := string(result.Status)
			failures[failureType]++

			if result.Error != "" {
				errorPatterns = append(errorPatterns, result.ErrorType)
			}
		}
	}

	var avgToolCalls float64
	if len(results) > 0 {
		avgToolCalls = float64(totalToolCalls) / float64(len(results))
	}

	metrics := BehaviorMetrics{
		AvgToolCalls:     avgToolCalls,
		ToolUsagePattern: toolUsage,
		CommonFailures:   failures,
		ErrorPatterns:    uniqueStrings(errorPatterns),
	}

	return metrics, nil
}

// SimpleMetricsStore 简单的指标存储
type SimpleMetricsStore struct {
	basePath string
}

// NewSimpleMetricsStore 创建简单指标存储
func NewSimpleMetricsStore(basePath string) *SimpleMetricsStore {
	return &SimpleMetricsStore{
		basePath: basePath,
	}
}

// Store 存储指标
func (sms *SimpleMetricsStore) Store(evaluationID string, metrics *EvaluationMetrics) error {
	if err := os.MkdirAll(sms.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create metrics directory: %w", err)
	}

	filename := filepath.Join(sms.basePath, fmt.Sprintf("%s_%d.json", evaluationID, time.Now().Unix()))

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write metrics file: %w", err)
	}

	return nil
}

// Load 加载指标
func (sms *SimpleMetricsStore) Load(evaluationID string) (*EvaluationMetrics, error) {
	pattern := filepath.Join(sms.basePath, fmt.Sprintf("%s_*.json", evaluationID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob metrics files: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no metrics found for evaluation %s", evaluationID)
	}

	// 使用最新的文件
	latestFile := matches[len(matches)-1]

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics file: %w", err)
	}

	var metrics EvaluationMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return &metrics, nil
}

// Utility functions

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func extractToolName(command string) string {
	// 简化的工具名提取
	if len(command) == 0 {
		return "unknown"
	}

	// 假设命令格式为 "tool_name args..."
	parts := []rune(command)
	for i, char := range parts {
		if char == ' ' {
			return command[:i]
		}
	}
	return command
}

func uniqueStrings(strs []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, str := range strs {
		if !keys[str] {
			keys[str] = true
			result = append(result, str)
		}
	}

	return result
}
