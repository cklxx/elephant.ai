package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// ParallelConfig - Configuration for parallel execution (simplified)
type ParallelConfig struct {
	MaxWorkers    int           `json:"max_workers"`    // Default: 3-5 (not 10-50)
	TaskTimeout   time.Duration `json:"task_timeout"`   // Default: 2min
	EnableMetrics bool          `json:"enable_metrics"` // Default: true
}

// DefaultParallelConfig - Returns default configuration
func DefaultParallelConfig() *ParallelConfig {
	return &ParallelConfig{
		MaxWorkers:    3,
		TaskTimeout:   2 * time.Minute,
		EnableMetrics: true,
	}
}

// SimpleParallelSubAgent - Simplified parallel execution system
// Uses Go's built-in concurrency primitives and existing SubAgent implementation
type SimpleParallelSubAgent struct {
	parentCore *ReactCore
	config     *ParallelConfig
}

// NewSimpleParallelSubAgent - Creates a new simplified parallel subagent
func NewSimpleParallelSubAgent(parentCore *ReactCore, config *ParallelConfig) (*SimpleParallelSubAgent, error) {
	if parentCore == nil {
		return nil, fmt.Errorf("parentCore cannot be nil")
	}

	if config == nil {
		config = DefaultParallelConfig()
	}

	// Validate configuration
	if config.MaxWorkers <= 0 || config.MaxWorkers > 10 {
		return nil, fmt.Errorf("maxWorkers must be between 1 and 10, got %d", config.MaxWorkers)
	}

	if config.TaskTimeout <= 0 {
		return nil, fmt.Errorf("taskTimeout must be positive, got %v", config.TaskTimeout)
	}

	subAgentLog("INFO", "Creating SimpleParallelSubAgent with %d workers, timeout %v",
		config.MaxWorkers, config.TaskTimeout)

	return &SimpleParallelSubAgent{
		parentCore: parentCore,
		config:     config,
	}, nil
}

