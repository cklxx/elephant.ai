package swe_bench

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/async"
)

// WorkerPoolImpl implements the WorkerPool interface
type WorkerPoolImpl struct {
	numWorkers   int
	taskQueue    chan WorkerTask
	resultQueue  chan WorkerResult
	workers      []*Worker
	agentFactory AgentFactory

	// State management
	mu             sync.RWMutex
	isStarted      bool
	isStopped      bool
	activeWorkers  int32
	queuedTasks    int32
	completedTasks int32
	failedTasks    int32

	// Coordination
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// Worker represents a single worker in the pool
type Worker struct {
	id          int
	pool        *WorkerPoolImpl
	agent       Agent
	isActive    bool
	currentTask *WorkerTask
	mu          sync.RWMutex
}

func clampWorkerCount(numWorkers int) int {
	const maxWorkers = 20

	if numWorkers <= 0 {
		return 1
	}
	if numWorkers > maxWorkers {
		return maxWorkers
	}

	return numWorkers
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(numWorkers int) *WorkerPoolImpl {
	numWorkers = clampWorkerCount(numWorkers)

	queueSize := numWorkers * 2

	return &WorkerPoolImpl{
		numWorkers:   numWorkers,
		taskQueue:    make(chan WorkerTask, queueSize), // Buffer for better throughput
		resultQueue:  make(chan WorkerResult, queueSize),
		workers:      make([]*Worker, numWorkers),
		agentFactory: NewAlexAgentFactory(),
	}
}

// Start starts the worker pool
func (wp *WorkerPoolImpl) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.isStarted {
		return fmt.Errorf("worker pool already started")
	}

	// Create context for worker coordination
	wp.ctx, wp.cancel = context.WithCancel(ctx)

	// Initialize workers
	for i := 0; i < wp.numWorkers; i++ {
		worker := &Worker{
			id:   i,
			pool: wp,
		}
		wp.workers[i] = worker

		// Start worker goroutine
		wp.wg.Add(1)
		async.Go(panicLogger{}, "swe-bench.worker", func() {
			wp.workerLoop(worker)
		})
	}

	wp.isStarted = true
	log.Printf("Worker pool started with %d workers", wp.numWorkers)

	return nil
}

// Stop stops the worker pool gracefully
func (wp *WorkerPoolImpl) Stop(ctx context.Context) error {
	wp.mu.Lock()
	if !wp.isStarted || wp.isStopped {
		wp.mu.Unlock()
		return nil
	}
	wp.isStopped = true
	wp.mu.Unlock()

	log.Printf("Stopping worker pool...")

	// Close task queue to signal workers to stop accepting new tasks
	close(wp.taskQueue)

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	async.Go(panicLogger{}, "swe-bench.worker-wait", func() {
		defer close(done)
		wp.wg.Wait()
	})

	select {
	case <-done:
		log.Printf("Worker pool stopped gracefully")
	case <-ctx.Done():
		log.Printf("Worker pool stop timeout, forcing shutdown")
		wp.cancel() // Force cancel all workers
		wp.wg.Wait()
	case <-time.After(30 * time.Second):
		log.Printf("Worker pool stop timeout, forcing shutdown")
		wp.cancel() // Force cancel all workers
		wp.wg.Wait()
	}

	// Close result queue
	close(wp.resultQueue)

	// Clean up agents
	for _, worker := range wp.workers {
		if worker.agent != nil {
			if err := worker.agent.Close(); err != nil {
				log.Printf("Warning: Failed to close worker agent: %v", err)
			}
		}
	}

	return nil
}

// SubmitTask submits a task for processing
func (wp *WorkerPoolImpl) SubmitTask(task WorkerTask) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.isStarted || wp.isStopped {
		return fmt.Errorf("worker pool not running")
	}

	select {
	case wp.taskQueue <- task:
		atomic.AddInt32(&wp.queuedTasks, 1)
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool context cancelled")
	default:
		return fmt.Errorf("task queue full")
	}
}

// GetResults returns a channel of results
func (wp *WorkerPoolImpl) GetResults() <-chan WorkerResult {
	return wp.resultQueue
}

