package agent_eval

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CLIManager CLI管理器
type CLIManager struct {
	evaluationManager *EvaluationManager
	config            *EvaluationConfig
	outputDir         string
}

// NewCLIManager 创建CLI管理器
func NewCLIManager(outputDir string) (*CLIManager, error) {
	if outputDir == "" {
		outputDir = "./evaluation_results"
	}

	cleanedOutputDir, err := sanitizeOutputPath(defaultOutputBaseDir, outputDir)
	if err != nil {
		return nil, err
	}

	config := &EvaluationConfig{
		DatasetType:    "general_agent",
		DatasetPath:    "",
		InstanceLimit:  10, // 默认少量实例用于测试
		MaxWorkers:     2,
		AgentID:        "default-agent",
		TimeoutPerTask: 300 * time.Second, // 5分钟超时
		EnableMetrics:  true,
		MetricsTypes:   []string{"performance", "quality", "resource", "behavior"},
		OutputDir:      cleanedOutputDir,
		ReportFormat:   "markdown",
	}

	evaluationManager := NewEvaluationManager(config)

	return &CLIManager{
		evaluationManager: evaluationManager,
		config:            config,
		outputDir:         outputDir,
	}, nil
}

// RunEvaluation 运行评估
func (cm *CLIManager) RunEvaluation(ctx context.Context, options *EvaluationOptions) (*EvaluationJob, error) {
	if options == nil {
		options = DefaultEvaluationOptions()
	}

	log.Printf("Starting agent evaluation with options: %+v", options)

	// 应用选项到配置
	config := cm.applyOptions(options)

	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	// 创建输出目录
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// 调度评估
	job, err := cm.evaluationManager.ScheduleEvaluation(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule evaluation: %w", err)
	}

	log.Printf("Evaluation job %s scheduled successfully", job.ID)

	// 等待完成（简化版本）
	return cm.waitForCompletion(ctx, job)
}

// applyOptions 应用选项到配置
func (cm *CLIManager) applyOptions(options *EvaluationOptions) *EvaluationConfig {
	config := *cm.config // 复制默认配置

	if options.DatasetPath != "" {
		config.DatasetPath = options.DatasetPath
	}
	if options.DatasetType != "" {
		config.DatasetType = options.DatasetType
	}
	if options.AgentID != "" {
		config.AgentID = options.AgentID
	}
	if options.InstanceLimit > 0 {
		config.InstanceLimit = options.InstanceLimit
	}
	if options.MaxWorkers > 0 {
		config.MaxWorkers = options.MaxWorkers
	}
	if options.TimeoutPerTask > 0 {
		config.TimeoutPerTask = options.TimeoutPerTask
	}
	if options.OutputDir != "" {
		sanitized, err := sanitizeOutputPath(defaultOutputBaseDir, options.OutputDir)
		if err != nil {
			log.Printf("Invalid output dir override %q: %v", options.OutputDir, err)
		} else {
			config.OutputDir = sanitized
		}
	}
	if !options.EnableMetrics {
		config.EnableMetrics = options.EnableMetrics
	}
	if options.ReportFormat != "" {
		config.ReportFormat = options.ReportFormat
	}

	return &config
}

// waitForCompletion 等待任务完成
func (cm *CLIManager) waitForCompletion(ctx context.Context, job *EvaluationJob) (*EvaluationJob, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			status := cm.evaluationManager.GetJobStatus(job.ID)

			log.Printf("Job %s status: %s", job.ID, status)

			switch status {
			case JobStatusCompleted:
				log.Printf("Job %s completed successfully", job.ID)

				// 获取结果
				results, err := cm.evaluationManager.GetJobResults(job.ID)
				if err != nil {
					log.Printf("Warning: Failed to get job results: %v", err)
				} else {
					totalTasks := results.Metrics.TotalTasks
					if totalTasks == 0 {
						totalTasks = len(results.Results)
					}
					if results.Analysis != nil {
						log.Printf("Results summary: %d total tasks, overall score: %.1f%%",
							totalTasks,
							results.Analysis.Summary.OverallScore*100)
					} else {
						log.Printf("Results summary: %d total tasks", totalTasks)
					}
				}

				return job, nil

			case JobStatusFailed:
				return job, fmt.Errorf("job %s failed", job.ID)
			}
		}
	}
}

// ListJobs 列出活跃任务
func (cm *CLIManager) ListJobs() map[string]JobStatus {
	return cm.evaluationManager.ListActiveJobs()
}

