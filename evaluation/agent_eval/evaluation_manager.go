package agent_eval

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"alex/evaluation/swe_bench"
)

// EvaluationManager - 简化的评估管理器（3层架构中的第一层）
type EvaluationManager struct {
	batchRunner  *swe_bench.BatchProcessorImpl
	metricsStore *SimpleMetricsStore
	analyzer     *BasicAnalyzer
	reporter     *MarkdownReporter
	reviewer     *ResultAutoReviewer

	// State management
	mu         sync.RWMutex
	activeJobs map[string]*EvaluationJob
	config     *EvaluationConfig
}

// EvaluationJob 代表一个评估任务
type EvaluationJob struct {
	ID        string
	Status    JobStatus
	Config    *EvaluationConfig
	Results   *EvaluationResults
	StartTime time.Time
	EndTime   time.Time
	Error     error
}

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// EvaluationConfig 评估配置
type EvaluationConfig struct {
	// Dataset configuration
	DatasetType   string `json:"dataset_type"`
	DatasetPath   string `json:"dataset_path"`
	InstanceLimit int    `json:"instance_limit"`

	// Execution configuration
	MaxWorkers     int           `json:"max_workers"`
	TimeoutPerTask time.Duration `json:"timeout_per_task"`

	// Metrics configuration
	EnableMetrics bool     `json:"enable_metrics"`
	MetricsTypes  []string `json:"metrics_types"`

	// Output configuration
	OutputDir    string `json:"output_dir"`
	ReportFormat string `json:"report_format"`

	// Auto review configuration
	AutoReview *AutoReviewOptions `json:"auto_review,omitempty"`
}

// NewEvaluationManager 创建新的评估管理器
func NewEvaluationManager(config *EvaluationConfig) *EvaluationManager {
	if config.AutoReview == nil {
		config.AutoReview = defaultAutoReviewOptions()
	}
	return &EvaluationManager{
		metricsStore: NewSimpleMetricsStore(filepath.Join(config.OutputDir, "metrics")),
		analyzer:     NewBasicAnalyzer(),
		reporter:     NewMarkdownReporter(),
		reviewer:     NewResultAutoReviewer(config.AutoReview),
		activeJobs:   make(map[string]*EvaluationJob),
		config:       config,
	}
}

// ScheduleEvaluation 调度评估任务
func (em *EvaluationManager) ScheduleEvaluation(ctx context.Context, config *EvaluationConfig) (*EvaluationJob, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	jobID := fmt.Sprintf("eval_%d", time.Now().Unix())
	job := &EvaluationJob{
		ID:        jobID,
		Status:    JobStatusPending,
		Config:    config,
		StartTime: time.Now(),
	}

	em.activeJobs[jobID] = job

	// 异步执行评估
	go em.executeEvaluation(ctx, job)

	return job, nil
}

// executeEvaluation 执行评估任务
func (em *EvaluationManager) executeEvaluation(ctx context.Context, job *EvaluationJob) {
	em.updateJobStatus(job.ID, JobStatusRunning)

	defer func() {
		job.EndTime = time.Now()
		if job.Error != nil {
			em.updateJobStatus(job.ID, JobStatusFailed)
			log.Printf("Evaluation job %s failed: %v", job.ID, job.Error)
		} else {
			em.updateJobStatus(job.ID, JobStatusCompleted)
			log.Printf("Evaluation job %s completed successfully", job.ID)
		}
	}()

	// 1. 加载数据集
	instances, err := em.loadDataset(ctx, job.Config)
	if err != nil {
		job.Error = fmt.Errorf("failed to load dataset: %w", err)
		return
	}

	// 2. 执行评估（基于现有SWE-Bench处理器）
	results, err := em.runEvaluation(ctx, instances, job.Config, "results.json")
	if err != nil {
		job.Error = fmt.Errorf("failed to run evaluation: %w", err)
		return
	}

	// 2.1 自动评审与反工
	var reviewSummary *AutoReviewReport
	results, reviewSummary = em.processAutoReview(ctx, results, instances, job.Config)

	// 3. 收集和分析指标
	if job.Config.EnableMetrics {
		metrics, err := em.collectMetrics(ctx, results)
		if err != nil {
			log.Printf("Warning: Failed to collect metrics: %v", err)
		} else {
			// 存储指标
			if err := em.metricsStore.Store(job.ID, metrics); err != nil {
				log.Printf("Warning: Failed to store metrics: %v", err)
			}

			// 分析指标
			analysis := em.analyzer.Analyze(metrics)

			// 生成报告
			report := &EvaluationResults{
				JobID:         job.ID,
				Results:       results,
				Metrics:       metrics,
				Analysis:      analysis,
				ReviewSummary: reviewSummary,
				Timestamp:     time.Now(),
			}

			job.Results = report

			// 生成报告文件
			if err := em.generateReport(ctx, report, job.Config); err != nil {
				log.Printf("Warning: Failed to generate report: %v", err)
			}
		}
	}
}

// loadDataset 加载数据集
func (em *EvaluationManager) loadDataset(ctx context.Context, config *EvaluationConfig) ([]swe_bench.Instance, error) {
	// 基于现有SWE-Bench加载器
	loader := swe_bench.NewDatasetLoader()

	datasetConfig := swe_bench.DatasetConfig{
		Type:          config.DatasetType,
		FilePath:      config.DatasetPath,
		InstanceLimit: config.InstanceLimit,
	}

	return loader.LoadInstances(ctx, datasetConfig)
}