// ExecuteTasksParallel - Execute multiple tasks in parallel with result ordering
// This is the core implementation following the simplified Phase 1 approach
func (spa *SimpleParallelSubAgent) ExecuteTasksParallel(
	ctx context.Context,
	tasks []string,
	streamCallback StreamCallback,
) ([]*SubAgentResult, error) {
	if len(tasks) == 0 {
		subAgentLog("INFO", "No tasks to execute")
		return []*SubAgentResult{}, nil
	}

	subAgentLog("INFO", "Starting parallel execution of %d tasks with %d workers",
		len(tasks), spa.config.MaxWorkers)

	startTime := time.Now()

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, spa.config.TaskTimeout)
	defer cancel()

	// Use errgroup for structured concurrency
	g, gCtx := errgroup.WithContext(execCtx)

	// Simple semaphore for concurrency control using buffered channel
	sem := make(chan struct{}, spa.config.MaxWorkers)

	// Pre-allocate results slice to maintain order
	results := make([]*SubAgentResult, len(tasks))

	// Stream callback for parallel execution start
	if streamCallback != nil {
		streamCallback(StreamChunk{
			Type:    "parallel_start",
			Content: fmt.Sprintf("🚀 Starting parallel execution: %d tasks, %d workers", len(tasks), spa.config.MaxWorkers),
			Metadata: map[string]any{
				"task_count":  len(tasks),
				"max_workers": spa.config.MaxWorkers,
				"start_time":  startTime,
			},
		})
	}

	// Launch a goroutine for each task
	for i, task := range tasks {
		i, task := i, task // Capture loop variables

		g.Go(func() error {
			// Comprehensive panic recovery for subagent execution
			defer func() {
				if r := recover(); r != nil {
					subAgentLog("ERROR", "Task %d panicked: %v", i, r)
					// Create failed result on panic
					results[i] = &SubAgentResult{
						Success:       false,
						TaskCompleted: false,
						Result:        "",
						SessionID:     fmt.Sprintf("panic-task-%d", i),
						ErrorMessage:  fmt.Sprintf("panic during execution: %v", r),
						Duration:      time.Since(startTime).Milliseconds(),
					}
					if streamCallback != nil {
						streamCallback(StreamChunk{
							Type:     "task_panic",
							Content:  fmt.Sprintf("⚠️  Task %d recovered from panic: %v", i, r),
							Metadata: map[string]any{"task_index": i, "panic_value": fmt.Sprintf("%v", r)},
						})
					}
				}
			}()

			// Acquire semaphore slot
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }() // Release semaphore
			case <-gCtx.Done():
				return gCtx.Err()
			}

			// Log task start
			subAgentLog("DEBUG", "Starting task %d: %s", i, task)

			// Create subagent with existing pattern (no modifications needed)
			config := &SubAgentConfig{
				MaxIterations: 50,
				ContextCache:  true,
			}

			subAgent, err := NewSubAgent(spa.parentCore, config)
			if err != nil {
				subAgentLog("ERROR", "Failed to create subagent for task %d: %v", i, err)
				return fmt.Errorf("failed to create subagent for task %d: %w", i, err)
			}

			// Create task-specific stream callback with worker ID prefix
			var taskStreamCallback StreamCallback
			if streamCallback != nil {
				taskStreamCallback = func(chunk StreamChunk) {
					// Prefix stream output with worker/task identifier
					chunk.Content = fmt.Sprintf("[Task-%d] %s", i, chunk.Content)
					if chunk.Metadata == nil {
						chunk.Metadata = make(map[string]any)
					}
					chunk.Metadata["task_index"] = i
					chunk.Metadata["worker_id"] = fmt.Sprintf("worker-%d", i)
					streamCallback(chunk)
				}
			}

			// Execute task using existing SubAgent implementation with enhanced error handling
			result, err := subAgent.ExecuteTask(gCtx, task, taskStreamCallback)
			if err != nil {
				subAgentLog("ERROR", "Task %d failed: %v", i, err)

				// Enhanced error categorization and handling
				errorMsg := err.Error()
				errorCategory := "general_error"
				partialResult := extractPartialResult(err)

				if isContextLimitError(err) {
					subAgentLog("WARN", "Task %d hit context limits, task partially completed", i)
					errorMsg = fmt.Sprintf("Task partially completed due to context limits: %v", err)
					errorCategory = "context_limit"
					partialResult = "Task execution reached context limits but may have produced partial results before interruption."
				} else if strings.Contains(strings.ToLower(errorMsg), "panic") {
					subAgentLog("WARN", "Task %d recovered from panic, system stable", i)
					errorCategory = "panic_recovered"
					partialResult = "Task execution was interrupted by system panic but recovered safely."
				} else if strings.Contains(strings.ToLower(errorMsg), "timeout") {
					subAgentLog("WARN", "Task %d timed out, may have partial work", i)
					errorCategory = "timeout"
					partialResult = "Task execution timed out. Some work may have been completed before timeout."
				}

				// Send error notification through stream callback
				if taskStreamCallback != nil {
					taskStreamCallback(StreamChunk{
						Type:    "parallel_task_error",
						Content: fmt.Sprintf("⚠️ Task %d encountered %s but recovered gracefully", i, errorCategory),
						Metadata: map[string]any{
							"task_index":         i,
							"error_category":     errorCategory,
							"has_partial_result": partialResult != "",
						},
					})
				}

				// Create enhanced result with error categorization
				results[i] = &SubAgentResult{
					Success:       false,
					TaskCompleted: false,
					Result:        partialResult,
					SessionID:     subAgent.GetSessionID(),
					ErrorMessage:  fmt.Sprintf("[%s] %s", errorCategory, errorMsg),
					Duration:      time.Since(startTime).Milliseconds(),
				}
				return nil // Don't fail entire execution for individual task failure
			}

			// Enhanced success handling with compression monitoring
			if result != nil {
				// Store result in correct position to maintain order
				results[i] = result

				// Send success notification with details
				if taskStreamCallback != nil {
					taskStreamCallback(StreamChunk{
						Type:    "parallel_task_success",
						Content: fmt.Sprintf("✅ Task %d completed successfully (tokens: %d, duration: %dms)", i, result.TokensUsed, result.Duration),
						Metadata: map[string]any{
							"task_index":     i,
							"tokens_used":    result.TokensUsed,
							"duration_ms":    result.Duration,
							"task_completed": result.TaskCompleted,
						},
					})
				}

				subAgentLog("INFO", "Task %d completed successfully: %d tokens, %dms duration, result: %.100s...",
					i, result.TokensUsed, result.Duration, result.Result)
			} else {
				subAgentLog("WARN", "Task %d returned nil result despite no error", i)
				results[i] = &SubAgentResult{
					Success:       false,
					TaskCompleted: false,
					Result:        "Task completed but returned no result",
					SessionID:     subAgent.GetSessionID(),
					ErrorMessage:  "nil result returned",
					Duration:      time.Since(startTime).Milliseconds(),
				}
			}

			return nil
		})
	}

	// Wait for all tasks to complete
	if err := g.Wait(); err != nil {
		subAgentLog("ERROR", "Parallel execution failed: %v", err)

		if streamCallback != nil {
			streamCallback(StreamChunk{
				Type:     "parallel_error",
				Content:  fmt.Sprintf("❌ Parallel execution failed: %v", err),
				Metadata: map[string]any{"error": err.Error()},
			})
		}

		return nil, fmt.Errorf("parallel execution failed: %w", err)
	}

	totalDuration := time.Since(startTime)

	// Calculate summary statistics
	successCount := 0
	totalTokens := 0
	for _, result := range results {
		if result != nil {
			if result.Success {
				successCount++
			}
			totalTokens += result.TokensUsed
		}
	}

	subAgentLog("INFO", "Parallel execution completed: %d/%d successful, %d total tokens, %v duration",
		successCount, len(tasks), totalTokens, totalDuration)

	// Stream completion callback
	if streamCallback != nil {
		streamCallback(StreamChunk{
			Type:    "parallel_complete",
			Content: fmt.Sprintf("✅ Parallel execution completed: %d/%d successful in %v", successCount, len(tasks), totalDuration),
			Metadata: map[string]any{
				"success_count":   successCount,
				"total_tasks":     len(tasks),
				"total_tokens":    totalTokens,
				"duration_ms":     totalDuration.Milliseconds(),
				"completion_time": time.Now(),
			},
		})
	}

	return results, nil
}