// GetJobStatus 获取任务状态
func (cm *CLIManager) GetJobStatus(jobID string) JobStatus {
	return cm.evaluationManager.GetJobStatus(jobID)
}

// GetJobResults 获取任务结果
func (cm *CLIManager) GetJobResults(jobID string) (*EvaluationResults, error) {
	return cm.evaluationManager.GetJobResults(jobID)
}

// EvaluationOptions 评估选项
type EvaluationOptions struct {
	DatasetPath    string        `json:"dataset_path"`
	DatasetType    string        `json:"dataset_type"`
	InstanceLimit  int           `json:"instance_limit"`
	MaxWorkers     int           `json:"max_workers"`
	TimeoutPerTask time.Duration `json:"timeout_per_task"`
	OutputDir      string        `json:"output_dir"`
	EnableMetrics  bool          `json:"enable_metrics"`
	ReportFormat   string        `json:"report_format"`
	AgentID        string        `json:"agent_id"`
	Verbose        bool          `json:"verbose"`
}

// DefaultEvaluationOptions 默认评估选项
func DefaultEvaluationOptions() *EvaluationOptions {
	return &EvaluationOptions{
		DatasetPath:    "",
		DatasetType:    "general_agent",
		InstanceLimit:  10,
		MaxWorkers:     2,
		TimeoutPerTask: 300 * time.Second,
		OutputDir:      "./evaluation_results",
		EnableMetrics:  true,
		ReportFormat:   "markdown",
		AgentID:        "default-agent",
		Verbose:        false,
	}
}

// RunQuickEvaluation 运行快速评估（用于测试）
func RunQuickEvaluation() error {
	ctx := context.Background()

	// 创建CLI管理器
	cliManager, err := NewCLIManager("./evaluation_results")
	if err != nil {
		return fmt.Errorf("failed to create CLI manager: %w", err)
	}

	// 使用默认选项
	options := DefaultEvaluationOptions()
	options.InstanceLimit = 3 // 更少的实例用于快速测试
	options.Verbose = true

	log.Println("Starting quick evaluation...")

	// 运行评估
	job, err := cliManager.RunEvaluation(ctx, options)
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	log.Printf("Quick evaluation completed successfully. Job ID: %s", job.ID)

	// 显示结果摘要
	if job.Results != nil {
		summary := job.Results.Analysis.Summary
		log.Printf("Results Summary:")
		log.Printf("- Overall Score: %.1f%% (%s)", summary.OverallScore*100, summary.PerformanceGrade)
		log.Printf("- Risk Level: %s", summary.RiskLevel)
		log.Printf("- Key Strengths: %v", summary.KeyStrengths)
		log.Printf("- Key Weaknesses: %v", summary.KeyWeaknesses)

		// 生成报告路径
		reportPath := filepath.Join(options.OutputDir, fmt.Sprintf("report_%s.md", job.ID))
		log.Printf("Detailed report saved to: %s", reportPath)
	}

	return nil
}

// CompareConfigurations 比较配置
func (cm *CLIManager) CompareConfigurations(ctx context.Context, config1, config2 *EvaluationConfig) (*ComparisonResult, error) {
	log.Printf("Starting configuration comparison...")

	// 运行基准评估
	log.Printf("Running baseline evaluation...")
	job1, err := cm.evaluationManager.ScheduleEvaluation(ctx, config1)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule baseline evaluation: %w", err)
	}

	// 等待基准完成
	job1, err = cm.waitForCompletion(ctx, job1)
	if err != nil {
		return nil, fmt.Errorf("baseline evaluation failed: %w", err)
	}

	// 运行实验评估
	log.Printf("Running experiment evaluation...")
	job2, err := cm.evaluationManager.ScheduleEvaluation(ctx, config2)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule experiment evaluation: %w", err)
	}

	// 等待实验完成
	job2, err = cm.waitForCompletion(ctx, job2)
	if err != nil {
		return nil, fmt.Errorf("experiment evaluation failed: %w", err)
	}

	// 比较结果
	comparisonResult := cm.compareResults(job1.Results, job2.Results)

	log.Printf("Configuration comparison completed")
	log.Printf("Success rate delta: %.2f%%", comparisonResult.ComparisonMetrics.SuccessRateDelta*100)
	log.Printf("Performance delta: %.2f%%", comparisonResult.ComparisonMetrics.PerformanceDelta*100)
	log.Printf("Cost delta: %.2f%%", comparisonResult.ComparisonMetrics.CostDelta*100)

	return comparisonResult, nil
}

