package agent_eval

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"alex/evaluation/swe_bench"
)

// TestBasicFunctionality 测试基本功能
func TestBasicFunctionality(t *testing.T) {
	// 测试指标收集器
	t.Run("MetricsCollector", func(t *testing.T) {
		collector := NewMetricsCollector()

		// 创建测试数据
		results := []swe_bench.WorkerResult{
			{
				TaskID:     "test_1",
				InstanceID: "instance_1",
				Status:     swe_bench.StatusCompleted,
				Duration:   30 * time.Second,
				TokensUsed: 1500,
				Cost:       0.5,
			},
			{
				TaskID:     "test_2",
				InstanceID: "instance_2",
				Status:     swe_bench.StatusFailed,
				Duration:   60 * time.Second,
				TokensUsed: 2000,
				Cost:       0.8,
			},
		}

		metrics, err := collector.Collect(results)
		if err != nil {
			t.Errorf("Failed to collect metrics: %v", err)
		}

		if metrics.TotalTasks != 2 {
			t.Errorf("Expected 2 total tasks, got %d", metrics.TotalTasks)
		}

		if metrics.Performance.SuccessRate != 0.5 {
			t.Errorf("Expected success rate 0.5, got %f", metrics.Performance.SuccessRate)
		}
	})

	// 测试分析器
	t.Run("BasicAnalyzer", func(t *testing.T) {
		analyzer := NewBasicAnalyzer()

		// 创建测试指标
		metrics := &EvaluationMetrics{
			Performance: PerformanceMetrics{
				SuccessRate:      0.8,
				AvgExecutionTime: 45 * time.Second,
				TimeoutRate:      0.1,
				RetryRate:        0.05,
			},
			Quality: QualityMetrics{
				SolutionQuality:   0.75,
				ErrorRecoveryRate: 0.6,
				ConsistencyScore:  0.7,
			},
			Resources: ResourceMetrics{
				AvgTokensUsed:  2500,
				TotalTokens:    10000,
				AvgCostPerTask: 1.0,
				TotalCost:      50.0,
			},
			Behavior: BehaviorMetrics{
				AvgToolCalls: 8.5,
			},
		}

		analysis := analyzer.Analyze(metrics)

		if analysis.Summary.OverallScore <= 0 {
			t.Errorf("Expected positive overall score, got %f", analysis.Summary.OverallScore)
		}

		if analysis.Summary.PerformanceGrade == "" {
			t.Error("Expected performance grade to be set")
		}

		if len(analysis.Insights) == 0 {
			t.Error("Expected some insights to be generated")
		}
	})

	// 测试规则引擎
	t.Run("SimpleRuleEngine", func(t *testing.T) {
		ruleEngine := NewSimpleRuleEngine()

		// 创建测试指标（低成功率以触发规则）
		metrics := &EvaluationMetrics{
			Performance: PerformanceMetrics{
				SuccessRate: 0.4, // 低成功率
				TimeoutRate: 0.3, // 高超时率
			},
			Quality: QualityMetrics{
				SolutionQuality: 0.5,
			},
			Resources: ResourceMetrics{
				TotalCost: 150.0, // 高成本
			},
		}

		recommendations := ruleEngine.GenerateRecommendations(metrics)

		if len(recommendations) == 0 {
			t.Error("Expected recommendations to be generated for poor metrics")
		}

		// 检查是否有高优先级建议
		hasHighPriority := false
		for _, rec := range recommendations {
			if rec.Priority == PriorityHigh {
				hasHighPriority = true
				break
			}
		}

		if !hasHighPriority {
			t.Error("Expected high priority recommendations for poor performance")
		}
	})

	// 测试自动评审器
	t.Run("ResultAutoReviewer", func(t *testing.T) {
		reviewer := NewResultAutoReviewer(&AutoReviewOptions{
			Enabled:            true,
			MinPassingScore:    0.75,
			EnableAutoRework:   true,
			MaxReworkTasks:     2,
			AlwaysReworkFailed: true,
		})

		results := []swe_bench.WorkerResult{
			{
				TaskID:       "ok",
				InstanceID:   "inst_ok",
				Status:       swe_bench.StatusCompleted,
				Solution:     strings.Repeat("a", 120),
				Explanation:  strings.Repeat("b", 150),
				FilesChanged: []string{"file1"},
			},
			{
				TaskID:      "bad",
				InstanceID:  "inst_bad",
				Status:      swe_bench.StatusFailed,
				Solution:    "short",
				Explanation: "tiny",
			},
		}

		assessments := reviewer.Review(results)
		if len(assessments) != 2 {
			t.Fatalf("expected 2 assessments, got %d", len(assessments))
		}

		var reworkIDs []string
		for _, a := range assessments {
			if a.InstanceID == "inst_ok" && a.NeedsRework {
				t.Fatalf("expected completed instance to pass review")
			}
		}
		reworkIDs = reviewer.SelectReworkCandidates(assessments)
		if len(reworkIDs) == 0 || reworkIDs[0] != "inst_bad" {
			t.Fatalf("expected failed instance to be flagged for rework")
		}
	})

	// 测试报告生成器
	t.Run("MarkdownReporter", func(t *testing.T) {
		reporter := NewMarkdownReporter()

		// 创建测试结果
		results := &EvaluationResults{
			JobID: "test_job_001",
			Results: []swe_bench.WorkerResult{
				{
					TaskID:     "test_1",
					InstanceID: "instance_1",
					Status:     swe_bench.StatusCompleted,
					Duration:   30 * time.Second,
				},
			},
			Metrics: &EvaluationMetrics{
				Performance: PerformanceMetrics{
					SuccessRate:      0.8,
					AvgExecutionTime: 45 * time.Second,
				},
				TotalTasks: 1,
			},
			Analysis: &AnalysisResult{
				Summary: AnalysisSummary{
					OverallScore:     0.75,
					PerformanceGrade: "B",
					RiskLevel:        "low",
				},
				Insights:        []Insight{},
				Recommendations: []Recommendation{},
				Alerts:          []Alert{},
			},
			Timestamp: time.Now(),
		}

		// 测试报告构建（不写文件）
		content := reporter.buildReportContent(results)

		if len(content) == 0 {
			t.Error("Expected report content to be generated")
		}

		// 检查是否包含关键信息
		expectedStrings := []string{
			"Agent Evaluation Report",
			"test_job_001",
			"Overall Score",
			"Performance Analysis",
		}

		for _, expected := range expectedStrings {
			if len(content) == 0 || !contains(content, expected) {
				t.Errorf("Expected report to contain '%s'", expected)
			}
		}
	})
}

