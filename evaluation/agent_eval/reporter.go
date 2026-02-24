package agent_eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/evaluation/swe_bench"
)

// MarkdownReporter Markdown报告生成器
type MarkdownReporter struct {
	// templatePath string // Reserved for future template support
	// outputDir    string // Reserved for future output directory support
}

// NewMarkdownReporter 创建Markdown报告生成器
func NewMarkdownReporter() *MarkdownReporter {
	return &MarkdownReporter{}
}

// GenerateReport 生成评估报告
func (mr *MarkdownReporter) GenerateReport(results *EvaluationResults, outputPath string, baseDir string) error {
	cleanedPath, err := sanitizeOutputPath(baseDir, outputPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cleanedPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	report := mr.buildReportContent(results)

	if err := os.WriteFile(cleanedPath, []byte(report), 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// buildReportContent 构建报告内容
func (mr *MarkdownReporter) buildReportContent(results *EvaluationResults) string {
	var report strings.Builder

	// Header
	report.WriteString(mr.buildHeader(results))
	report.WriteString("\n")

	// Executive Summary
	report.WriteString(mr.buildExecutiveSummary(results))
	report.WriteString("\n")

	// Performance Analysis
	report.WriteString(mr.buildPerformanceSection(results))
	report.WriteString("\n")

	// Quality Analysis
	report.WriteString(mr.buildQualitySection(results))
	report.WriteString("\n")

	// Resource Usage
	report.WriteString(mr.buildResourceSection(results))
	report.WriteString("\n")

	// Behavior Analysis
	report.WriteString(mr.buildBehaviorSection(results))
	report.WriteString("\n")

	// Attention Analysis
	report.WriteString(mr.buildAttentionSection(results))
	report.WriteString("\n")

	// Insights and Recommendations
	report.WriteString(mr.buildInsightsSection(results))
	report.WriteString("\n")

	// Detailed Recommendations
	report.WriteString(mr.buildRecommendationsSection(results))
	report.WriteString("\n")

	// Alerts
	if len(results.Analysis.Alerts) > 0 {
		report.WriteString(mr.buildAlertsSection(results))
		report.WriteString("\n")
	}

	// Task Results Summary
	report.WriteString(mr.buildTaskResultsSummary(results))
	report.WriteString("\n")

	// Footer
	report.WriteString(mr.buildFooter(results))

	return report.String()
}

// buildHeader 构建报告头部
func (mr *MarkdownReporter) buildHeader(results *EvaluationResults) string {
	return fmt.Sprintf(`# Agent Evaluation Report

**Evaluation ID:** %s  
**Generated:** %s  
**Total Tasks:** %d  
**Overall Score:** %.1f%% (%s)

---

`, results.JobID,
		results.Timestamp.Format("2006-01-02 15:04:05"),
		results.Metrics.TotalTasks,
		results.Analysis.Summary.OverallScore*100,
		results.Analysis.Summary.PerformanceGrade)
}

// buildExecutiveSummary 构建执行摘要
func (mr *MarkdownReporter) buildExecutiveSummary(results *EvaluationResults) string {
	summary := results.Analysis.Summary

	var report strings.Builder
	report.WriteString("## Executive Summary\n\n")

	// Overall Assessment
	report.WriteString(fmt.Sprintf("The agent achieved an overall score of **%.1f%%** with a grade of **%s**. ",
		summary.OverallScore*100, summary.PerformanceGrade))

	report.WriteString(fmt.Sprintf("Risk level is assessed as **%s**.\n\n", summary.RiskLevel))

	// Key Strengths
	if len(summary.KeyStrengths) > 0 {
		report.WriteString("### Key Strengths\n")
		for _, strength := range summary.KeyStrengths {
			report.WriteString(fmt.Sprintf("- %s\n", strength))
		}
		report.WriteString("\n")
	}

	// Key Weaknesses
	if len(summary.KeyWeaknesses) > 0 {
		report.WriteString("### Areas for Improvement\n")
		for _, weakness := range summary.KeyWeaknesses {
			report.WriteString(fmt.Sprintf("- %s\n", weakness))
		}
		report.WriteString("\n")
	}

	return report.String()
}

// buildPerformanceSection 构建性能分析部分
func (mr *MarkdownReporter) buildPerformanceSection(results *EvaluationResults) string {
	perf := results.Metrics.Performance

	return fmt.Sprintf(`## Performance Analysis

| Metric | Value | Assessment |
|--------|-------|------------|
| **Success Rate** | %.1f%% | %s |
| **Average Execution Time** | %s | %s |
| **Median Execution Time** | %s | %s |
| **95th Percentile Time** | %s | %s |
| **Timeout Rate** | %.1f%% | %s |
| **Retry Rate** | %.1f%% | %s |

### Performance Insights

%s

`,
		perf.SuccessRate*100, mr.assessSuccessRate(perf.SuccessRate),
		mr.formatDuration(perf.AvgExecutionTime), mr.assessExecutionTime(perf.AvgExecutionTime),
		mr.formatDuration(perf.MedianTime), mr.assessExecutionTime(perf.MedianTime),
		mr.formatDuration(perf.P95Time), mr.assessExecutionTime(perf.P95Time),
		perf.TimeoutRate*100, mr.assessTimeoutRate(perf.TimeoutRate),
		perf.RetryRate*100, mr.assessRetryRate(perf.RetryRate),
		mr.generatePerformanceInsights(perf))
}

// buildQualitySection 构建质量分析部分
func (mr *MarkdownReporter) buildQualitySection(results *EvaluationResults) string {
	quality := results.Metrics.Quality

	return fmt.Sprintf(`## Quality Analysis

| Metric | Value | Assessment |
|--------|-------|------------|
| **Solution Quality** | %.1f%% | %s |
| **Error Recovery Rate** | %.1f%% | %s |
| **Consistency Score** | %.1f%% | %s |
| **Complexity Handling** | %.1f%% | %s |

### Quality Insights

%s

`,
		quality.SolutionQuality*100, mr.assessQualityScore(quality.SolutionQuality),
		quality.ErrorRecoveryRate*100, mr.assessErrorRecovery(quality.ErrorRecoveryRate),
		quality.ConsistencyScore*100, mr.assessConsistency(quality.ConsistencyScore),
		quality.ComplexityHandling*100, mr.assessComplexityHandling(quality.ComplexityHandling),
		mr.generateQualityInsights(quality))
}

// buildResourceSection 构建资源使用部分
func (mr *MarkdownReporter) buildResourceSection(results *EvaluationResults) string {
	resources := results.Metrics.Resources

	return fmt.Sprintf(`## Resource Usage

| Metric | Value | Assessment |
|--------|-------|------------|
| **Total Tokens Used** | %s | %s |
| **Average Tokens per Task** | %s | %s |
| **Total Cost** | $%.2f | %s |
| **Average Cost per Task** | $%.2f | %s |
| **Memory Usage** | %d MB | %s |

### Resource Insights

%s

`,
		mr.formatNumber(resources.TotalTokens), mr.assessTokenUsage(resources.TotalTokens),
		mr.formatNumber(resources.AvgTokensUsed), mr.assessAvgTokens(resources.AvgTokensUsed),
		resources.TotalCost, mr.assessTotalCost(resources.TotalCost),
		resources.AvgCostPerTask, mr.assessCostPerTask(resources.AvgCostPerTask),
		resources.MemoryUsage, mr.assessMemoryUsage(resources.MemoryUsage),
		mr.generateResourceInsights(resources))
}

// buildBehaviorSection 构建行为分析部分
func (mr *MarkdownReporter) buildBehaviorSection(results *EvaluationResults) string {
	behavior := results.Metrics.Behavior

	var report strings.Builder
	report.WriteString("## Behavior Analysis\n\n")

	// Tool Usage Summary
	report.WriteString(fmt.Sprintf("**Average Tool Calls per Task:** %.1f\n\n", behavior.AvgToolCalls))

	// Tool Usage Pattern
	if len(behavior.ToolUsagePattern) > 0 {
		report.WriteString("### Tool Usage Pattern\n\n")
		report.WriteString("| Tool | Usage Count | Percentage |\n")
		report.WriteString("|------|-------------|------------|\n")

		totalUsage := 0
		for _, count := range behavior.ToolUsagePattern {
			totalUsage += count
		}

		for tool, count := range behavior.ToolUsagePattern {
			percentage := float64(count) / float64(totalUsage) * 100
			report.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", tool, count, percentage))
		}
		report.WriteString("\n")
	}

	// Common Failures
	if len(behavior.CommonFailures) > 0 {
		report.WriteString("### Common Failure Patterns\n\n")
		report.WriteString("| Failure Type | Count | Impact |\n")
		report.WriteString("|-------------|-------|--------|\n")

		for failureType, count := range behavior.CommonFailures {
			impact := mr.assessFailureImpact(count)
			report.WriteString(fmt.Sprintf("| %s | %d | %s |\n", failureType, count, impact))
		}
		report.WriteString("\n")
	}

	// Error Patterns
	if len(behavior.ErrorPatterns) > 0 {
		report.WriteString("### Error Patterns\n\n")
		for i, pattern := range behavior.ErrorPatterns {
			if i >= 5 { // Limit to top 5 patterns
				break
			}
			report.WriteString(fmt.Sprintf("- %s\n", pattern))
		}
		report.WriteString("\n")
	}

	return report.String()
}

// buildAttentionSection 构建注意力节省分析部分
func (mr *MarkdownReporter) buildAttentionSection(results *EvaluationResults) string {
	attention := results.Metrics.Attention

	return fmt.Sprintf(`## Attention-Saving Analysis

| Metric | Value | Assessment |
|--------|-------|------------|
| **Attention Saving Ratio** | %.1f%% | %s |
| **HAM (Agent/Baseline)** | %.1f / %.1f min | %s |
| **Avg Verification Time** | %.2f min | %s |
| **P95 Verification Time** | %.2f min | %s |
| **Interruptions per Task** | %.2f | %s |
| **Avg Recovery Cost** | %.2f min | %s |
| **P95 Recovery Cost** | %.2f min | %s |
| **Severe Failure Rate** | %.1f%% | %s |
| **Deliverable Readiness** | %.1f%% | %s |
| **Trust Calibration Error** | %.2f | %s |

### Attention Insights

%s

`,
		attention.AttentionSaving*100, mr.assessAttentionSaving(attention.AttentionSaving),
		attention.HAMAgentMinutes, attention.HAMBaselineMinutes, mr.assessHAM(attention.HAMAgentMinutes, attention.HAMBaselineMinutes),
		attention.AvgVerificationMinutes, mr.assessVerificationMinutes(attention.AvgVerificationMinutes),
		attention.P95VerificationMinutes, mr.assessVerificationMinutes(attention.P95VerificationMinutes),
		attention.InterruptionsPerTask, mr.assessInterruptions(attention.InterruptionsPerTask),
		attention.AvgRecoveryCostMinutes, mr.assessRecoveryCost(attention.AvgRecoveryCostMinutes),
		attention.P95RecoveryCostMinutes, mr.assessRecoveryCost(attention.P95RecoveryCostMinutes),
		attention.SevereFailureRate*100, mr.assessSevereFailure(attention.SevereFailureRate),
		attention.DeliverableReadiness*100, mr.assessDeliverableReadiness(attention.DeliverableReadiness),
		attention.TrustCalibrationErr, mr.assessTrustCalibration(attention.TrustCalibrationErr),
		mr.generateAttentionInsights(attention))
}

// buildInsightsSection 构建洞察部分
func (mr *MarkdownReporter) buildInsightsSection(results *EvaluationResults) string {
	if len(results.Analysis.Insights) == 0 {
		return "## Key Insights\n\nNo specific insights generated for this evaluation.\n\n"
	}

	var report strings.Builder
	report.WriteString("## Key Insights\n\n")

	for _, insight := range results.Analysis.Insights {
		report.WriteString(fmt.Sprintf("### %s %s\n", mr.getInsightIcon(insight.Type), insight.Title))
		impact := string(insight.Impact)
		if len(impact) > 0 {
			impact = strings.ToUpper(impact[:1]) + strings.ToLower(impact[1:])
		}
		report.WriteString(fmt.Sprintf("**Impact:** %s | **Confidence:** %.0f%%\n\n",
			impact, insight.Confidence*100))
		report.WriteString(fmt.Sprintf("%s\n\n", insight.Description))
	}

	return report.String()
}

// buildRecommendationsSection 构建建议部分
func (mr *MarkdownReporter) buildRecommendationsSection(results *EvaluationResults) string {
	if len(results.Analysis.Recommendations) == 0 {
		return "## Recommendations\n\nNo specific recommendations generated for this evaluation.\n\n"
	}

	var report strings.Builder
	report.WriteString("## Recommendations\n\n")

	// Group by priority
	highPriority := []Recommendation{}
	mediumPriority := []Recommendation{}
	lowPriority := []Recommendation{}

	for _, rec := range results.Analysis.Recommendations {
		switch rec.Priority {
		case PriorityHigh:
			highPriority = append(highPriority, rec)
		case PriorityMedium:
			mediumPriority = append(mediumPriority, rec)
		case PriorityLow:
			lowPriority = append(lowPriority, rec)
		}
	}

	// High Priority Recommendations
	if len(highPriority) > 0 {
		report.WriteString("### 🔴 High Priority\n\n")
		for _, rec := range highPriority {
			report.WriteString(mr.formatRecommendation(rec))
		}
	}

	// Medium Priority Recommendations
	if len(mediumPriority) > 0 {
		report.WriteString("### 🟡 Medium Priority\n\n")
		for _, rec := range mediumPriority {
			report.WriteString(mr.formatRecommendation(rec))
		}
	}

	// Low Priority Recommendations
	if len(lowPriority) > 0 {
		report.WriteString("### 🟢 Low Priority\n\n")
		for _, rec := range lowPriority {
			report.WriteString(mr.formatRecommendation(rec))
		}
	}

	return report.String()
}

// buildAlertsSection 构建警报部分
func (mr *MarkdownReporter) buildAlertsSection(results *EvaluationResults) string {
	var report strings.Builder
	report.WriteString("## Alerts\n\n")

	for _, alert := range results.Analysis.Alerts {
		icon := mr.getAlertIcon(alert.Level)
		report.WriteString(fmt.Sprintf("### %s %s\n", icon, alert.Title))
		level := string(alert.Level)
		if len(level) > 0 {
			level = strings.ToUpper(level[:1]) + strings.ToLower(level[1:])
		}
		report.WriteString(fmt.Sprintf("**Level:** %s\n\n", level))
		report.WriteString(fmt.Sprintf("%s\n\n", alert.Description))
		if alert.Action != "" {
			report.WriteString(fmt.Sprintf("**Suggested Action:** %s\n\n", alert.Action))
		}
	}

	return report.String()
}

// buildTaskResultsSummary 构建任务结果摘要
func (mr *MarkdownReporter) buildTaskResultsSummary(results *EvaluationResults) string {
	if len(results.Results) == 0 {
		return "## Task Results Summary\n\nNo task results available.\n\n"
	}

	var report strings.Builder
	report.WriteString("## Task Results Summary\n\n")

	// Status distribution
	statusCount := make(map[swe_bench.ResultStatus]int)
	for _, result := range results.Results {
		statusCount[result.Status]++
	}

	report.WriteString("### Task Status Distribution\n\n")
	report.WriteString("| Status | Count | Percentage |\n")
	report.WriteString("|--------|-------|------------|\n")

	total := len(results.Results)
	for status, count := range statusCount {
		percentage := float64(count) / float64(total) * 100
		report.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", status, count, percentage))
	}

	report.WriteString("\n")

	return report.String()
}

// buildFooter 构建报告尾部
func (mr *MarkdownReporter) buildFooter(results *EvaluationResults) string {
	return fmt.Sprintf(`---

**Report Generation Details:**
- Generated by ALEX Agent Evaluation Framework
- Generation time: %s
- Framework version: v1.0.0
- Report format: Markdown

*This report was automatically generated. For questions or issues, please refer to the evaluation documentation.*

`,
		time.Now().Format("2006-01-02 15:04:05"))
}

// Helper methods for formatting and assessment

func (mr *MarkdownReporter) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
}

func (mr *MarkdownReporter) formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func (mr *MarkdownReporter) assessSuccessRate(rate float64) string {
	switch {
	case rate >= 0.9:
		return "Excellent"
	case rate >= 0.8:
		return "Good"
	case rate >= 0.7:
		return "Acceptable"
	case rate >= 0.6:
		return "Needs Improvement"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessExecutionTime(d time.Duration) string {
	switch {
	case d < 30*time.Second:
		return "Fast"
	case d < 2*time.Minute:
		return "Normal"
	case d < 5*time.Minute:
		return "Slow"
	default:
		return "Very Slow"
	}
}

func (mr *MarkdownReporter) assessTimeoutRate(rate float64) string {
	switch {
	case rate == 0:
		return "Excellent"
	case rate < 0.05:
		return "Good"
	case rate < 0.1:
		return "Acceptable"
	case rate < 0.2:
		return "Concerning"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessRetryRate(rate float64) string {
	switch {
	case rate == 0:
		return "Excellent"
	case rate < 0.05:
		return "Good"
	case rate < 0.15:
		return "Acceptable"
	case rate < 0.25:
		return "Concerning"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessQualityScore(score float64) string {
	switch {
	case score >= 0.9:
		return "Excellent"
	case score >= 0.8:
		return "Good"
	case score >= 0.7:
		return "Acceptable"
	case score >= 0.6:
		return "Needs Improvement"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessErrorRecovery(rate float64) string {
	return mr.assessQualityScore(rate)
}

func (mr *MarkdownReporter) assessConsistency(score float64) string {
	return mr.assessQualityScore(score)
}

func (mr *MarkdownReporter) assessComplexityHandling(score float64) string {
	return mr.assessQualityScore(score)
}

func (mr *MarkdownReporter) assessTokenUsage(tokens int) string {
	switch {
	case tokens < 10000:
		return "Low"
	case tokens < 50000:
		return "Normal"
	case tokens < 100000:
		return "High"
	default:
		return "Very High"
	}
}

func (mr *MarkdownReporter) assessAvgTokens(tokens int) string {
	switch {
	case tokens < 1000:
		return "Efficient"
	case tokens < 3000:
		return "Normal"
	case tokens < 6000:
		return "High"
	default:
		return "Excessive"
	}
}

func (mr *MarkdownReporter) assessTotalCost(cost float64) string {
	switch {
	case cost < 10:
		return "Low"
	case cost < 50:
		return "Normal"
	case cost < 100:
		return "High"
	default:
		return "Very High"
	}
}

func (mr *MarkdownReporter) assessCostPerTask(cost float64) string {
	switch {
	case cost < 0.5:
		return "Efficient"
	case cost < 1.0:
		return "Normal"
	case cost < 2.0:
		return "High"
	default:
		return "Expensive"
	}
}

func (mr *MarkdownReporter) assessMemoryUsage(memory int64) string {
	switch {
	case memory < 50:
		return "Low"
	case memory < 100:
		return "Normal"
	case memory < 200:
		return "High"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessFailureImpact(count int) string {
	switch {
	case count == 0:
		return "None"
	case count < 3:
		return "Low"
	case count < 10:
		return "Medium"
	default:
		return "High"
	}
}

func (mr *MarkdownReporter) assessAttentionSaving(ratio float64) string {
	switch {
	case ratio >= 0.4:
		return "Excellent"
	case ratio >= 0.25:
		return "Good"
	case ratio >= 0.15:
		return "Acceptable"
	case ratio >= 0:
		return "Needs Improvement"
	default:
		return "Regression"
	}
}

func (mr *MarkdownReporter) assessHAM(agent, baseline float64) string {
	if baseline <= 0 {
		return "Unavailable"
	}
	if agent <= baseline*0.75 {
		return "Efficient"
	}
	if agent <= baseline {
		return "Normal"
	}
	return "Inefficient"
}

func (mr *MarkdownReporter) assessVerificationMinutes(minutes float64) string {
	switch {
	case minutes <= 1.0:
		return "Fast"
	case minutes <= 2.5:
		return "Normal"
	case minutes <= 4.0:
		return "Slow"
	default:
		return "Very Slow"
	}
}

func (mr *MarkdownReporter) assessInterruptions(rate float64) string {
	switch {
	case rate <= 1:
		return "Excellent"
	case rate <= 2:
		return "Good"
	case rate <= 3:
		return "Concerning"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessRecoveryCost(minutes float64) string {
	switch {
	case minutes <= 5:
		return "Low"
	case minutes <= 10:
		return "Medium"
	case minutes <= 15:
		return "High"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessSevereFailure(rate float64) string {
	switch {
	case rate == 0:
		return "Excellent"
	case rate <= 0.01:
		return "Good"
	case rate <= 0.03:
		return "Concerning"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) assessDeliverableReadiness(rate float64) string {
	return mr.assessQualityScore(rate)
}

func (mr *MarkdownReporter) assessTrustCalibration(err float64) string {
	switch {
	case err <= 0.15:
		return "Good"
	case err <= 0.25:
		return "Acceptable"
	case err <= 0.35:
		return "Concerning"
	default:
		return "Critical"
	}
}

func (mr *MarkdownReporter) getInsightIcon(insightType InsightType) string {
	switch insightType {
	case InsightPerformance:
		return "⚡"
	case InsightQuality:
		return "🎯"
	case InsightEfficiency:
		return "🔧"
	case InsightBehavior:
		return "🧠"
	default:
		return "💡"
	}
}

func (mr *MarkdownReporter) getAlertIcon(level AlertLevel) string {
	switch level {
	case AlertCritical:
		return "🚨"
	case AlertWarning:
		return "⚠️"
	case AlertInfo:
		return "ℹ️"
	default:
		return "📌"
	}
}

func (mr *MarkdownReporter) formatRecommendation(rec Recommendation) string {
	var report strings.Builder

	report.WriteString(fmt.Sprintf("#### %s\n\n", rec.Title))
	report.WriteString(fmt.Sprintf("%s\n\n", rec.Description))

	if len(rec.ActionItems) > 0 {
		report.WriteString("**Action Items:**\n")
		for _, item := range rec.ActionItems {
			report.WriteString(fmt.Sprintf("- %s\n", item))
		}
		report.WriteString("\n")
	}

	if rec.Expected != "" {
		report.WriteString(fmt.Sprintf("**Expected Improvement:** %s\n\n", rec.Expected))
	}

	return report.String()
}

// Generate insights for different sections
func (mr *MarkdownReporter) generatePerformanceInsights(perf PerformanceMetrics) string {
	insights := []string{}

	if perf.SuccessRate >= 0.8 {
		insights = append(insights, "✅ High success rate indicates strong problem-solving capability")
	} else if perf.SuccessRate < 0.6 {
		insights = append(insights, "❌ Low success rate requires immediate attention")
	}

	if perf.TimeoutRate > 0.15 {
		insights = append(insights, "⏱️ High timeout rate suggests performance bottlenecks")
	}

	if perf.RetryRate > 0.2 {
		insights = append(insights, "🔄 High retry rate indicates reliability issues")
	}

	if len(insights) == 0 {
		insights = append(insights, "Performance metrics are within normal ranges.")
	}

	return strings.Join(insights, "\n\n")
}

func (mr *MarkdownReporter) generateQualityInsights(quality QualityMetrics) string {
	insights := []string{}

	if quality.SolutionQuality >= 0.8 {
		insights = append(insights, "✅ High solution quality indicates effective problem-solving approach")
	} else if quality.SolutionQuality < 0.6 {
		insights = append(insights, "❌ Solution quality needs significant improvement")
	}

	if quality.ErrorRecoveryRate >= 0.7 {
		insights = append(insights, "🛡️ Strong error recovery capabilities")
	}

	if quality.ConsistencyScore < 0.7 {
		insights = append(insights, "⚖️ Inconsistent performance across similar tasks")
	}

	if len(insights) == 0 {
		insights = append(insights, "Quality metrics are within acceptable ranges.")
	}

	return strings.Join(insights, "\n\n")
}

func (mr *MarkdownReporter) generateResourceInsights(resources ResourceMetrics) string {
	insights := []string{}

	if resources.AvgTokensUsed > 6000 {
		insights = append(insights, "🔤 High token usage may indicate verbose reasoning or inefficient prompts")
	}

	if resources.AvgCostPerTask > 2.0 {
		insights = append(insights, "💰 High cost per task affects scalability")
	}

	if resources.MemoryUsage > 100 {
		insights = append(insights, "🧠 High memory usage may cause stability issues")
	}

	if len(insights) == 0 {
		insights = append(insights, "Resource usage is within expected ranges.")
	}

	return strings.Join(insights, "\n\n")
}

func (mr *MarkdownReporter) generateAttentionInsights(attention AttentionMetrics) string {
	insights := []string{}

	if attention.AttentionSaving >= 0.25 {
		insights = append(insights, "✅ Attention-saving ratio meets the primary target")
	} else if attention.AttentionSaving < 0.15 {
		insights = append(insights, "❌ Attention-saving ratio is below target and needs immediate tuning")
	}

	if attention.InterruptionsPerTask > 2 {
		insights = append(insights, "🔔 Frequent interruptions indicate context-switch overhead")
	}

	if attention.P95RecoveryCostMinutes > 15 {
		insights = append(insights, "🧯 Tail recovery cost is high; recovery policies should be strengthened")
	}

	if attention.SevereFailureRate > 0.01 {
		insights = append(insights, "🚨 Severe failure rate exceeds guardrail threshold")
	}

	if attention.TrustCalibrationErr > 0.3 {
		insights = append(insights, "📉 Trust calibration is unstable; confidence signals need tighter grounding")
	}

	if len(insights) == 0 {
		insights = append(insights, "Attention metrics are within expected ranges.")
	}

	return strings.Join(insights, "\n\n")
}
