package agent_eval

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"alex/evaluation/swe_bench"
)

// EvaluationManager - 简化的评估管理器（3层架构中的第一层）
type EvaluationManager struct {
	batchRunner  *swe_bench.BatchProcessorImpl
	metricsStore *SimpleMetricsStore
	agentStore   *AgentDataStore
	analyzer     *BasicAnalyzer
	reporter     *MarkdownReporter

	hydrated bool
	jobSeq   atomic.Uint64

	// State management
	mu         sync.RWMutex
	activeJobs map[string]*EvaluationJob
	config     *EvaluationConfig
}

// fallbackAnalysisResult provides a minimal summary when full metric analysis
// cannot be produced (for example when metric collection is disabled or
// partially fails). It relies on auto scores and basic result stats only.
type fallbackAnalysisResult struct {
	overallScore float64
	successRate  float64
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

	// Agent metadata
	AgentID string `json:"agent_id"`

	// Execution configuration
	MaxWorkers     int           `json:"max_workers"`
	TimeoutPerTask time.Duration `json:"timeout_per_task"`

	// Metrics configuration
	EnableMetrics bool     `json:"enable_metrics"`
	MetricsTypes  []string `json:"metrics_types"`

	// Output configuration
	OutputDir    string `json:"output_dir"`
	ReportFormat string `json:"report_format"`
}

// NewEvaluationManager 创建新的评估管理器
func NewEvaluationManager(config *EvaluationConfig) *EvaluationManager {
	return &EvaluationManager{
		metricsStore: NewSimpleMetricsStore(filepath.Join(config.OutputDir, "metrics")),
		agentStore:   NewAgentDataStore(filepath.Join(config.OutputDir, "agents")),
		analyzer:     NewBasicAnalyzer(),
		reporter:     NewMarkdownReporter(),
		activeJobs:   make(map[string]*EvaluationJob),
		config:       config,
	}
}

