package swe_bench

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"alex/internal/shared/async"
)

// BatchProcessorImpl implements the BatchProcessor interface
type BatchProcessorImpl struct {
	config       *BatchConfig
	dataLoader   DatasetLoader
	workerPool   WorkerPool
	resultWriter ResultWriter
	progress     ProgressReporter
	monitor      Monitor

	// State management
	mu             sync.RWMutex
	isRunning      bool
	totalTasks     int
	completedTasks int
	failedTasks    int
	startTime      time.Time
	results        []WorkerResult
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(config *BatchConfig) *BatchProcessorImpl {
	return &BatchProcessorImpl{
		config:       config,
		dataLoader:   NewDatasetLoader(),
		workerPool:   NewWorkerPool(config.NumWorkers),
		resultWriter: NewResultWriter(),
		progress:     NewProgressReporter(),
		monitor:      NewMonitor(),
		results:      make([]WorkerResult, 0),
	}
}

// ProcessBatch processes a batch of instances
func (bp *BatchProcessorImpl) ProcessBatch(ctx context.Context, instances []Instance, config *BatchConfig) (*BatchResult, error) {
	bp.mu.Lock()
	bp.isRunning = true
	bp.totalTasks = len(instances)
	bp.completedTasks = 0
	bp.failedTasks = 0
	bp.startTime = time.Now()
	bp.results = make([]WorkerResult, 0, len(instances))
	bp.mu.Unlock()

	// Start monitoring and progress reporting
	if err := bp.monitor.StartMonitoring(ctx); err != nil {
		log.Printf("Warning: Failed to start monitoring: %v", err)
	}
	defer func() {
		if err := bp.monitor.StopMonitoring(); err != nil {
			log.Printf("Warning: Failed to stop monitoring: %v", err)
		}
	}()

	if err := bp.progress.Start(ctx); err != nil {
		log.Printf("Warning: Failed to start progress reporting: %v", err)
	}
	defer func() {
		if err := bp.progress.Stop(); err != nil {
			log.Printf("Warning: Failed to stop progress: %v", err)
		}
	}()

	// Start worker pool
	if err := bp.workerPool.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start worker pool: %w", err)
	}
	defer func() {
		if err := bp.workerPool.Stop(ctx); err != nil {
			log.Printf("Warning: Failed to stop worker pool: %v", err)
		}
	}()

	// Submit tasks to worker pool
	async.Go(panicLogger{}, "swe-bench.submit", func() {
		bp.submitTasks(ctx, instances)
	})

	// Collect results
	results, err := bp.collectResults(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect results: %w", err)
	}

	// Create batch result
	batchResult := bp.createBatchResult(results)

	// Write final results
	if err := bp.resultWriter.WriteResults(ctx, batchResult, config.OutputPath); err != nil {
		log.Printf("Warning: Failed to write results: %v", err)
	}

	bp.mu.Lock()
	bp.isRunning = false
	bp.mu.Unlock()

	return batchResult, nil
}

// ProcessInstance processes a single instance
func (bp *BatchProcessorImpl) ProcessInstance(ctx context.Context, instance Instance, config *BatchConfig) (*WorkerResult, error) {
	// Create agent
	agentFactory := NewAlexAgentFactory()
	agent, err := agentFactory.CreateAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}
	defer func() {
		if err := agent.Close(); err != nil {
			log.Printf("Warning: Failed to close agent: %v", err)
		}
	}()

	// Process instance
	startTime := time.Now()
	result, err := agent.ProcessInstance(ctx, instance)
	if err != nil {
		return &WorkerResult{
			InstanceID: instance.ID,
			Status:     StatusFailed,
			StartTime:  startTime,
			EndTime:    time.Now(),
			Duration:   time.Since(startTime),
			Error:      err.Error(),
			ErrorType:  "processing_error",
		}, nil
	}

	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

	return result, nil
}

