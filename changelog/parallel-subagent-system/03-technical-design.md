# Technical Design: Parallel Subagent System with Mutex Locks

## System Overview

The parallel subagent system implements an orchestrator-worker pattern with semaphore-controlled concurrency, structured error handling, and sequential result ordering. It extends the existing Alex-Code subagent architecture while maintaining backward compatibility.

## Core Architecture Components

### 1. Parallel Subagent Manager

```go
// ParallelSubAgentManager - Main orchestrator for parallel subagent execution
type ParallelSubAgentManager struct {
    // Core components
    parentCore     *ReactCore
    config         *ParallelConfig
    
    // Worker pool management
    workerSemaphore chan struct{}     // Semaphore for concurrency control
    workerPool      sync.Pool         // Pool of reusable subagent instances
    
    // Task and result management
    taskQueue       chan *ParallelTask
    resultCollector *ResultCollector
    
    // Synchronization and metrics
    mutex          sync.RWMutex
    metrics        *ParallelMetrics
    
    // Lifecycle management
    ctx            context.Context
    cancel         context.CancelFunc
    done           chan struct{}
}

// ParallelConfig - Configuration for parallel execution
type ParallelConfig struct {
    MaxWorkers      int           `json:"max_workers"`      // Default: 10
    TaskTimeout     time.Duration `json:"task_timeout"`     // Default: 5min
    ResultTimeout   time.Duration `json:"result_timeout"`   // Default: 10min
    RetryAttempts   int           `json:"retry_attempts"`   // Default: 3
    EnableMetrics   bool          `json:"enable_metrics"`   // Default: true
    BufferSize      int           `json:"buffer_size"`      // Default: 100
}
```

### 2. Parallel Task Definition

```go
// ParallelTask - Represents a task for parallel execution
type ParallelTask struct {
    ID            string                 `json:"id"`
    Task          string                 `json:"task"`
    Priority      int                    `json:"priority"`       // Higher = more priority
    MaxIterations int                    `json:"max_iterations"`
    AllowedTools  []string              `json:"allowed_tools"`
    Metadata      map[string]interface{} `json:"metadata"`
    
    // Execution context
    ExecutionCtx  context.Context `json:"-"`
    StreamCallback StreamCallback `json:"-"`
    
    // Result tracking
    StartTime     time.Time `json:"start_time"`
    ResultChan    chan *ParallelTaskResult `json:"-"`
}

// ParallelTaskResult - Result of parallel task execution
type ParallelTaskResult struct {
    TaskID        string                 `json:"task_id"`
    Success       bool                   `json:"success"`
    Result        *SubAgentResult        `json:"result"`
    Error         error                  `json:"error,omitempty"`
    Duration      time.Duration          `json:"duration"`
    WorkerID      string                 `json:"worker_id"`
    Attempts      int                    `json:"attempts"`
    Metadata      map[string]interface{} `json:"metadata"`
}
```

### 3. Result Collection System

```go
// ResultCollector - Manages sequential result collection and ordering
type ResultCollector struct {
    // Ordered result storage
    results       map[string]*ParallelTaskResult
    resultOrder   []string  // Maintains original task order
    completedIDs  map[string]bool
    
    // Synchronization
    mutex         sync.RWMutex
    resultChan    chan *ParallelTaskResult
    doneChan      chan struct{}
    
    // Configuration
    timeout       time.Duration
    maxResults    int
    
    // State tracking
    totalTasks    int
    completedTasks int
}

// OrderedResults - Sequential result collection
type OrderedResults struct {
    Results       []*ParallelTaskResult `json:"results"`
    TotalTasks    int                   `json:"total_tasks"`
    Completed     int                   `json:"completed"`
    Failed        int                   `json:"failed"`
    Duration      time.Duration         `json:"duration"`
    Metadata      map[string]interface{} `json:"metadata"`
}
```

### 4. Worker Management System

