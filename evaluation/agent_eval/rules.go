package agent_eval

import (
	"fmt"
	"time"
)

// SimpleRuleEngine 简单规则引擎（替代复杂ML系统）
type SimpleRuleEngine struct {
	rules []PerformanceRule
}

// PerformanceRule 性能规则
type PerformanceRule struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Category       RuleCategory                  `json:"category"`
	Condition      func(*EvaluationMetrics) bool `json:"-"`
	Recommendation *Recommendation               `json:"recommendation"`
	Priority       Priority                      `json:"priority"`
	Enabled        bool                          `json:"enabled"`
}

// RuleCategory 规则分类
type RuleCategory string

const (
	CategoryPerformance RuleCategory = "performance"
	CategoryQuality     RuleCategory = "quality"
	CategoryEfficiency  RuleCategory = "efficiency"
	CategoryCost        RuleCategory = "cost"
	CategoryReliability RuleCategory = "reliability"
)

// NewSimpleRuleEngine 创建简单规则引擎
func NewSimpleRuleEngine() *SimpleRuleEngine {
	engine := &SimpleRuleEngine{}
	engine.loadDefaultRules()
	return engine
}

// loadDefaultRules 加载默认规则
func (sre *SimpleRuleEngine) loadDefaultRules() {
	sre.rules = []PerformanceRule{
		// 性能相关规则
		{
			ID:       "PERF_001",
			Name:     "Low Success Rate",
			Category: CategoryPerformance,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Performance.SuccessRate < 0.7
			},
			Recommendation: &Recommendation{
				Title:       "Improve Success Rate",
				Description: "The agent's success rate is below acceptable threshold. Consider reviewing prompts, model configuration, or task complexity.",
				Priority:    PriorityHigh,
				ActionItems: []string{
					"Review and optimize system prompts",
					"Adjust model temperature and max tokens",
					"Analyze failed tasks for common patterns",
					"Consider breaking down complex tasks",
				},
				Expected: "Increase success rate by 15-25%",
			},
			Priority: PriorityHigh,
			Enabled:  true,
		},
		{
			ID:       "PERF_002",
			Name:     "High Timeout Rate",
			Category: CategoryPerformance,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Performance.TimeoutRate > 0.2
			},
			Recommendation: &Recommendation{
				Title:       "Reduce Timeout Issues",
				Description: "High timeout rate indicates tasks are taking too long to complete.",
				Priority:    PriorityMedium,
				ActionItems: []string{
					"Increase task timeout limits",
					"Optimize reasoning loops to avoid infinite cycles",
					"Review tool execution efficiency",
					"Consider using faster models for simpler tasks",
				},
				Expected: "Reduce timeout rate to <10%",
			},
			Priority: PriorityMedium,
			Enabled:  true,
		},
		{
			ID:       "PERF_003",
			Name:     "Slow Execution Time",
			Category: CategoryPerformance,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Performance.AvgExecutionTime > 300*time.Second // 5 minutes
			},
			Recommendation: &Recommendation{
				Title:       "Optimize Execution Time",
				Description: "Average execution time is high, impacting overall efficiency.",
				Priority:    PriorityMedium,
				ActionItems: []string{
					"Profile task execution to identify bottlenecks",
					"Reduce unnecessary tool calls",
					"Optimize prompt length and complexity",
					"Consider parallel execution for independent operations",
				},
				Expected: "Reduce average execution time by 20-30%",
			},
			Priority: PriorityMedium,
			Enabled:  true,
		},

		// 质量相关规则
		{
			ID:       "QUAL_001",
			Name:     "Low Solution Quality",
			Category: CategoryQuality,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Quality.SolutionQuality < 0.6
			},
			Recommendation: &Recommendation{
				Title:       "Enhance Solution Quality",
				Description: "Solution quality metrics indicate need for improvement in reasoning and problem-solving approach.",
				Priority:    PriorityHigh,
				ActionItems: []string{
					"Improve reasoning prompts with examples",
					"Add solution validation steps",
					"Implement code review mechanisms",
					"Enhance error detection and correction",
				},
				Expected: "Improve solution quality score by 20-30%",
			},
			Priority: PriorityHigh,
			Enabled:  true,
		},
		{
			ID:       "QUAL_002",
			Name:     "Poor Error Recovery",
			Category: CategoryQuality,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Quality.ErrorRecoveryRate < 0.5
			},
			Recommendation: &Recommendation{
				Title:       "Improve Error Handling",
				Description: "Low error recovery rate suggests the agent struggles with error scenarios.",
				Priority:    PriorityMedium,
				ActionItems: []string{
					"Implement robust error handling patterns",
					"Add retry logic with exponential backoff",
					"Improve error message interpretation",
					"Train on more error recovery scenarios",
				},
				Expected: "Increase error recovery rate to >70%",
			},
			Priority: PriorityMedium,
			Enabled:  true,
		},
		{
			ID:       "QUAL_003",
			Name:     "Inconsistent Performance",
			Category: CategoryQuality,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Quality.ConsistencyScore < 0.6
			},
			Recommendation: &Recommendation{
				Title:       "Improve Consistency",
				Description: "Performance varies significantly across similar tasks, indicating inconsistent behavior.",
				Priority:    PriorityMedium,
				ActionItems: []string{
					"Standardize problem-solving approach",
					"Add consistency checks in reasoning",
					"Implement deterministic fallback strategies",
					"Review temperature and randomness settings",
				},
				Expected: "Achieve consistency score >75%",
			},
			Priority: PriorityMedium,
			Enabled:  true,
		},

		// 效率相关规则
		{
			ID:       "EFF_001",
			Name:     "Excessive Tool Usage",
			Category: CategoryEfficiency,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Behavior.AvgToolCalls > 15
			},
			Recommendation: &Recommendation{
				Title:       "Optimize Tool Usage",
				Description: "High number of tool calls suggests inefficient problem-solving approach.",
				Priority:    PriorityLow,
				ActionItems: []string{
					"Analyze tool usage patterns",
					"Implement tool call optimization",
					"Add planning phase before execution",
					"Remove redundant or unnecessary tool calls",
				},
				Expected: "Reduce average tool calls to <10 per task",
			},
			Priority: PriorityLow,
			Enabled:  true,
		},
		{
			ID:       "EFF_002",
			Name:     "High Token Consumption",
			Category: CategoryEfficiency,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Resources.AvgTokensUsed > 8000
			},
			Recommendation: &Recommendation{
				Title:       "Reduce Token Usage",
				Description: "High token consumption increases costs and may indicate verbose or inefficient reasoning.",
				Priority:    PriorityLow,
				ActionItems: []string{
					"Optimize prompt templates",
					"Implement context compression",
					"Remove verbose reasoning steps",
					"Use more concise response formats",
				},
				Expected: "Reduce token usage by 20-30%",
			},
			Priority: PriorityLow,
			Enabled:  true,
		},

		// 成本相关规则
		{
			ID:       "COST_001",
			Name:     "High Evaluation Cost",
			Category: CategoryCost,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Resources.TotalCost > 100
			},
			Recommendation: &Recommendation{
				Title:       "Cost Optimization",
				Description: "Total evaluation cost exceeds budget threshold.",
				Priority:    PriorityMedium,
				ActionItems: []string{
					"Consider using more cost-effective models",
					"Implement smart caching strategies",
					"Optimize context length and token usage",
					"Use tiered model approach for different task complexities",
				},
				Expected: "Reduce total cost by 25-40%",
			},
			Priority: PriorityMedium,
			Enabled:  true,
		},
		{
			ID:       "COST_002",
			Name:     "High Cost Per Task",
			Category: CategoryCost,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Resources.AvgCostPerTask > 2.0
			},
			Recommendation: &Recommendation{
				Title:       "Reduce Per-Task Cost",
				Description: "Cost per task is higher than optimal, affecting scalability.",
				Priority:    PriorityLow,
				ActionItems: []string{
					"Profile token usage per task type",
					"Implement early termination for simple tasks",
					"Use cheaper models for routine operations",
					"Batch similar operations",
				},
				Expected: "Reduce cost per task to <$1.50",
			},
			Priority: PriorityLow,
			Enabled:  true,
		},

		// 可靠性相关规则
		{
			ID:       "REL_001",
			Name:     "High Retry Rate",
			Category: CategoryReliability,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Performance.RetryRate > 0.3
			},
			Recommendation: &Recommendation{
				Title:       "Improve First-Attempt Success",
				Description: "High retry rate indicates reliability issues with initial execution.",
				Priority:    PriorityMedium,
				ActionItems: []string{
					"Analyze common failure patterns",
					"Improve input validation",
					"Add pre-execution checks",
					"Enhance initial reasoning quality",
				},
				Expected: "Reduce retry rate to <15%",
			},
			Priority: PriorityMedium,
			Enabled:  true,
		},
		{
			ID:       "REL_002",
			Name:     "Memory Usage Warning",
			Category: CategoryReliability,
			Condition: func(m *EvaluationMetrics) bool {
				return m.Resources.MemoryUsage > 100 // MB
			},
			Recommendation: &Recommendation{
				Title:       "Optimize Memory Usage",
				Description: "High memory usage may cause stability issues in production.",
				Priority:    PriorityLow,
				ActionItems: []string{
					"Profile memory usage patterns",
					"Implement memory cleanup strategies",
					"Optimize data structure usage",
					"Add memory monitoring and alerts",
				},
				Expected: "Reduce memory usage to <75MB",
			},
			Priority: PriorityLow,
			Enabled:  true,
		},
	}
}