// ExecuteTasksParallelWithWorkerLimit - Convenience method with explicit worker limit
func (spa *SimpleParallelSubAgent) ExecuteTasksParallelWithWorkerLimit(
	ctx context.Context,
	tasks []string,
	maxWorkers int,
	streamCallback StreamCallback,
) ([]*SubAgentResult, error) {
	// Temporarily override config for this execution
	originalMaxWorkers := spa.config.MaxWorkers
	spa.config.MaxWorkers = maxWorkers
	defer func() { spa.config.MaxWorkers = originalMaxWorkers }()

	return spa.ExecuteTasksParallel(ctx, tasks, streamCallback)
}

// GetConfig - Returns current configuration
func (spa *SimpleParallelSubAgent) GetConfig() *ParallelConfig {
	return spa.config
}

// UpdateConfig - Updates configuration with validation
func (spa *SimpleParallelSubAgent) UpdateConfig(config *ParallelConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.MaxWorkers <= 0 || config.MaxWorkers > 10 {
		return fmt.Errorf("maxWorkers must be between 1 and 10, got %d", config.MaxWorkers)
	}

	if config.TaskTimeout <= 0 {
		return fmt.Errorf("taskTimeout must be positive, got %v", config.TaskTimeout)
	}

	spa.config = config
	subAgentLog("INFO", "Updated parallel config: %d workers, %v timeout",
		config.MaxWorkers, config.TaskTimeout)

	return nil
}

// ExecuteTasksParallelFromTool - Tool interface adapter for parallel execution
// This method implements the ParallelSubAgentExecutor interface for tool integration
func (spa *SimpleParallelSubAgent) ExecuteTasksParallelFromTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Parse tasks
	tasksRaw, ok := args["tasks"]
	if !ok {
		return nil, fmt.Errorf("tasks parameter is required")
	}

	tasksSlice, ok := tasksRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("tasks must be an array")
	}

	tasks := make([]string, len(tasksSlice))
	for i, task := range tasksSlice {
		taskStr, ok := task.(string)
		if !ok {
			return nil, fmt.Errorf("task %d must be a string", i)
		}
		tasks[i] = taskStr
	}

	// Parse optional parameters
	if maxWorkersRaw, exists := args["max_workers"]; exists {
		if maxWorkers, ok := maxWorkersRaw.(int); ok {
			spa.config.MaxWorkers = maxWorkers
		}
	}

	if taskTimeoutRaw, exists := args["task_timeout"]; exists {
		if timeoutStr, ok := taskTimeoutRaw.(string); ok {
			if timeout, err := time.ParseDuration(timeoutStr); err == nil {
				spa.config.TaskTimeout = timeout
			}
		}
	}

	// Execute tasks
	results, err := spa.ExecuteTasksParallel(ctx, tasks, nil)
	if err != nil {
		return nil, err
	}

	// Return results in a format compatible with the tool interface
	return map[string]interface{}{
		"results":      results,
		"task_count":   len(tasks),
		"success_rate": float64(spa.countSuccessful(results)) / float64(len(results)),
	}, nil
}

// countSuccessful - Helper to count successful results
func (spa *SimpleParallelSubAgent) countSuccessful(results []*SubAgentResult) int {
	count := 0
	for _, result := range results {
		if result != nil && result.Success {
			count++
		}
	}
	return count
}

// isContextLimitError - Check if error is related to API context limits
func isContextLimitError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "context") &&
		(strings.Contains(errorStr, "limit") ||
			strings.Contains(errorStr, "exceed") ||
			strings.Contains(errorStr, "too large") ||
			strings.Contains(errorStr, "maximum")) ||
		strings.Contains(errorStr, "token") &&
			(strings.Contains(errorStr, "limit") ||
				strings.Contains(errorStr, "exceed"))
}

// extractPartialResult - Extract any partial results from error for graceful degradation
func extractPartialResult(err error) string {
	if err == nil {
		return ""
	}

	errorStr := err.Error()
	if isContextLimitError(err) {
		return "Task execution was interrupted due to API context limits. Partial work may have been completed."
	}

	// Look for any useful information in the error message
	if strings.Contains(errorStr, "panic") {
		return "Task execution was interrupted unexpectedly. System recovered safely."
	}

	return "Task failed to complete"
}
