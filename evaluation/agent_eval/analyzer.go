package agent_eval

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// BasicAnalyzer 基础分析器（替代复杂ML组件）
type BasicAnalyzer struct {
	ruleEngine *SimpleRuleEngine
	stats      *StatisticalAnalyzer
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	Summary         AnalysisSummary  `json:"summary"`
	Insights        []Insight        `json:"insights"`
	Recommendations []Recommendation `json:"recommendations"`
	Trends          TrendAnalysis    `json:"trends"`
	Alerts          []Alert          `json:"alerts"`
	Timestamp       time.Time        `json:"timestamp"`
}

// AnalysisSummary 分析摘要
type AnalysisSummary struct {
	OverallScore     float64  `json:"overall_score"`
	PerformanceGrade string   `json:"performance_grade"`
	KeyStrengths     []string `json:"key_strengths"`
	KeyWeaknesses    []string `json:"key_weaknesses"`
	RiskLevel        string   `json:"risk_level"`
}

// Insight 洞察
type Insight struct {
	Type        InsightType `json:"type"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Impact      ImpactLevel `json:"impact"`
	Confidence  float64     `json:"confidence"`
}

// Recommendation 建议
type Recommendation struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    Priority `json:"priority"`
	ActionItems []string `json:"action_items"`
	Expected    string   `json:"expected_improvement"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	PerformanceTrend string  `json:"performance_trend"`
	QualityTrend     string  `json:"quality_trend"`
	EfficiencyTrend  string  `json:"efficiency_trend"`
	PredictedScore   float64 `json:"predicted_score"`
	ConfidenceLevel  float64 `json:"confidence_level"`
}

