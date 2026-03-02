//go:build ignore
package evolution

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// PerformanceAnalyzer 分析任务执行性能并生成改进建议
type PerformanceAnalyzer struct {
	history *EvolutionHistory
}

// NewPerformanceAnalyzer 创建性能分析器
func NewPerformanceAnalyzer(history *EvolutionHistory) *PerformanceAnalyzer {
	return &PerformanceAnalyzer{history: history}
}

// AnalyzeExecution 分析单次执行结果
func (pa *PerformanceAnalyzer) AnalyzeExecution(ctx context.Context, record ExecutionRecord) (*PerformanceAnalysis, error) {
	analysis := &PerformanceAnalysis{
		RecordID:    record.ID,
		AnalyzedAt:  time.Now(),
		TaskType:    record.TaskType,
		Duration:    record.EndTime.Sub(record.StartTime),
		IterationCount: record.IterationCount,
		TokenUsage:  record.TokenUsage,
		ToolCalls:   len(record.ToolCalls),
		Success:     record.Success,
		Errors:      record.Errors,
	}

	// 计算效率分数 (0-100)
	efficiencyScore := pa.calculateEfficiencyScore(analysis)
	analysis.EfficiencyScore = efficiencyScore

	// 生成改进建议
	suggestions := pa.generateImprovementSuggestions(record, analysis)
	analysis.ImprovementSuggestions = suggestions

	// 识别成功模式
	if record.Success && efficiencyScore >= 80 {
		patterns := pa.extractSuccessPatterns(record)
		analysis.SuccessPatterns = patterns
	}

	// 识别失败模式
	if !record.Success || efficiencyScore < 50 {
		patterns := pa.extractFailurePatterns(record)
		analysis.FailurePatterns = patterns
	}

	return analysis, nil
}

// CompareWithBaseline 与基线性能对比
func (pa *PerformanceAnalyzer) CompareWithBaseline(analysis *PerformanceAnalysis, baseline *PerformanceBaseline) (*PerformanceDelta, error) {
	if baseline == nil {
		return nil, fmt.Errorf("baseline is nil")
	}

	delta := &PerformanceDelta{
		TaskType:    analysis.TaskType,
		ComparedAt:  time.Now(),
		BaselineID:  baseline.ID,
	}

	// 计算各项指标差异
	delta.DurationDelta = calculatePercentageDelta(
		float64(analysis.Duration.Milliseconds()),
		float64(baseline.AvgDuration.Milliseconds()),
	)
	delta.TokenDelta = calculatePercentageDelta(
		float64(analysis.TokenUsage),
		float64(baseline.AvgTokenUsage),
	)
	delta.IterationDelta = calculatePercentageDelta(
		float64(analysis.IterationCount),
		float64(baseline.AvgIterations),
	)
	delta.EfficiencyDelta = analysis.EfficiencyScore - baseline.AvgEfficiencyScore

	// 判断趋势
	if delta.EfficiencyDelta > 10 {
		delta.Trend = TrendImproving
	} else if delta.EfficiencyDelta < -10 {
		delta.Trend = TrendDegrading
	} else {
		delta.Trend = TrendStable
	}

	return delta, nil
}

// calculateEfficiencyScore 计算执行效率分数
func (pa *PerformanceAnalyzer) calculateEfficiencyScore(analysis *PerformanceAnalysis) float64 {
	score := 100.0

	// 基于迭代次数扣分 (每多一次迭代扣5分，最多扣30分)
	iterationPenalty := math.Min(float64(analysis.IterationCount-1)*5, 30)
	score -= iterationPenalty

	// 基于token使用量扣分 (每1000 token扣1分，最多扣20分)
	tokenPenalty := math.Min(float64(analysis.TokenUsage)/1000, 20)
	score -= tokenPenalty

	// 基于工具调用次数扣分 (每多一个工具调用扣2分，最多扣20分)
	toolPenalty := math.Min(float64(analysis.ToolCalls)*2, 20)
	score -= toolPenalty

	// 如果有错误，根据错误严重程度扣分
	if len(analysis.Errors) > 0 {
		for _, err := range analysis.Errors {
			if isCriticalError(err) {
				score -= 15
			} else {
				score -= 5
			}
		}
	}

	// 基于任务类型调整基准分
	switch analysis.TaskType {
	case TaskTypeSimple:
		// 简单任务应该有更高要求
		if analysis.IterationCount > 2 {
			score -= 10
		}
		if analysis.ToolCalls > 2 {
			score -= 5
		}
	case TaskTypeComplex:
		// 复杂任务适当放宽
		score += 5
	}

	return math.Max(0, math.Min(100, score))
}