// TestEvaluationManager 测试评估管理器
func TestEvaluationManager(t *testing.T) {
	config := &EvaluationConfig{
		DatasetType:    "test",
		DatasetPath:    "test_data.json",
		InstanceLimit:  2,
		MaxWorkers:     1,
		TimeoutPerTask: 60 * time.Second,
		EnableMetrics:  true,
		OutputDir:      "./test_output",
		ReportFormat:   "markdown",
	}

	manager := NewEvaluationManager(config)

	if manager == nil {
		t.Error("Expected evaluation manager to be created")
	}

	// 测试任务调度（不实际执行）
	ctx := context.Background()

	// 这个测试会失败，因为没有实际的数据集，但可以测试调度逻辑
	job, err := manager.ScheduleEvaluation(ctx, config)
	if err == nil && job == nil {
		t.Error("Expected either job or error")
	}

	// 测试状态查询
	if job != nil {
		status := manager.GetJobStatus(job.ID)
		if status == "" {
			t.Error("Expected job status to be returned")
		}
	}
}

// TestCLIManager 测试CLI管理器
func TestCLIManager(t *testing.T) {
	cliManager, err := NewCLIManager("./test_output")
	if err != nil {
		t.Errorf("Failed to create CLI manager: %v", err)
	}

	if cliManager == nil {
		t.Error("Expected CLI manager to be created")
	}

	// 测试默认选项
	options := DefaultEvaluationOptions()
	if options.InstanceLimit <= 0 {
		t.Error("Expected positive instance limit in default options")
	}

	if options.OutputDir == "" {
		t.Error("Expected output directory in default options")
	}

	if cliManager != nil {
		disable := false
		minScore := 0.9
		rework := false
		maxTasks := 1
		alwaysReworkFailed := false

		options.AutoReview = &AutoReviewOverrides{
			Enabled:            &disable,
			MinPassingScore:    &minScore,
			EnableAutoRework:   &rework,
			MaxReworkTasks:     &maxTasks,
			AlwaysReworkFailed: &alwaysReworkFailed,
		}

		config := cliManager.applyOptions(options)
		if config.AutoReview == nil {
			t.Fatalf("expected auto review config to be populated")
		}
		if config.AutoReview.Enabled != disable {
			t.Errorf("expected auto review enabled to be %v", disable)
		}
		if config.AutoReview.MinPassingScore != minScore {
			t.Errorf("expected min passing score override to be %.2f", minScore)
		}
		if config.AutoReview.EnableAutoRework != rework {
			t.Errorf("expected enable auto rework override to be %v", rework)
		}
		if config.AutoReview.MaxReworkTasks != maxTasks {
			t.Errorf("expected max rework tasks override to be %d", maxTasks)
		}
		if config.AutoReview.AlwaysReworkFailed != alwaysReworkFailed {
			t.Errorf("expected always rework failed override to be %v", alwaysReworkFailed)
		}
	}
}