// ScheduleEvaluation 调度评估任务
func (em *EvaluationManager) ScheduleEvaluation(ctx context.Context, config *EvaluationConfig) (*EvaluationJob, error) {
	em.ensureHydrated()

	em.mu.Lock()
	defer em.mu.Unlock()

	jobID := em.nextJobID()
	job := &EvaluationJob{
		ID:        jobID,
		Status:    JobStatusPending,
		Config:    em.cloneConfig(config),
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
		em.mu.Lock()
		job.EndTime = time.Now()
		hasError := job.Error != nil
		em.mu.Unlock()

		if hasError {
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
		em.setJobError(job.ID, fmt.Errorf("failed to load dataset: %w", err))
		return
	}

	// 2. 执行评估（基于现有SWE-Bench处理器）
	results, err := em.runEvaluation(ctx, instances, job.Config)
	if err != nil {
		em.setJobError(job.ID, fmt.Errorf("failed to run evaluation: %w", err))
		return
	}

	autoScores := em.scoreResults(results)
	evaluation := &EvaluationResults{
		JobID:      job.ID,
		AgentID:    job.Config.AgentID,
		Config:     em.cloneConfig(job.Config),
		Results:    results,
		AutoScores: autoScores,
		Timestamp:  time.Now(),
	}

	metrics, metricsErr := em.collectMetrics(ctx, results)
	if metricsErr != nil {
		log.Printf("Warning: Failed to collect metrics: %v", metricsErr)
	} else {
		metrics.EvaluationID = job.ID
		evaluation.Metrics = metrics
	}

	// 3. 收集和分析指标
	if evaluation.Metrics != nil {
		if job.Config.EnableMetrics {
			if err := em.metricsStore.Store(job.ID, evaluation.Metrics); err != nil {
				log.Printf("Warning: Failed to store metrics: %v", err)
			}
		}

		evaluation.Analysis = em.analyzer.Analyze(evaluation.Metrics)
		evaluation.Agent = em.buildAgentProfile(job.Config.AgentID, evaluation.Metrics, autoScores, results)

		if job.Config.EnableMetrics {
			if err := em.generateReport(ctx, evaluation, job.Config); err != nil {
				log.Printf("Warning: Failed to generate report: %v", err)
			}
		}
	}

	// 如果指标不可用，仍然生成一个基本分析结果，避免调用方出现空指针
	if evaluation.Analysis == nil {
		evaluation.Analysis = em.fallbackAnalysis(results, autoScores)
	}
	if evaluation.Agent == nil {
		evaluation.Agent = em.buildAgentProfile(job.Config.AgentID, metrics, autoScores, results)
	}

	if evaluation.Agent != nil && em.agentStore != nil {
		if profile, err := em.agentStore.UpsertProfile(evaluation.Agent); err != nil {
			log.Printf("Warning: Failed to store agent profile: %v", err)
		} else {
			evaluation.Agent = profile
		}
		if err := em.agentStore.StoreEvaluation(job.Config.AgentID, evaluation); err != nil {
			log.Printf("Warning: Failed to store agent evaluation: %v", err)
		}
	}

	// 最终回写以确保最新的画像和评分可见
	em.setJobResults(job.ID, evaluation)
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
func (em *EvaluationManager) runEvaluation(ctx context.Context, instances []swe_bench.Instance, config *EvaluationConfig) ([]swe_bench.WorkerResult, error) {
	// 创建批处理配置
	batchConfig := &swe_bench.BatchConfig{
		NumWorkers: config.MaxWorkers,
		OutputPath: filepath.Join(config.OutputDir, "results.json"),
	}

	// 设置超时
	if config.TimeoutPerTask > 0 {
		batchConfig.Agent.Timeout = int(config.TimeoutPerTask.Seconds())
	}

	// 使用现有批处理器
	if em.batchRunner == nil {
		em.batchRunner = swe_bench.NewBatchProcessor(batchConfig)
	}

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

// generateReport 生成报告
func (em *EvaluationManager) generateReport(ctx context.Context, report *EvaluationResults, config *EvaluationConfig) error {
	outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("report_%s.md", report.JobID))
	return em.reporter.GenerateReport(report, outputPath, config.OutputDir)
}

// scoreResults 基于状态、耗时和错误为每个任务生成自动评分
func (em *EvaluationManager) scoreResults(results []swe_bench.WorkerResult) []AutoScore {
	scores := make([]AutoScore, 0, len(results))

	for _, result := range results {
		score := 50.0
		reason := "基础分"

		switch result.Status {
		case swe_bench.StatusCompleted:
			score += 40
			reason = "任务完成"
		case swe_bench.StatusTimeout:
			score -= 25
			reason = "超时"
		case swe_bench.StatusFailed:
			score -= 20
			reason = "执行失败"
		}

		durationSec := result.Duration.Seconds()
		if durationSec > 0 {
			// 快速完成加分，过慢扣分
			if durationSec <= 60 {
				score += 5
			} else if durationSec > 600 {
				score -= 5
			}
		}

		if result.RetryCount > 0 {
			score -= float64(result.RetryCount) * 2
			reason += "，多次重试"
		}

		if result.Error != "" {
			score -= 5
			if reason == "基础分" {
				reason = "存在错误"
			}
		}

		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}

		scores = append(scores, AutoScore{
			TaskID:     result.TaskID,
			InstanceID: result.InstanceID,
			Score:      score,
			Grade:      em.scoreToGrade(score),
			Reason:     reason,
		})
	}

	return scores
}

func (em *EvaluationManager) scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	default:
		return "D"
	}
}

func (em *EvaluationManager) fallbackAnalysis(results []swe_bench.WorkerResult, scores []AutoScore) *AnalysisResult {
	summary := fallbackAnalysisResult{}

	if len(scores) > 0 {
		var total float64
		for _, score := range scores {
			total += score.Score
		}
		summary.overallScore = total / float64(len(scores)) / 100
	}

	if len(results) > 0 {
		var completed int
		for _, result := range results {
			if result.Status == swe_bench.StatusCompleted {
				completed++
			}
		}
		summary.successRate = float64(completed) / float64(len(results))
		if summary.overallScore == 0 {
			summary.overallScore = summary.successRate
		}
	}

	// 如果没有任何数据，返回空以避免误导
	if summary.overallScore == 0 && summary.successRate == 0 {
		return nil
	}

	return &AnalysisResult{
		Summary: AnalysisSummary{
			OverallScore:     summary.overallScore,
			PerformanceGrade: em.scoreToGrade(summary.overallScore * 100),
			KeyStrengths:     []string{},
			KeyWeaknesses:    []string{},
			RiskLevel:        "medium",
		},
		Timestamp: time.Now(),
	}
}