// generateImprovementSuggestions 生成改进建议
func (pa *PerformanceAnalyzer) generateImprovementSuggestions(record ExecutionRecord, analysis *PerformanceAnalysis) []ImprovementSuggestion {
	var suggestions []ImprovementSuggestion

	// 基于迭代次数的建议
	if analysis.IterationCount > 5 {
		suggestions = append(suggestions, ImprovementSuggestion{
			Category:    SuggestionCategoryPlanning,
			Priority:    PriorityHigh,
			Description: "任务迭代次数过多，建议在任务分析阶段生成更详细的执行计划",
			Action:      "优化任务分解策略，增加前置分析深度",
		})
	}

	// 基于token使用量的建议
	if analysis.TokenUsage > 5000 {
		suggestions = append(suggestions, ImprovementSuggestion{
			Category:    SuggestionCategoryContext,
			Priority:    PriorityMedium,
			Description: "Token使用量较高，可能存在上下文冗余",
			Action:      "启用更激进的上下文压缩策略",
		})
	}

	// 基于工具调用次数的建议
	if analysis.ToolCalls > 10 {
		suggestions = append(suggestions, ImprovementSuggestion{
			Category:    SuggestionCategoryTools,
			Priority:    PriorityMedium,
			Description: "工具调用次数过多，可能存在重复调用或低效调用",
			Action:      "添加工具调用缓存机制，优化工具选择策略",
		})
	}

	// 基于错误的建议
	for _, err := range analysis.Errors {
		if strings.Contains(err, "timeout") || strings.Contains(err, "deadline") {
			suggestions = append(suggestions, ImprovementSuggestion{
				Category:    SuggestionCategoryExecution,
				Priority:    PriorityHigh,
				Description: "存在超时错误，需要优化任务执行策略",
				Action:      "增加任务分段执行支持，优化超时处理",
			})
		}
		if strings.Contains(err, "rate limit") || strings.Contains(err, "quota") {
			suggestions = append(suggestions, ImprovementSuggestion{
				Category:    SuggestionCategoryExecution,
				Priority:    PriorityHigh,
				Description: "触发API限流，需要优化调用频率",
				Action:      "实现指数退避重试机制，优化批处理策略",
			})
		}
	}

	// 基于执行时间的建议
	if analysis.Duration > 2*time.Minute {
		suggestions = append(suggestions, ImprovementSuggestion{
			Category:    SuggestionCategoryExecution,
			Priority:    PriorityLow,
			Description: "任务执行时间较长",
			Action:      "考虑异步执行或流式输出以提升用户体验",
		})
	}

	return suggestions
}

// extractSuccessPatterns 提取成功模式
func (pa *PerformanceAnalyzer) extractSuccessPatterns(record ExecutionRecord) []string {
	var patterns []string

	// 分析成功执行的特征
	if record.IterationCount <= 3 {
		patterns = append(patterns, "高效迭代: 3次迭代内完成任务")
	}

	if len(record.ToolCalls) <= 3 {
		patterns = append(patterns, "精准工具选择: 使用少量工具达成目标")
	}

	if record.TokenUsage < 2000 {
		patterns = append(patterns, "高效token使用: 保持较低的上下文开销")
	}

	return patterns
}

// extractFailurePatterns 提取失败模式
func (pa *PerformanceAnalyzer) extractFailurePatterns(record ExecutionRecord) []string {
	var patterns []string

	// 分析失败的共同特征
	if record.IterationCount >= 10 {
		patterns = append(patterns, "迭代陷阱: 过多迭代未能收敛")
	}

	for _, err := range record.Errors {
		if strings.Contains(err, "tool") {
			patterns = append(patterns, "工具调用问题: 存在工具执行错误")
			break
		}
	}

	return patterns
}

// isCriticalError 判断是否为严重错误
func isCriticalError(err string) bool {
	criticalPatterns := []string{
		"panic", "fatal", "crash",
		"out of memory", "oom",
		"deadlock", "race condition",
	}
	lowerErr := strings.ToLower(err)
	for _, pattern := range criticalPatterns {
		if strings.Contains(lowerErr, pattern) {
			return true
		}
	}
	return false
}

// calculatePercentageDelta 计算百分比差异
func calculatePercentageDelta(current, baseline float64) float64 {
	if baseline == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return ((current - baseline) / baseline) * 100
}

// AnalyzeTrend 分析性能趋势
func (pa *PerformanceAnalyzer) AnalyzeTrend(taskType string, windowSize int) (*PerformanceTrend, error) {
	records := pa.history.GetRecentRecords(windowSize)
	if len(records) == 0 {
		return nil, fmt.Errorf("no records found")
	}

	var taskRecords []ExecutionRecord
	for _, r := range records {
		if r.TaskType == taskType || taskType == "" {
			taskRecords = append(taskRecords, r)
		}
	}

	if len(taskRecords) < 2 {
		return nil, fmt.Errorf("insufficient data for trend analysis")
	}

	trend := &PerformanceTrend{
		TaskType:   taskType,
		WindowSize: len(taskRecords),
		StartTime:  taskRecords[0].StartTime,
		EndTime:    taskRecords[len(taskRecords)-1].EndTime,
	}

	// 计算平均值
	var totalEfficiency, totalDuration, totalTokens, totalIterations float64
	for _, r := range taskRecords {
		duration := r.EndTime.Sub(r.StartTime).Milliseconds()
		totalDuration += float64(duration)
		totalTokens += float64(r.TokenUsage)
		totalIterations += float64(r.IterationCount)
	}

	count := float64(len(taskRecords))
	trend.AvgDurationMs = totalDuration / count
	trend.AvgTokenUsage = int(totalTokens / count)
	trend.AvgIterations = totalIterations / count

	// 计算效率分数趋势
	firstHalf := taskRecords[:len(taskRecords)/2]
	secondHalf := taskRecords[len(taskRecords)/2:]

	firstHalfScore := calculateAvgEfficiency(firstHalf)
	secondHalfScore := calculateAvgEfficiency(secondHalf)

	if secondHalfScore > firstHalfScore+5 {
		trend.Direction = TrendImproving
	} else if secondHalfScore < firstHalfScore-5 {
		trend.Direction = TrendDegrading
	} else {
		trend.Direction = TrendStable
	}

	trend.EfficiencyChange = secondHalfScore - firstHalfScore

	return trend, nil
}

func calculateAvgEfficiency(records []ExecutionRecord) float64 {
	if len(records) == 0 {
		return 0
	}
	var total float64
	for _, r := range records {
		// 简化的效率计算
		duration := r.EndTime.Sub(r.StartTime).Seconds()
		score := 100.0
		score -= float64(r.IterationCount) * 5
		score -= duration * 2
		score -= float64(r.TokenUsage) / 100
		if score < 0 {
			score = 0
		}
		if r.Success {
			score += 20
		}
		total += score
	}
	return total / float64(len(records))
}