// Resume resumes processing from a previous state
func (bp *BatchProcessorImpl) Resume(ctx context.Context, resultPath string, config *BatchConfig) (*BatchResult, error) {
	// Read previous results
	previousResult, err := bp.resultWriter.ReadResults(ctx, resultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read previous results: %w", err)
	}

	// Load all instances
	instances, err := bp.dataLoader.LoadInstances(ctx, config.Instances)
	if err != nil {
		return nil, fmt.Errorf("failed to load instances: %w", err)
	}

	// Find completed instance IDs
	completedIDs := make(map[string]bool)
	for _, result := range previousResult.Results {
		if result.Status == StatusCompleted {
			completedIDs[result.InstanceID] = true
		}
	}

	// Filter out completed instances
	remainingInstances := make([]Instance, 0)
	for _, instance := range instances {
		if !completedIDs[instance.ID] {
			remainingInstances = append(remainingInstances, instance)
		}
	}

	log.Printf("Resuming batch processing: %d completed, %d remaining",
		len(completedIDs), len(remainingInstances))

	// Process remaining instances
	newResult, err := bp.ProcessBatch(ctx, remainingInstances, config)
	if err != nil {
		return nil, fmt.Errorf("failed to process remaining instances: %w", err)
	}

	// Merge results
	mergedResult := bp.mergeResults(previousResult, newResult)

	return mergedResult, nil
}

// submitTasks submits tasks to the worker pool
func (bp *BatchProcessorImpl) submitTasks(ctx context.Context, instances []Instance) {
	for i, instance := range instances {
		select {
		case <-ctx.Done():
			return
		default:
			// Apply random delay if configured
			if bp.config.MaxDelay > 0 {
				delay := time.Duration(rand.Int63n(int64(bp.config.MaxDelay)))
				time.Sleep(delay)
			}

			task := WorkerTask{
				ID:         fmt.Sprintf("task_%d", i),
				Instance:   instance,
				Config:     bp.config,
				RetryCount: 0,
				CreatedAt:  time.Now(),
			}

			if err := bp.workerPool.SubmitTask(task); err != nil {
				log.Printf("Failed to submit task %s: %v", task.ID, err)
				// Handle task submission failure
				bp.handleTaskFailure(task, err)
			}
		}
	}
}

// collectResults collects results from the worker pool
func (bp *BatchProcessorImpl) collectResults(ctx context.Context) ([]WorkerResult, error) {
	results := make([]WorkerResult, 0, bp.totalTasks)
	resultsChan := bp.workerPool.GetResults()

	for len(results) < bp.totalTasks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case result := <-resultsChan:
			results = append(results, result)

			bp.mu.Lock()
			if result.Status == StatusCompleted {
				bp.completedTasks++
			} else {
				bp.failedTasks++
			}
			bp.mu.Unlock()

			// Update progress
			bp.updateProgress()

			// Handle retries if needed
			if result.Status == StatusFailed && result.RetryCount < bp.config.MaxRetries {
				bp.retryTask(result)
			}

			// Write partial results
			if err := bp.resultWriter.AppendResult(ctx, result, bp.config.OutputPath); err != nil {
				log.Printf("Warning: Failed to write partial result: %v", err)
			}

			// Record metrics
			bp.recordMetrics(result)

			// Check fail-fast condition
			if bp.config.FailFast && result.Status == StatusFailed {
				return results, fmt.Errorf("fail-fast enabled: stopping on first failure")
			}
		}
	}

	return results, nil
}

// createBatchResult creates a BatchResult from individual results
func (bp *BatchProcessorImpl) createBatchResult(results []WorkerResult) *BatchResult {
	endTime := time.Now()
	duration := endTime.Sub(bp.startTime)

	var totalTokens int
	var totalCost float64
	var totalDuration time.Duration
	errorSummary := make(map[string]int)

	completedCount := 0
	failedCount := 0

	for _, result := range results {
		if result.Status == StatusCompleted {
			completedCount++
		} else {
			failedCount++
			if result.ErrorType != "" {
				errorSummary[result.ErrorType]++
			}
		}

		totalTokens += result.TokensUsed
		totalCost += result.Cost
		totalDuration += result.Duration
	}

	var avgDuration time.Duration
	if len(results) > 0 {
		avgDuration = totalDuration / time.Duration(len(results))
	}

	successRate := float64(completedCount) / float64(len(results)) * 100

	return &BatchResult{
		Config:         bp.config,
		StartTime:      bp.startTime,
		EndTime:        endTime,
		Duration:       duration,
		TotalTasks:     len(results),
		CompletedTasks: completedCount,
		FailedTasks:    failedCount,
		SuccessRate:    successRate,
		TotalTokens:    totalTokens,
		TotalCost:      totalCost,
		AvgDuration:    avgDuration,
		Results:        results,
		ErrorSummary:   errorSummary,
	}
}