```go
// ParallelWorker - Worker instance for executing subagent tasks
type ParallelWorker struct {
    ID            string
    subAgent      *SubAgent
    parentManager *ParallelSubAgentManager
    
    // State management
    state         WorkerState
    currentTask   *ParallelTask
    mutex         sync.RWMutex
    
    // Performance tracking
    tasksCompleted int
    totalDuration  time.Duration
    lastActivity   time.Time
}

// WorkerState - Worker lifecycle states
type WorkerState int

const (
    WorkerIdle WorkerState = iota
    WorkerBusy
    WorkerError
    WorkerShutdown
)

// WorkerPool - Manages worker lifecycle and allocation
type WorkerPool struct {
    workers       map[string]*ParallelWorker
    idleWorkers   chan *ParallelWorker
    busyWorkers   map[string]*ParallelWorker
    mutex         sync.RWMutex
    
    maxWorkers    int
    currentSize   int
    config        *ParallelConfig
}
```

### 5. Synchronization and Concurrency Control

```go
// SynchronizationManager - Coordinates locks and semaphores
type SynchronizationManager struct {
    // Semaphore for worker pool
    workerSemaphore *Semaphore
    
    // Mutex for shared resources
    resourceMutex   sync.RWMutex
    metricsMutex    sync.RWMutex
    resultsMutex    sync.RWMutex
    
    // Channel coordination
    shutdownChan    chan struct{}
    errorChan       chan error
    
    // Context management
    ctx            context.Context
    cancel         context.CancelFunc
}

// Semaphore - Counting semaphore implementation
type Semaphore struct {
    permits chan struct{}
    size    int
}

func NewSemaphore(size int) *Semaphore {
    return &Semaphore{
        permits: make(chan struct{}, size),
        size:    size,
    }
}

func (s *Semaphore) Acquire(ctx context.Context) error {
    select {
    case s.permits <- struct{}{}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *Semaphore) Release() {
    <-s.permits
}
```

## Implementation Details

### 1. Parallel Execution Flow

```go
// ExecuteTasksParallel - Main entry point for parallel execution
func (pm *ParallelSubAgentManager) ExecuteTasksParallel(
    ctx context.Context, 
    tasks []*ParallelTask,
    streamCallback StreamCallback,
) (*OrderedResults, error) {
    
    // 1. Initialize execution context
    execCtx, cancel := context.WithTimeout(ctx, pm.config.ResultTimeout)
    defer cancel()
    
    // 2. Setup result collector
    collector := NewResultCollector(len(tasks), pm.config.ResultTimeout)
    
    // 3. Start worker pool
    if err := pm.startWorkerPool(execCtx); err != nil {
        return nil, fmt.Errorf("failed to start worker pool: %w", err)
    }
    
    // 4. Queue tasks with priority sorting
    sort.Slice(tasks, func(i, j int) bool {
        return tasks[i].Priority > tasks[j].Priority
    })
    
    // 5. Submit tasks to worker pool
    var wg sync.WaitGroup
    for _, task := range tasks {
        wg.Add(1)
        go pm.executeTaskAsync(execCtx, task, collector, &wg, streamCallback)
    }
    
    // 6. Wait for completion or timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return collector.GetOrderedResults(), nil
    case <-execCtx.Done():
        return collector.GetOrderedResults(), execCtx.Err()
    }
}
```

### 2. Worker Execution Logic