// runEvaluation 运行评估
func (em *EvaluationManager) runEvaluation(ctx context.Context, instances []swe_bench.Instance, config *EvaluationConfig, outputName string) ([]swe_bench.WorkerResult, error) {
	// 创建批处理配置
	batchConfig := &swe_bench.BatchConfig{
		NumWorkers: config.MaxWorkers,
		OutputPath: filepath.Join(config.OutputDir, outputName),
	}

	// 设置超时
	if config.TimeoutPerTask > 0 {
		batchConfig.Agent.Timeout = int(config.TimeoutPerTask.Seconds())
	}

	// 每次运行都创建新的处理器以隔离输出
	em.batchRunner = swe_bench.NewBatchProcessor(batchConfig)

	batchResult, err := em.batchRunner.ProcessBatch(ctx, instances, batchConfig)
	if err != nil {
		return nil, err
	}

	return batchResult.Results, nil
}

// collectMetrics 收集指标
func (em *EvaluationManager) collectMetrics(ctx context.Context, results []swe_bench.WorkerResult) (*EvaluationMetrics, error) {
	collector := NewMetricsCollector()
	return collector.Collect(results)
}

// processAutoReview 根据配置自动评估并尝试反工
func (em *EvaluationManager) processAutoReview(ctx context.Context, results []swe_bench.WorkerResult, instances []swe_bench.Instance, config *EvaluationConfig) ([]swe_bench.WorkerResult, *AutoReviewReport) {
	if em.reviewer == nil || config.AutoReview == nil || !config.AutoReview.Enabled {
		return results, nil
	}

	em.reviewer.UpdateOptions(config.AutoReview)
	assessments := em.reviewer.Review(results)
	report := &AutoReviewReport{Assessments: assessments}

	if !config.AutoReview.EnableAutoRework {
		return results, report
	}

	reworkIDs := em.reviewer.SelectReworkCandidates(assessments)
	if len(reworkIDs) == 0 {
		return results, report
	}

	instanceIndex := make(map[string]swe_bench.Instance, len(instances))
	for _, instance := range instances {
		instanceIndex[instance.ID] = instance
	}

	reworkInstances := make([]swe_bench.Instance, 0, len(reworkIDs))
	for _, id := range reworkIDs {
		if instance, ok := instanceIndex[id]; ok {
			reworkInstances = append(reworkInstances, instance)
		}
	}

	if len(reworkInstances) == 0 {
		return results, report
	}

	reworkOutput := fmt.Sprintf("rework_%d.json", time.Now().Unix())
	reworkResults, err := em.runEvaluation(ctx, reworkInstances, config, reworkOutput)
	if err != nil {
		report.Rework = &ReworkSummary{
			Attempted: len(reworkInstances),
			Notes:     []string{fmt.Sprintf("auto rework failed: %v", err)},
		}
		return results, report
	}

	merged, summary := mergeReworkResults(results, reworkResults)
	report.Rework = summary
	report.Assessments = em.reviewer.Review(merged)

	return merged, report
}

// generateReport 生成报告
func (em *EvaluationManager) generateReport(ctx context.Context, report *EvaluationResults, config *EvaluationConfig) error {
	outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("report_%s.md", report.JobID))
	return em.reporter.GenerateReport(report, outputPath)
}

// mergeReworkResults 将反工结果融合进原始结果中
func mergeReworkResults(original []swe_bench.WorkerResult, rework []swe_bench.WorkerResult) ([]swe_bench.WorkerResult, *ReworkSummary) {
	if len(rework) == 0 {
		return original, &ReworkSummary{}
	}

	merged := make([]swe_bench.WorkerResult, len(original))
	copy(merged, original)
	index := make(map[string]int, len(original))
	for i, result := range original {
		index[result.InstanceID] = i
	}

	summary := &ReworkSummary{Attempted: len(rework)}

	for _, result := range rework {
		if idx, ok := index[result.InstanceID]; ok {
			previous := merged[idx]
			if improved(previous, result) {
				summary.Improved++
			}
			merged[idx] = result
		} else {
			merged = append(merged, result)
		}

		if result.Status == swe_bench.StatusCompleted {
			summary.Completed++
		}
	}

	summary.StillFailed = summary.Attempted - summary.Completed

	return merged, summary
}

func improved(previous, current swe_bench.WorkerResult) bool {
	if previous.Status == swe_bench.StatusCompleted {
		return current.Status == swe_bench.StatusCompleted && current.Duration < previous.Duration
	}

	return current.Status == swe_bench.StatusCompleted || len(current.Solution) > len(previous.Solution)
}

// updateJobStatus 更新任务状态
func (em *EvaluationManager) updateJobStatus(jobID string, status JobStatus) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if job, exists := em.activeJobs[jobID]; exists {
		job.Status = status
	}
}

// GetJobStatus 获取任务状态
func (em *EvaluationManager) GetJobStatus(jobID string) JobStatus {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if job, exists := em.activeJobs[jobID]; exists {
		return job.Status
	}
	return JobStatusFailed
}

// GetJobResults 获取任务结果
func (em *EvaluationManager) GetJobResults(jobID string) (*EvaluationResults, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	job, exists := em.activeJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	if job.Results == nil {
		return nil, fmt.Errorf("job results not ready: %s", jobID)
	}

	return job.Results, nil
}

// ListActiveJobs 列出活跃任务
func (em *EvaluationManager) ListActiveJobs() map[string]JobStatus {
	em.mu.RLock()
	defer em.mu.RUnlock()

	jobs := make(map[string]JobStatus)
	for id, job := range em.activeJobs {
		jobs[id] = job.Status
	}
	return jobs
}