// GetStatus returns the current status of the worker pool
func (wp *WorkerPoolImpl) GetStatus() PoolStatus {
	return PoolStatus{
		ActiveWorkers:  int(atomic.LoadInt32(&wp.activeWorkers)),
		QueuedTasks:    int(atomic.LoadInt32(&wp.queuedTasks)),
		CompletedTasks: int(atomic.LoadInt32(&wp.completedTasks)),
		FailedTasks:    int(atomic.LoadInt32(&wp.failedTasks)),
	}
}

// workerLoop is the main loop for each worker
func (wp *WorkerPoolImpl) workerLoop(worker *Worker) {
	defer wp.wg.Done()

	log.Printf("Worker %d started", worker.id)

	for {
		select {
		case <-wp.ctx.Done():
			log.Printf("Worker %d stopped (context cancelled)", worker.id)
			return

		case task, ok := <-wp.taskQueue:
			if !ok {
				log.Printf("Worker %d stopped (task queue closed)", worker.id)
				return
			}

			// Process task
			result := wp.processTask(worker, task)

			// Send result
			select {
			case wp.resultQueue <- result:
				// Result sent successfully
			case <-wp.ctx.Done():
				log.Printf("Worker %d stopped while sending result", worker.id)
				return
			}
		}
	}
}

// processTask processes a single task
func (wp *WorkerPoolImpl) processTask(worker *Worker, task WorkerTask) WorkerResult {
	atomic.AddInt32(&wp.queuedTasks, -1)
	atomic.AddInt32(&wp.activeWorkers, 1)
	defer atomic.AddInt32(&wp.activeWorkers, -1)

	worker.mu.Lock()
	worker.isActive = true
	worker.currentTask = &task
	worker.mu.Unlock()

	defer func() {
		worker.mu.Lock()
		worker.isActive = false
		worker.currentTask = nil
		worker.mu.Unlock()
	}()

	log.Printf("Worker %d processing task %s (instance %s)", worker.id, task.ID, task.Instance.ID)

	startTime := time.Now()

	// Create context with timeout
	timeout := time.Duration(task.Config.Agent.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(wp.ctx, timeout)
	defer cancel()

	// Ensure worker has an agent
	if worker.agent == nil {
		agent, err := wp.agentFactory.CreateAgent(ctx, task.Config)
		if err != nil {
			log.Printf("Worker %d failed to create agent: %v", worker.id, err)
			result := WorkerResult{
				TaskID:     task.ID,
				InstanceID: task.Instance.ID,
				Status:     StatusFailed,
				StartTime:  startTime,
				EndTime:    time.Now(),
				Duration:   time.Since(startTime),
				Error:      fmt.Sprintf("Failed to create agent: %v", err),
				ErrorType:  "agent_creation_error",
				RetryCount: task.RetryCount,
			}
			atomic.AddInt32(&wp.failedTasks, 1)
			return result
		}
		worker.agent = agent
	}

	// Process instance with timeout handling
	var result *WorkerResult
	var err error

	done := make(chan struct{})
	async.Go(panicLogger{}, "swe-bench.process-instance", func() {
		defer close(done)
		result, err = worker.agent.ProcessInstance(ctx, task.Instance)
	})

	select {
	case <-done:
		// Processing completed
		if err != nil {
			log.Printf("Worker %d failed to process instance %s: %v", worker.id, task.Instance.ID, err)
			result = &WorkerResult{
				TaskID:     task.ID,
				InstanceID: task.Instance.ID,
				Status:     StatusFailed,
				StartTime:  startTime,
				EndTime:    time.Now(),
				Duration:   time.Since(startTime),
				Error:      err.Error(),
				ErrorType:  "processing_error",
				RetryCount: task.RetryCount,
			}
			atomic.AddInt32(&wp.failedTasks, 1)
		} else {
			// Ensure result has required fields
			if result == nil {
				result = &WorkerResult{
					TaskID:     task.ID,
					InstanceID: task.Instance.ID,
					Status:     StatusFailed,
					StartTime:  startTime,
					EndTime:    time.Now(),
					Duration:   time.Since(startTime),
					Error:      "Agent returned nil result",
					ErrorType:  "nil_result_error",
					RetryCount: task.RetryCount,
				}
				atomic.AddInt32(&wp.failedTasks, 1)
			} else {
				result.TaskID = task.ID
				result.InstanceID = task.Instance.ID
				result.StartTime = startTime
				result.EndTime = time.Now()
				result.Duration = time.Since(startTime)
				result.RetryCount = task.RetryCount

				if result.Status == "" {
					result.Status = StatusCompleted
				}

				if result.Status == StatusCompleted {
					atomic.AddInt32(&wp.completedTasks, 1)
				} else {
					atomic.AddInt32(&wp.failedTasks, 1)
				}
			}
		}

	case <-ctx.Done():
		// Timeout or cancellation
		log.Printf("Worker %d timed out processing instance %s", worker.id, task.Instance.ID)
		result = &WorkerResult{
			TaskID:     task.ID,
			InstanceID: task.Instance.ID,
			Status:     StatusTimeout,
			StartTime:  startTime,
			EndTime:    time.Now(),
			Duration:   time.Since(startTime),
			Error:      "Task timed out",
			ErrorType:  "timeout_error",
			RetryCount: task.RetryCount,
		}
		atomic.AddInt32(&wp.failedTasks, 1)

		// Try to cleanup the agent as it might be in a bad state
		if worker.agent != nil {
			if err := worker.agent.Close(); err != nil {
				log.Printf("Warning: Failed to close failed worker agent: %v", err)
			}
			worker.agent = nil
		}
	}

	log.Printf("Worker %d completed task %s in %v (status: %s)",
		worker.id, task.ID, result.Duration, result.Status)

	return *result
}

// GetWorkerStatus returns the status of individual workers
func (wp *WorkerPoolImpl) GetWorkerStatus() []WorkerStatus {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	status := make([]WorkerStatus, len(wp.workers))
	for i, worker := range wp.workers {
		worker.mu.RLock()
		status[i] = WorkerStatus{
			ID:       worker.id,
			IsActive: worker.isActive,
		}
		if worker.currentTask != nil {
			status[i].CurrentTaskID = worker.currentTask.ID
			status[i].CurrentInstanceID = worker.currentTask.Instance.ID
		}
		worker.mu.RUnlock()
	}

	return status
}

// WorkerStatus represents the status of a single worker
type WorkerStatus struct {
	ID                int    `json:"id"`
	IsActive          bool   `json:"is_active"`
	CurrentTaskID     string `json:"current_task_id,omitempty"`
	CurrentInstanceID string `json:"current_instance_id,omitempty"`
}

// WaitForCompletion waits for all tasks to complete or timeout
func (wp *WorkerPoolImpl) WaitForCompletion(ctx context.Context, expectedTasks int) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status := wp.GetStatus()
			completed := status.CompletedTasks + status.FailedTasks

			if completed >= expectedTasks {
				log.Printf("All tasks completed: %d completed, %d failed",
					status.CompletedTasks, status.FailedTasks)
				return nil
			}

			log.Printf("Progress: %d/%d tasks completed (%d active, %d queued)",
				completed, expectedTasks, status.ActiveWorkers, status.QueuedTasks)
		}
	}
}