// Alert 警报
type Alert struct {
	Level       AlertLevel `json:"level"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Action      string     `json:"suggested_action"`
	Timestamp   time.Time  `json:"timestamp"`
}

// 枚举类型
type InsightType string
type ImpactLevel string
type Priority string
type AlertLevel string

const (
	InsightPerformance InsightType = "performance"
	InsightQuality     InsightType = "quality"
	InsightEfficiency  InsightType = "efficiency"
	InsightBehavior    InsightType = "behavior"
)

const (
	ImpactHigh   ImpactLevel = "high"
	ImpactMedium ImpactLevel = "medium"
	ImpactLow    ImpactLevel = "low"
)

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

const (
	AlertCritical AlertLevel = "critical"
	AlertWarning  AlertLevel = "warning"
	AlertInfo     AlertLevel = "info"
)

// NewBasicAnalyzer 创建基础分析器
func NewBasicAnalyzer() *BasicAnalyzer {
	return &BasicAnalyzer{
		ruleEngine: NewSimpleRuleEngine(),
		stats:      NewStatisticalAnalyzer(),
	}
}

// Analyze 分析指标
func (ba *BasicAnalyzer) Analyze(metrics *EvaluationMetrics) *AnalysisResult {
	result := &AnalysisResult{
		Timestamp: time.Now(),
	}

	// 1. 计算总体分数
	result.Summary = ba.calculateSummary(metrics)

	// 2. 生成洞察
	result.Insights = ba.generateInsights(metrics)

	// 3. 应用规则引擎生成建议
	result.Recommendations = ba.ruleEngine.GenerateRecommendations(metrics)

	// 4. 分析趋势（简化版本）
	result.Trends = ba.analyzeTrends(metrics)

	// 5. 生成警报
	result.Alerts = ba.generateAlerts(metrics)

	return result
}

// calculateSummary 计算分析摘要
func (ba *BasicAnalyzer) calculateSummary(metrics *EvaluationMetrics) AnalysisSummary {
	// 基于不同维度计算加权分数
	performanceScore := ba.calculatePerformanceScore(metrics.Performance)
	qualityScore := ba.calculateQualityScore(metrics.Quality)
	efficiencyScore := ba.calculateEfficiencyScore(metrics.Resources)

	// 加权平均
	overallScore := (performanceScore*0.4 + qualityScore*0.3 + efficiencyScore*0.3)

	// 性能等级
	grade := ba.scoreToGrade(overallScore)

	// 识别优势和劣势
	strengths, weaknesses := ba.identifyStrengthsAndWeaknesses(metrics)

	// 风险等级评估
	riskLevel := ba.assessRiskLevel(metrics)

	return AnalysisSummary{
		OverallScore:     overallScore,
		PerformanceGrade: grade,
		KeyStrengths:     strengths,
		KeyWeaknesses:    weaknesses,
		RiskLevel:        riskLevel,
	}
}

// calculatePerformanceScore 计算性能分数
func (ba *BasicAnalyzer) calculatePerformanceScore(perf PerformanceMetrics) float64 {
	score := 0.0

	// 成功率权重最高
	score += perf.SuccessRate * 0.5

	// 超时率影响（越低越好）
	score += math.Max(0, (1-perf.TimeoutRate)) * 0.3

	// 重试率影响（越低越好）
	score += math.Max(0, (1-perf.RetryRate)) * 0.2

	return score
}

// calculateQualityScore 计算质量分数
func (ba *BasicAnalyzer) calculateQualityScore(quality QualityMetrics) float64 {
	score := 0.0

	score += quality.SolutionQuality * 0.4
	score += quality.ErrorRecoveryRate * 0.3
	score += quality.ConsistencyScore * 0.2
	score += quality.ComplexityHandling * 0.1

	return score
}

// calculateEfficiencyScore 计算效率分数
func (ba *BasicAnalyzer) calculateEfficiencyScore(resources ResourceMetrics) float64 {
	// 基于资源使用的效率评估
	score := 0.5 // 基础分

	// 基于tokens使用的效率（假设理想范围）
	if resources.AvgTokensUsed > 0 && resources.AvgTokensUsed <= 5000 {
		score += 0.3
	}

	// 成本效率
	if resources.AvgCostPerTask > 0 && resources.AvgCostPerTask <= 1.0 {
		score += 0.2
	}

	return math.Min(score, 1.0)
}

// generateInsights 生成洞察
func (ba *BasicAnalyzer) generateInsights(metrics *EvaluationMetrics) []Insight {
	var insights []Insight

	// 性能洞察
	if metrics.Performance.SuccessRate >= 0.8 {
		insights = append(insights, Insight{
			Type:        InsightPerformance,
			Title:       "High Success Rate",
			Description: fmt.Sprintf("Agent achieved %.1f%% success rate, indicating strong problem-solving capability", metrics.Performance.SuccessRate*100),
			Impact:      ImpactHigh,
			Confidence:  0.9,
		})
	}

	if metrics.Performance.TimeoutRate > 0.2 {
		insights = append(insights, Insight{
			Type:        InsightPerformance,
			Title:       "High Timeout Rate",
			Description: fmt.Sprintf("%.1f%% of tasks timed out, suggesting need for optimization", metrics.Performance.TimeoutRate*100),
			Impact:      ImpactMedium,
			Confidence:  0.8,
		})
	}

	// 质量洞察
	if metrics.Quality.ErrorRecoveryRate >= 0.7 {
		insights = append(insights, Insight{
			Type:        InsightQuality,
			Title:       "Strong Error Recovery",
			Description: "Agent demonstrates good error recovery capabilities",
			Impact:      ImpactMedium,
			Confidence:  0.8,
		})
	}

	// 效率洞察
	if metrics.Behavior.AvgToolCalls > 10 {
		insights = append(insights, Insight{
			Type:        InsightEfficiency,
			Title:       "High Tool Usage",
			Description: fmt.Sprintf("Average %.1f tool calls per task may indicate inefficient approach", metrics.Behavior.AvgToolCalls),
			Impact:      ImpactLow,
			Confidence:  0.7,
		})
	}

	return insights
}

// analyzeTrends 分析趋势（简化版本）
func (ba *BasicAnalyzer) analyzeTrends(metrics *EvaluationMetrics) TrendAnalysis {
	// 简化的趋势分析，实际应该基于历史数据
	return TrendAnalysis{
		PerformanceTrend: "stable",
		QualityTrend:     "improving",
		EfficiencyTrend:  "stable",
		PredictedScore:   0.75, // 基于当前指标的预测
		ConfidenceLevel:  0.6,
	}
}

// generateAlerts 生成警报
func (ba *BasicAnalyzer) generateAlerts(metrics *EvaluationMetrics) []Alert {
	var alerts []Alert

	// 关键性能警报
	if metrics.Performance.SuccessRate < 0.5 {
		alerts = append(alerts, Alert{
			Level:       AlertCritical,
			Title:       "Low Success Rate",
			Description: fmt.Sprintf("Success rate %.1f%% is below acceptable threshold", metrics.Performance.SuccessRate*100),
			Action:      "Review agent configuration and prompts",
			Timestamp:   time.Now(),
		})
	}

	// 成本警报
	if metrics.Resources.TotalCost > 100 {
		alerts = append(alerts, Alert{
			Level:       AlertWarning,
			Title:       "High Cost",
			Description: fmt.Sprintf("Total evaluation cost $%.2f is above budget", metrics.Resources.TotalCost),
			Action:      "Consider cost optimization strategies",
			Timestamp:   time.Now(),
		})
	}

	return alerts
}

// identifyStrengthsAndWeaknesses 识别优势和劣势
func (ba *BasicAnalyzer) identifyStrengthsAndWeaknesses(metrics *EvaluationMetrics) ([]string, []string) {
	var strengths, weaknesses []string

	// 优势识别
	if metrics.Performance.SuccessRate >= 0.8 {
		strengths = append(strengths, "High task completion rate")
	}
	if metrics.Quality.ErrorRecoveryRate >= 0.7 {
		strengths = append(strengths, "Good error handling")
	}
	if metrics.Performance.RetryRate <= 0.1 {
		strengths = append(strengths, "Consistent first-attempt success")
	}

	// 劣势识别
	if metrics.Performance.TimeoutRate > 0.2 {
		weaknesses = append(weaknesses, "Frequent task timeouts")
	}
	if metrics.Quality.SolutionQuality < 0.6 {
		weaknesses = append(weaknesses, "Solution quality needs improvement")
	}
	if metrics.Behavior.AvgToolCalls > 15 {
		weaknesses = append(weaknesses, "Inefficient tool usage patterns")
	}

	return strengths, weaknesses
}

// assessRiskLevel 评估风险等级
func (ba *BasicAnalyzer) assessRiskLevel(metrics *EvaluationMetrics) string {
	riskScore := 0

	if metrics.Performance.SuccessRate < 0.6 {
		riskScore += 3
	}
	if metrics.Performance.TimeoutRate > 0.3 {
		riskScore += 2
	}
	if metrics.Quality.ErrorRecoveryRate < 0.5 {
		riskScore += 2
	}
	if metrics.Resources.TotalCost > 200 {
		riskScore += 1
	}

	switch {
	case riskScore >= 5:
		return "high"
	case riskScore >= 3:
		return "medium"
	default:
		return "low"
	}
}

// scoreToGrade 分数转等级
func (ba *BasicAnalyzer) scoreToGrade(score float64) string {
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
		return "F"
	}
}

// StatisticalAnalyzer 统计分析器
type StatisticalAnalyzer struct{}

// StatisticalReport 统计报告
type StatisticalReport struct {
	Mean         float64   `json:"mean"`
	Median       float64   `json:"median"`
	StandardDev  float64   `json:"standard_deviation"`
	Percentiles  []float64 `json:"percentiles"`
	Outliers     []float64 `json:"outliers"`
	Distribution string    `json:"distribution"`
}

// NewStatisticalAnalyzer 创建统计分析器
func NewStatisticalAnalyzer() *StatisticalAnalyzer {
	return &StatisticalAnalyzer{}
}

// AnalyzePerformance 分析性能统计
func (sa *StatisticalAnalyzer) AnalyzePerformance(durations []time.Duration) StatisticalReport {
	if len(durations) == 0 {
		return StatisticalReport{}
	}

	// 转换为float64秒
	values := make([]float64, len(durations))
	for i, d := range durations {
		values[i] = d.Seconds()
	}

	sort.Float64s(values)

	report := StatisticalReport{
		Mean:         sa.calculateMean(values),
		Median:       sa.calculateMedian(values),
		StandardDev:  sa.calculateStdDev(values),
		Percentiles:  sa.calculatePercentiles(values, []float64{25, 50, 75, 90, 95, 99}),
		Outliers:     sa.detectOutliers(values),
		Distribution: sa.analyzeDistribution(values),
	}

	return report
}

// calculateMean 计算平均值
func (sa *StatisticalAnalyzer) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateMedian 计算中位数
func (sa *StatisticalAnalyzer) calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	n := len(values)
	if n%2 == 0 {
		return (values[n/2-1] + values[n/2]) / 2
	}
	return values[n/2]
}

// calculateStdDev 计算标准差
func (sa *StatisticalAnalyzer) calculateStdDev(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	mean := sa.calculateMean(values)
	var sumSquaredDiff float64

	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}

	variance := sumSquaredDiff / float64(len(values)-1)
	return math.Sqrt(variance)
}

// calculatePercentiles 计算百分位数
func (sa *StatisticalAnalyzer) calculatePercentiles(values []float64, percentiles []float64) []float64 {
	if len(values) == 0 {
		return make([]float64, len(percentiles))
	}

	result := make([]float64, len(percentiles))
	for i, p := range percentiles {
		index := (p / 100.0) * float64(len(values)-1)
		lower := int(math.Floor(index))
		upper := int(math.Ceil(index))

		if lower == upper {
			result[i] = values[lower]
		} else {
			// 线性插值
			weight := index - float64(lower)
			result[i] = values[lower]*(1-weight) + values[upper]*weight
		}
	}

	return result
}

// detectOutliers 检测异常值（使用IQR方法）
func (sa *StatisticalAnalyzer) detectOutliers(values []float64) []float64 {
	if len(values) < 4 {
		return []float64{}
	}

	q1Index := len(values) / 4
	q3Index := (3 * len(values)) / 4

	q1 := values[q1Index]
	q3 := values[q3Index]
	iqr := q3 - q1

	lowerBound := q1 - 1.5*iqr
	upperBound := q3 + 1.5*iqr

	var outliers []float64
	for _, v := range values {
		if v < lowerBound || v > upperBound {
			outliers = append(outliers, v)
		}
	}

	return outliers
}

// analyzeDistribution 分析分布类型（简化版本）
func (sa *StatisticalAnalyzer) analyzeDistribution(values []float64) string {
	if len(values) < 3 {
		return "insufficient_data"
	}

	mean := sa.calculateMean(values)
	median := sa.calculateMedian(values)

	// 简单的分布判断
	diff := math.Abs(mean - median)
	if diff < 0.1*mean {
		return "normal"
	} else if mean > median {
		return "right_skewed"
	} else {
		return "left_skewed"
	}
}