func (em *EvaluationManager) buildAgentProfile(agentID string, metrics *EvaluationMetrics, scores []AutoScore, results []swe_bench.WorkerResult) *AgentProfile {
	if agentID == "" {
		return nil
	}

	profile := &AgentProfile{
		AgentID:         agentID,
		EvaluationCount: 1,
		LastEvaluated:   time.Now(),
		PreferredTools:  make(map[string]int),
		CommonErrors:    make(map[string]int),
	}

	// 优先使用完整指标；否则根据结果与自动评分推导基础画像
	if metrics != nil {
		profile.AvgSuccessRate = metrics.Performance.SuccessRate
		profile.AvgExecTime = metrics.Performance.AvgExecutionTime
		profile.AvgCostPerTask = metrics.Resources.AvgCostPerTask
		if metrics.Quality.SolutionQuality > 0 {
			profile.AvgQualityScore = metrics.Quality.SolutionQuality
		}
	}

	if len(scores) > 0 {
		var total float64
		for _, score := range scores {
			total += score.Score
		}
		// 自动评分以 0-1 的比例纳入质量得分
		profile.AvgQualityScore = total / float64(len(scores)) / 100
	}

	if profile.AvgSuccessRate == 0 && len(results) > 0 {
		var success int
		for _, result := range results {
			if result.Status == swe_bench.StatusCompleted {
				success++
			}
		}
		profile.AvgSuccessRate = float64(success) / float64(len(results))
	}

	if profile.AvgExecTime == 0 && len(results) > 0 {
		var total time.Duration
		for _, result := range results {
			total += result.Duration
		}
		profile.AvgExecTime = total / time.Duration(len(results))
	}

	if profile.AvgCostPerTask == 0 && len(results) > 0 {
		var total float64
		for _, result := range results {
			total += result.Cost
		}
		profile.AvgCostPerTask = total / float64(len(results))
	}

	return profile
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
	em.ensureHydrated()
	em.mu.RLock()
	defer em.mu.RUnlock()

	if job, exists := em.activeJobs[jobID]; exists {
		return job.Status
	}
	return JobStatusFailed
}