// compareResults 比较结果
func (cm *CLIManager) compareResults(baseline, experiment *EvaluationResults) *ComparisonResult {
	baselineMetrics := baseline.Metrics
	experimentMetrics := experiment.Metrics

	// 计算差异
	successRateDelta := experimentMetrics.Performance.SuccessRate - baselineMetrics.Performance.SuccessRate

	// 简化的性能delta计算（基于执行时间）
	performanceDelta := 0.0
	if baselineMetrics.Performance.AvgExecutionTime > 0 {
		performanceDelta = (float64(baselineMetrics.Performance.AvgExecutionTime - experimentMetrics.Performance.AvgExecutionTime)) / float64(baselineMetrics.Performance.AvgExecutionTime)
	}

	// 成本delta
	costDelta := 0.0
	if baselineMetrics.Resources.AvgCostPerTask > 0 {
		costDelta = (experimentMetrics.Resources.AvgCostPerTask - baselineMetrics.Resources.AvgCostPerTask) / baselineMetrics.Resources.AvgCostPerTask
	}

	// 质量delta
	qualityDelta := experimentMetrics.Quality.SolutionQuality - baselineMetrics.Quality.SolutionQuality

	comparisonMetrics := &ComparisonMetrics{
		SuccessRateDelta:      successRateDelta,
		PerformanceDelta:      performanceDelta,
		CostDelta:             costDelta,
		QualityDelta:          qualityDelta,
		SignificanceLevel:     0.05, // 固定显著性水平
		StatisticalConfidence: 0.85, // 简化的置信度
	}

	// 识别改进和回归
	var improvements []Improvement
	var regressions []Regression

	if successRateDelta > 0.05 {
		improvements = append(improvements, Improvement{
			Category:    "Performance",
			Metric:      "Success Rate",
			Improvement: successRateDelta,
			Description: fmt.Sprintf("Success rate improved by %.1f%%", successRateDelta*100),
		})
	} else if successRateDelta < -0.05 {
		regressions = append(regressions, Regression{
			Category:    "Performance",
			Metric:      "Success Rate",
			Regression:  -successRateDelta,
			Description: fmt.Sprintf("Success rate decreased by %.1f%%", -successRateDelta*100),
			Impact:      "High",
		})
	}

	if performanceDelta > 0.1 {
		improvements = append(improvements, Improvement{
			Category:    "Performance",
			Metric:      "Execution Time",
			Improvement: performanceDelta,
			Description: fmt.Sprintf("Execution time improved by %.1f%%", performanceDelta*100),
		})
	}

	if costDelta < -0.1 {
		improvements = append(improvements, Improvement{
			Category:    "Cost",
			Metric:      "Cost per Task",
			Improvement: -costDelta,
			Description: fmt.Sprintf("Cost per task reduced by %.1f%%", -costDelta*100),
		})
	}

	return &ComparisonResult{
		BaselineResults:   baseline,
		ExperimentResults: experiment,
		ComparisonMetrics: comparisonMetrics,
		Improvements:      improvements,
		Regressions:       regressions,
		Timestamp:         time.Now(),
	}
}

// ValidateConfig 验证配置
func ValidateConfig(config *EvaluationConfig) error {
	datasetType := normalizeDatasetType(config.DatasetType)

	datasetPath := strings.TrimSpace(config.DatasetPath)

	if config.InstanceLimit <= 0 {
		return fmt.Errorf("instance limit must be positive")
	}

	if config.MaxWorkers <= 0 {
		return fmt.Errorf("max workers must be positive")
	}

	if config.TimeoutPerTask <= 0 {
		return fmt.Errorf("timeout per task must be positive")
	}

	if config.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}

	cleanedOutputDir, err := sanitizeOutputPath(defaultOutputBaseDir, config.OutputDir)
	if err != nil {
		return err
	}
	config.OutputDir = cleanedOutputDir

	// 检查数据集文件是否存在（general_agent 可使用内置数据集）
	if datasetType != "general_agent" || datasetPath != "" {
		if datasetPath == "" {
			return fmt.Errorf("dataset path is required")
		}
		if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
			return fmt.Errorf("dataset file does not exist: %s", datasetPath)
		}
	}

	return nil
}

func normalizeDatasetType(datasetType string) string {
	d := strings.TrimSpace(datasetType)
	switch d {
	case "test":
		return "general_agent"
	default:
		return d
	}
}