// SetMaxWorkers dynamically adjusts the number of workers (not recommended during active processing)
func (wp *WorkerPoolImpl) SetMaxWorkers(maxWorkers int) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.isStarted && !wp.isStopped {
		return fmt.Errorf("cannot change worker count while pool is running")
	}

	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	if maxWorkers > 20 {
		maxWorkers = 20
	}

	wp.numWorkers = maxWorkers
	wp.workers = make([]*Worker, maxWorkers)

	return nil
}

// GetMetrics returns performance metrics for the worker pool
func (wp *WorkerPoolImpl) GetMetrics() WorkerPoolMetrics {
	status := wp.GetStatus()

	return WorkerPoolMetrics{
		NumWorkers:     wp.numWorkers,
		ActiveWorkers:  status.ActiveWorkers,
		QueuedTasks:    status.QueuedTasks,
		CompletedTasks: status.CompletedTasks,
		FailedTasks:    status.FailedTasks,
		TotalTasks:     status.CompletedTasks + status.FailedTasks,
		IsRunning:      wp.isStarted && !wp.isStopped,
	}
}

// WorkerPoolMetrics represents performance metrics for the worker pool
type WorkerPoolMetrics struct {
	NumWorkers     int  `json:"num_workers"`
	ActiveWorkers  int  `json:"active_workers"`
	QueuedTasks    int  `json:"queued_tasks"`
	CompletedTasks int  `json:"completed_tasks"`
	FailedTasks    int  `json:"failed_tasks"`
	TotalTasks     int  `json:"total_tasks"`
	IsRunning      bool `json:"is_running"`
}