// updateProgress updates progress reporting
func (bp *BatchProcessorImpl) updateProgress() {
	bp.mu.RLock()
	completed := bp.completedTasks
	failed := bp.failedTasks
	total := bp.totalTasks
	running := total - completed - failed
	remaining := total - completed - failed
	bp.mu.RUnlock()

	var successRate float64
	if completed+failed > 0 {
		successRate = float64(completed) / float64(completed+failed) * 100
	}

	var avgDuration time.Duration
	if completed > 0 && len(bp.results) > 0 {
		var totalDuration time.Duration
		for _, result := range bp.results {
			if result.Status == StatusCompleted {
				totalDuration += result.Duration
			}
		}
		avgDuration = totalDuration / time.Duration(completed)
	}

	var estimatedETA time.Duration
	if avgDuration > 0 && remaining > 0 {
		estimatedETA = avgDuration * time.Duration(remaining)
	}

	update := ProgressUpdate{
		Timestamp:    time.Now(),
		Total:        total,
		Completed:    completed,
		Failed:       failed,
		Running:      running,
		Remaining:    remaining,
		SuccessRate:  successRate,
		AvgDuration:  avgDuration,
		EstimatedETA: estimatedETA,
	}

	if err := bp.progress.Update(update); err != nil {
		log.Printf("Warning: Failed to update progress: %v", err)
	}
}

// handleTaskFailure handles task submission failures
func (bp *BatchProcessorImpl) handleTaskFailure(task WorkerTask, err error) {
	result := WorkerResult{
		TaskID:     task.ID,
		InstanceID: task.Instance.ID,
		Status:     StatusFailed,
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		Duration:   0,
		Error:      err.Error(),
		ErrorType:  "submission_error",
		RetryCount: task.RetryCount,
	}

	bp.mu.Lock()
	bp.failedTasks++
	bp.results = append(bp.results, result)
	bp.mu.Unlock()
}

// retryTask retries a failed task
func (bp *BatchProcessorImpl) retryTask(result WorkerResult) {
	// Find original instance
	var instance Instance
	// This would need to be implemented based on how we store original instances

	task := WorkerTask{
		ID:         fmt.Sprintf("%s_retry_%d", result.TaskID, result.RetryCount+1),
		Instance:   instance,
		Config:     bp.config,
		RetryCount: result.RetryCount + 1,
		CreatedAt:  time.Now(),
	}

	if err := bp.workerPool.SubmitTask(task); err != nil {
		log.Printf("Failed to retry task %s: %v", task.ID, err)
	}
}

// recordMetrics records metrics for monitoring
func (bp *BatchProcessorImpl) recordMetrics(result WorkerResult) {
	tags := map[string]string{
		"status":      string(result.Status),
		"instance_id": result.InstanceID,
	}

	_ = bp.monitor.RecordMetric("task_duration_seconds", result.Duration.Seconds(), tags)
	_ = bp.monitor.RecordMetric("task_tokens_used", float64(result.TokensUsed), tags)
	_ = bp.monitor.RecordMetric("task_cost", result.Cost, tags)

	if result.Status == StatusCompleted {
		_ = bp.monitor.RecordMetric("task_success", 1, tags)
	} else {
		_ = bp.monitor.RecordMetric("task_failure", 1, tags)
	}
}

// mergeResults merges two batch results
func (bp *BatchProcessorImpl) mergeResults(previous *BatchResult, new *BatchResult) *BatchResult {
	allResults := make([]WorkerResult, 0, len(previous.Results)+len(new.Results))
	allResults = append(allResults, previous.Results...)
	allResults = append(allResults, new.Results...)

	return bp.createBatchResult(allResults)
}