// TestConfigValidation 测试配置验证
func TestConfigValidation(t *testing.T) {
	// 测试有效配置
	validConfig := &EvaluationConfig{
		DatasetPath:    "/tmp/test.json",
		InstanceLimit:  5,
		MaxWorkers:     2,
		TimeoutPerTask: 60 * time.Second,
		OutputDir:      "/tmp/output",
	}

	// 创建临时文件
	tmpFile := "/tmp/test.json"
	if err := createTempFile(tmpFile); err == nil {
		defer func() { _ = deleteTempFile(tmpFile) }()

		if err := ValidateConfig(validConfig); err != nil {
			t.Errorf("Expected valid config to pass validation: %v", err)
		}
	}

	// 测试无效配置
	invalidConfigs := []*EvaluationConfig{
		{DatasetPath: "", InstanceLimit: 5, MaxWorkers: 2, TimeoutPerTask: 60 * time.Second, OutputDir: "/tmp"},
		{DatasetPath: "/tmp/test.json", InstanceLimit: 0, MaxWorkers: 2, TimeoutPerTask: 60 * time.Second, OutputDir: "/tmp"},
		{DatasetPath: "/tmp/test.json", InstanceLimit: 5, MaxWorkers: 0, TimeoutPerTask: 60 * time.Second, OutputDir: "/tmp"},
		{DatasetPath: "/tmp/test.json", InstanceLimit: 5, MaxWorkers: 2, TimeoutPerTask: 0, OutputDir: "/tmp"},
		{DatasetPath: "/tmp/test.json", InstanceLimit: 5, MaxWorkers: 2, TimeoutPerTask: 60 * time.Second, OutputDir: ""},
	}

	for i, config := range invalidConfigs {
		if err := ValidateConfig(config); err == nil {
			t.Errorf("Expected invalid config %d to fail validation", i)
		}
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		len(s) >= len(substr) &&
		findInString(s, substr)
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func createTempFile(path string) error {
	// 创建空的JSON文件用于测试
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// 写入空JSON数组
	_, err = file.WriteString("[]")
	return err
}

func deleteTempFile(path string) error {
	// 删除临时文件
	return os.Remove(path)
}