// GenerateRecommendations 生成建议
func (sre *SimpleRuleEngine) GenerateRecommendations(metrics *EvaluationMetrics) []Recommendation {
	var recommendations []Recommendation

	for _, rule := range sre.rules {
		if rule.Enabled && rule.Condition != nil && rule.Condition(metrics) {
			if rule.Recommendation != nil {
				// 添加规则上下文信息
				rec := *rule.Recommendation
				rec.Description = fmt.Sprintf("[Rule: %s] %s", rule.ID, rec.Description)
				recommendations = append(recommendations, rec)
			}
		}
	}

	// 按优先级排序
	sre.sortRecommendationsByPriority(recommendations)

	return recommendations
}

// sortRecommendationsByPriority 按优先级排序建议
func (sre *SimpleRuleEngine) sortRecommendationsByPriority(recommendations []Recommendation) {
	priorityOrder := map[Priority]int{
		PriorityHigh:   1,
		PriorityMedium: 2,
		PriorityLow:    3,
	}

	// 简单的冒泡排序
	n := len(recommendations)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if priorityOrder[recommendations[j].Priority] > priorityOrder[recommendations[j+1].Priority] {
				recommendations[j], recommendations[j+1] = recommendations[j+1], recommendations[j]
			}
		}
	}
}

// EvaluateRules 评估所有规则
func (sre *SimpleRuleEngine) EvaluateRules(metrics *EvaluationMetrics) map[string]bool {
	results := make(map[string]bool)

	for _, rule := range sre.rules {
		if rule.Enabled && rule.Condition != nil {
			results[rule.ID] = rule.Condition(metrics)
		}
	}

	return results
}