```go
// executeTaskAsync - Execute single task with retry logic
func (pm *ParallelSubAgentManager) executeTaskAsync(
    ctx context.Context,
    task *ParallelTask,
    collector *ResultCollector,
    wg *sync.WaitGroup,
    streamCallback StreamCallback,
) {
    defer wg.Done()
    
    // Acquire worker semaphore
    if err := pm.workerSemaphore.Acquire(ctx); err != nil {
        collector.AddResult(&ParallelTaskResult{
            TaskID:   task.ID,
            Success:  false,
            Error:    err,
            Duration: 0,
        })
        return
    }
    defer pm.workerSemaphore.Release()
    
    // Get or create worker
    worker, err := pm.getWorker()
    if err != nil {
        collector.AddResult(&ParallelTaskResult{
            TaskID:   task.ID,
            Success:  false,
            Error:    err,
            Duration: 0,
        })
        return
    }
    defer pm.releaseWorker(worker)
    
    // Execute with retry logic
    result := pm.executeWithRetry(ctx, worker, task, streamCallback)
    
    // Collect result
    collector.AddResult(result)
}

// executeWithRetry - Retry logic for failed tasks
func (pm *ParallelSubAgentManager) executeWithRetry(
    ctx context.Context,
    worker *ParallelWorker,
    task *ParallelTask,
    streamCallback StreamCallback,
) *ParallelTaskResult {
    
    var lastErr error
    startTime := time.Now()
    
    for attempt := 1; attempt <= pm.config.RetryAttempts; attempt++ {
        // Create task-specific context with timeout
        taskCtx, cancel := context.WithTimeout(ctx, pm.config.TaskTimeout)
        
        // Execute task
        result, err := worker.executeTask(taskCtx, task, streamCallback)
        cancel()
        
        if err == nil && result.Success {
            // Success
            return &ParallelTaskResult{
                TaskID:   task.ID,
                Success:  true,
                Result:   result,
                Duration: time.Since(startTime),
                WorkerID: worker.ID,
                Attempts: attempt,
            }
        }
        
        lastErr = err
        
        // Exponential backoff for retries
        if attempt < pm.config.RetryAttempts {
            backoff := time.Duration(attempt) * time.Second
            time.Sleep(backoff)
        }
    }
    
    // All retries failed
    return &ParallelTaskResult{
        TaskID:   task.ID,
        Success:  false,
        Error:    lastErr,
        Duration: time.Since(startTime),
        WorkerID: worker.ID,
        Attempts: pm.config.RetryAttempts,
    }
}
```

### 3. Result Collection and Ordering

```go
// ResultCollector implementation
func NewResultCollector(totalTasks int, timeout time.Duration) *ResultCollector {
    return &ResultCollector{
        results:       make(map[string]*ParallelTaskResult),
        resultOrder:   make([]string, 0, totalTasks),
        completedIDs:  make(map[string]bool),
        resultChan:    make(chan *ParallelTaskResult, totalTasks),
        doneChan:      make(chan struct{}),
        timeout:       timeout,
        maxResults:    totalTasks,
        totalTasks:    totalTasks,
    }
}

func (rc *ResultCollector) AddResult(result *ParallelTaskResult) {
    rc.mutex.Lock()
    defer rc.mutex.Unlock()
    
    // Store result
    rc.results[result.TaskID] = result
    rc.completedIDs[result.TaskID] = true
    rc.completedTasks++
    
    // Maintain order
    rc.resultOrder = append(rc.resultOrder, result.TaskID)
    
    // Check if all tasks completed
    if rc.completedTasks >= rc.totalTasks {
        close(rc.doneChan)
    }
}

func (rc *ResultCollector) GetOrderedResults() *OrderedResults {
    rc.mutex.RLock()
    defer rc.mutex.RUnlock()
    
    orderedResults := make([]*ParallelTaskResult, 0, len(rc.resultOrder))
    failed := 0
    
    for _, taskID := range rc.resultOrder {
        if result, exists := rc.results[taskID]; exists {
            orderedResults = append(orderedResults, result)
            if !result.Success {
                failed++
            }
        }
    }
    
    return &OrderedResults{
        Results:    orderedResults,
        TotalTasks: rc.totalTasks,
        Completed:  rc.completedTasks,
        Failed:     failed,
        Metadata:   map[string]interface{}{
            "collection_time": time.Now(),
            "ordered":         true,
        },
    }
}
```

## Performance and Monitoring

### 1. Metrics Collection

```go
// ParallelMetrics - Performance metrics for parallel execution
type ParallelMetrics struct {
    // Execution metrics
    TasksSubmitted    int64         `json:"tasks_submitted"`
    TasksCompleted    int64         `json:"tasks_completed"`
    TasksFailed       int64         `json:"tasks_failed"`
    
    // Timing metrics
    AverageExecTime   time.Duration `json:"average_exec_time"`
    TotalExecTime     time.Duration `json:"total_exec_time"`
    
    // Concurrency metrics
    ActiveWorkers     int           `json:"active_workers"`
    MaxConcurrency    int           `json:"max_concurrency"`
    
    // Resource metrics
    TotalTokensUsed   int64         `json:"total_tokens_used"`
    AverageTokensPerTask int64      `json:"average_tokens_per_task"`
    
    // Error metrics
    RetryAttempts     int64         `json:"retry_attempts"`
    TimeoutErrors     int64         `json:"timeout_errors"`
    
    mutex             sync.RWMutex
}
```