// GetJobResults 获取任务结果
func (em *EvaluationManager) GetJobResults(jobID string) (*EvaluationResults, error) {
	job, err := em.GetJob(jobID)
	if err != nil {
		return nil, err
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

// GetJob 获取任务详情快照
func (em *EvaluationManager) GetJob(jobID string) (*EvaluationJob, error) {
	em.ensureHydrated()

	em.mu.RLock()
	job, exists := em.activeJobs[jobID]
	em.mu.RUnlock()

	if exists {
		return em.cloneJob(job), nil
	}

	// Attempt to restore from persistence for historical lookups.
	if restored, err := em.loadPersistedEvaluation(jobID); err == nil {
		return restored, nil
	}

	return nil, fmt.Errorf("job not found: %s", jobID)
}

// DeleteEvaluation removes a persisted evaluation snapshot and evicts it from memory.
func (em *EvaluationManager) DeleteEvaluation(jobID string) error {
	em.ensureHydrated()

	if em.agentStore == nil {
		return fmt.Errorf("agent store not configured")
	}

	if err := em.agentStore.RemoveEvaluation(jobID); err != nil {
		return err
	}

	em.mu.Lock()
	defer em.mu.Unlock()
	delete(em.activeJobs, jobID)
	return nil
}

// GetAgentProfile loads the persisted profile for an agent, if available.
func (em *EvaluationManager) GetAgentProfile(agentID string) (*AgentProfile, error) {
	if em.agentStore == nil {
		return nil, fmt.Errorf("agent store not configured")
	}
	return em.agentStore.LoadProfile(agentID)
}

// ListAgentProfiles returns every known agent profile snapshot.
func (em *EvaluationManager) ListAgentProfiles() ([]*AgentProfile, error) {
	if em.agentStore == nil {
		return nil, fmt.Errorf("agent store not configured")
	}
	return em.agentStore.ListProfiles()
}

// ListAgentEvaluations returns stored evaluation snapshots for an agent.
func (em *EvaluationManager) ListAgentEvaluations(agentID string) ([]*EvaluationResults, error) {
	if em.agentStore == nil {
		return nil, fmt.Errorf("agent store not configured")
	}
	return em.agentStore.ListEvaluations(agentID)
}

// ListAllEvaluations returns stored evaluation snapshots across agents, optionally capped by limit.
func (em *EvaluationManager) ListAllEvaluations(limit int) ([]*EvaluationResults, error) {
	if em.agentStore == nil {
		return nil, fmt.Errorf("agent store not configured")
	}
	return em.agentStore.ListRecentEvaluations(limit)
}

// QueryEvaluations returns stored evaluation snapshots filtered by the provided query.
func (em *EvaluationManager) QueryEvaluations(query EvaluationQuery) ([]*EvaluationResults, error) {
	if em.agentStore == nil {
		return nil, fmt.Errorf("agent store not configured")
	}
	return em.agentStore.QueryEvaluations(query)
}

// ListJobs 返回所有已知任务快照
func (em *EvaluationManager) ListJobs() []*EvaluationJob {
	em.ensureHydrated()
	em.mu.RLock()
	defer em.mu.RUnlock()

	jobs := make([]*EvaluationJob, 0, len(em.activeJobs))
	for _, job := range em.activeJobs {
		jobs = append(jobs, em.cloneJob(job))
	}

	return jobs
}

// HydrateFromStore loads previously persisted evaluations into the in-memory job index.
func (em *EvaluationManager) HydrateFromStore() error {
	if em.agentStore == nil {
		return nil
	}

	evaluations, err := em.agentStore.ListAllEvaluations()
	if err != nil {
		return err
	}

	em.mu.Lock()
	defer em.mu.Unlock()
	em.hydrated = true

	for _, eval := range evaluations {
		if eval == nil || eval.JobID == "" {
			continue
		}
		if _, exists := em.activeJobs[eval.JobID]; exists {
			continue
		}

		job := &EvaluationJob{
			ID:        eval.JobID,
			Status:    JobStatusCompleted,
			Config:    em.cloneConfig(eval.Config),
			Results:   eval,
			StartTime: eval.Timestamp,
			EndTime:   eval.Timestamp,
		}
		em.activeJobs[job.ID] = job
	}

	return nil
}

func (em *EvaluationManager) ensureHydrated() {
	em.mu.RLock()
	hydrated := em.hydrated
	em.mu.RUnlock()
	if hydrated {
		return
	}
	_ = em.HydrateFromStore()
}

func (em *EvaluationManager) nextJobID() string {
	seq := em.jobSeq.Add(1)
	return fmt.Sprintf("eval_%d_%d", time.Now().Unix(), seq)
}

func (em *EvaluationManager) loadPersistedEvaluation(jobID string) (*EvaluationJob, error) {
	if em.agentStore == nil {
		return nil, fmt.Errorf("agent store not configured")
	}

	eval, err := em.agentStore.LoadEvaluation(jobID)
	if err != nil {
		return nil, err
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	job := &EvaluationJob{
		ID:        eval.JobID,
		Status:    JobStatusCompleted,
		Config:    em.cloneConfig(eval.Config),
		Results:   eval,
		StartTime: eval.Timestamp,
		EndTime:   eval.Timestamp,
	}
	em.activeJobs[job.ID] = job
	return em.cloneJob(job), nil
}

func (em *EvaluationManager) cloneJob(job *EvaluationJob) *EvaluationJob {
	if job == nil {
		return nil
	}

	clone := *job
	if job.Config != nil {
		clone.Config = em.cloneConfig(job.Config)
	}
	return &clone
}

func (em *EvaluationManager) cloneConfig(config *EvaluationConfig) *EvaluationConfig {
	if config == nil {
		return nil
	}
	clone := *config
	return &clone
}

func (em *EvaluationManager) setJobResults(jobID string, results *EvaluationResults) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if job, ok := em.activeJobs[jobID]; ok {
		job.Results = results
	}
}

func (em *EvaluationManager) setJobError(jobID string, err error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if job, ok := em.activeJobs[jobID]; ok {
		job.Error = err
	}
}