// GetRulesByCategory 按分类获取规则
func (sre *SimpleRuleEngine) GetRulesByCategory(category RuleCategory) []PerformanceRule {
	var filteredRules []PerformanceRule

	for _, rule := range sre.rules {
		if rule.Category == category {
			filteredRules = append(filteredRules, rule)
		}
	}

	return filteredRules
}

// AddCustomRule 添加自定义规则
func (sre *SimpleRuleEngine) AddCustomRule(rule PerformanceRule) {
	sre.rules = append(sre.rules, rule)
}

// EnableRule 启用规则
func (sre *SimpleRuleEngine) EnableRule(ruleID string) {
	for i := range sre.rules {
		if sre.rules[i].ID == ruleID {
			sre.rules[i].Enabled = true
			break
		}
	}
}

// DisableRule 禁用规则
func (sre *SimpleRuleEngine) DisableRule(ruleID string) {
	for i := range sre.rules {
		if sre.rules[i].ID == ruleID {
			sre.rules[i].Enabled = false
			break
		}
	}
}

// GetRuleCount 获取规则数量
func (sre *SimpleRuleEngine) GetRuleCount() map[string]int {
	counts := map[string]int{
		"total":   len(sre.rules),
		"enabled": 0,
	}

	for _, rule := range sre.rules {
		if rule.Enabled {
			counts["enabled"]++
		}
	}

	return counts
}

// ValidateRules 验证规则配置
func (sre *SimpleRuleEngine) ValidateRules() []string {
	var errors []string

	seenIDs := make(map[string]bool)
	for _, rule := range sre.rules {
		// 检查重复ID
		if seenIDs[rule.ID] {
			errors = append(errors, fmt.Sprintf("Duplicate rule ID: %s", rule.ID))
		}
		seenIDs[rule.ID] = true

		// 检查必需字段
		if rule.Name == "" {
			errors = append(errors, fmt.Sprintf("Rule %s missing name", rule.ID))
		}

		if rule.Condition == nil {
			errors = append(errors, fmt.Sprintf("Rule %s missing condition", rule.ID))
		}

		if rule.Recommendation == nil {
			errors = append(errors, fmt.Sprintf("Rule %s missing recommendation", rule.ID))
		}
	}

	return errors
}

// GenerateRuleReport 生成规则报告
func (sre *SimpleRuleEngine) GenerateRuleReport(metrics *EvaluationMetrics) RuleEvaluationReport {
	triggered := 0
	skipped := 0

	var triggeredRules []string
	var skippedRules []string

	for _, rule := range sre.rules {
		if !rule.Enabled {
			skipped++
			skippedRules = append(skippedRules, rule.ID)
			continue
		}

		if rule.Condition != nil && rule.Condition(metrics) {
			triggered++
			triggeredRules = append(triggeredRules, rule.ID)
		}
	}

	return RuleEvaluationReport{
		TotalRules:     len(sre.rules),
		TriggeredRules: triggered,
		SkippedRules:   skipped,
		TriggeredList:  triggeredRules,
		SkippedList:    skippedRules,
		Timestamp:      time.Now(),
	}
}

// RuleEvaluationReport 规则评估报告
type RuleEvaluationReport struct {
	TotalRules     int       `json:"total_rules"`
	TriggeredRules int       `json:"triggered_rules"`
	SkippedRules   int       `json:"skipped_rules"`
	TriggeredList  []string  `json:"triggered_list"`
	SkippedList    []string  `json:"skipped_list"`
	Timestamp      time.Time `json:"timestamp"`
}