### 2. Health Monitoring

```go
// HealthChecker - Monitors system health and performance
type HealthChecker struct {
    manager         *ParallelSubAgentManager
    checkInterval   time.Duration
    thresholds      *HealthThresholds
    
    // Health state
    isHealthy       bool
    lastCheck       time.Time
    issues          []HealthIssue
    mutex           sync.RWMutex
}

// HealthThresholds - Configurable health thresholds
type HealthThresholds struct {
    MaxFailureRate    float64       `json:"max_failure_rate"`    // Default: 0.1 (10%)
    MaxAvgExecTime    time.Duration `json:"max_avg_exec_time"`   // Default: 5min
    MaxWorkerUtilization float64    `json:"max_worker_util"`     // Default: 0.9 (90%)
    MinSuccessRate    float64       `json:"min_success_rate"`    // Default: 0.8 (80%)
}
```

## Integration with Existing Architecture

### 1. SubAgent Extensions

```go
// Extend existing SubAgent to support parallel execution
func (sa *SubAgent) ExecuteTaskParallel(
    ctx context.Context, 
    task string, 
    streamCallback StreamCallback,
) (*SubAgentResult, error) {
    // Implementation maintains existing interface
    // but uses parallel execution internally when beneficial
    return sa.ExecuteTask(ctx, task, streamCallback)
}
```

### 2. Tool Integration

```go
// ParallelSubAgentTool - Tool for parallel subagent execution
type ParallelSubAgentTool struct {
    manager *ParallelSubAgentManager
}

func (psat *ParallelSubAgentTool) Execute(
    ctx context.Context, 
    args map[string]interface{},
) (interface{}, error) {
    
    // Parse parallel execution arguments
    tasks, err := psat.parseParallelTasks(args)
    if err != nil {
        return nil, err
    }
    
    // Execute in parallel
    return psat.manager.ExecuteTasksParallel(ctx, tasks, nil)
}
```

## Error Handling and Recovery

### 1. Error Types

```go
// ParallelExecutionError - Specialized error types
type ParallelExecutionError struct {
    Type      ErrorType             `json:"type"`
    TaskID    string                `json:"task_id"`
    WorkerID  string                `json:"worker_id"`
    Message   string                `json:"message"`
    Cause     error                 `json:"cause"`
    Metadata  map[string]interface{} `json:"metadata"`
}

type ErrorType int

const (
    ErrorWorkerTimeout ErrorType = iota
    ErrorWorkerPanic
    ErrorResourceExhausted
    ErrorInvalidTask
    ErrorSystemOverload
)
```

### 2. Recovery Strategies

```go
// RecoveryManager - Handles error recovery and system resilience
type RecoveryManager struct {
    strategies map[ErrorType]RecoveryStrategy
    mutex      sync.RWMutex
}

type RecoveryStrategy func(ctx context.Context, err *ParallelExecutionError) error

// Circuit breaker pattern for overload protection
type CircuitBreaker struct {
    state         CircuitState
    failureCount  int
    failureThreshold int
    resetTimeout  time.Duration
    lastFailure   time.Time
    mutex         sync.RWMutex
}
```

## Summary

This technical design provides a comprehensive parallel subagent execution system that:

1. **Maintains Compatibility**: Extends existing SubAgent interface
2. **Provides Scalability**: Semaphore-controlled worker pools
3. **Ensures Reliability**: Structured error handling and recovery
4. **Guarantees Ordering**: Sequential result collection
5. **Enables Monitoring**: Comprehensive metrics and health checking
6. **Supports Ultra Think**: Advanced reasoning capabilities through parallel execution

The implementation follows Go best practices for concurrency while integrating seamlessly with the existing Alex-Code architecture.