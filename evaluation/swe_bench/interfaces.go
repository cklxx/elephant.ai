package swe_bench

import (
	"context"
	"io"
)

// DatasetLoader defines the interface for loading dataset instances
type DatasetLoader interface {
	// LoadInstances loads instances based on the dataset configuration
	LoadInstances(ctx context.Context, config DatasetConfig) ([]Instance, error)

	// GetInstanceCount returns the total number of instances in the dataset
	GetInstanceCount(ctx context.Context, config DatasetConfig) (int, error)

	// ValidateConfig validates the dataset configuration
	ValidateConfig(config DatasetConfig) error
}

// BatchProcessor defines the interface for batch processing
type BatchProcessor interface {
	// ProcessBatch processes a batch of instances
	ProcessBatch(ctx context.Context, instances []Instance, config *BatchConfig) (*BatchResult, error)

	// ProcessInstance processes a single instance
	ProcessInstance(ctx context.Context, instance Instance, config *BatchConfig) (*WorkerResult, error)

	// Resume resumes processing from a previous state
	Resume(ctx context.Context, resultPath string, config *BatchConfig) (*BatchResult, error)
}

// WorkerPool defines the interface for managing worker processes
type WorkerPool interface {
	// Start starts the worker pool
	Start(ctx context.Context) error

	// Stop stops the worker pool gracefully
	Stop(ctx context.Context) error

	// SubmitTask submits a task for processing
	SubmitTask(task WorkerTask) error

	// GetResults returns a channel of results
	GetResults() <-chan WorkerResult

	// GetStatus returns the current status of the worker pool
	GetStatus() PoolStatus
}

// PoolStatus represents the status of a worker pool
type PoolStatus struct {
	ActiveWorkers  int `json:"active_workers"`
	QueuedTasks    int `json:"queued_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks    int `json:"failed_tasks"`
}

// ResultWriter defines the interface for writing batch results
type ResultWriter interface {
	// WriteResults writes batch results to storage
	WriteResults(ctx context.Context, result *BatchResult, path string) error

	// WritePartialResults writes partial results during processing
	WritePartialResults(ctx context.Context, results []WorkerResult, path string) error

	// ReadResults reads previously saved results
	ReadResults(ctx context.Context, path string) (*BatchResult, error)

	// AppendResult appends a single result to the output
	AppendResult(ctx context.Context, result WorkerResult, path string) error
}

// ProgressReporter defines the interface for reporting progress
type ProgressReporter interface {
	// Start starts progress reporting
	Start(ctx context.Context) error

	// Stop stops progress reporting
	Stop() error

	// Update updates the progress
	Update(update ProgressUpdate) error

	// SetOutput sets the output writer for progress updates
	SetOutput(w io.Writer)
}

// AgentFactory defines the interface for creating agent instances
type AgentFactory interface {
	// CreateAgent creates a new agent instance
	CreateAgent(ctx context.Context, config *BatchConfig) (Agent, error)

	// ValidateConfig validates the agent configuration
	ValidateConfig(config *BatchConfig) error
}

// Agent defines the interface for AI agents processing SWE-Bench instances
type Agent interface {
	// ProcessInstance processes a single SWE-Bench instance
	ProcessInstance(ctx context.Context, instance Instance) (*WorkerResult, error)

	// GetConfiguration returns the agent configuration
	GetConfiguration() map[string]any

	// Close releases agent resources
	Close() error
}

// Monitor defines the interface for monitoring batch processing
type Monitor interface {
	// StartMonitoring starts monitoring the batch process
	StartMonitoring(ctx context.Context) error

	// StopMonitoring stops monitoring
	StopMonitoring() error

	// RecordMetric records a metric
	RecordMetric(name string, value float64, tags map[string]string) error

	// RecordEvent records an event
	RecordEvent(event string, data map[string]any) error

	// GetMetrics returns current metrics
	GetMetrics() map[string]float64
}

// Validator defines the interface for validating solutions
type Validator interface {
	// ValidateSolution validates a solution against test cases
	ValidateSolution(ctx context.Context, instance Instance, solution string) (*ValidationResult, error)

	// RunTests runs the test cases for an instance
	RunTests(ctx context.Context, instance Instance, solutionPath string) (*TestResult, error)
}

// ValidationResult represents the result of solution validation
type ValidationResult struct {
	IsValid     bool             `json:"is_valid"`
	Score       float64          `json:"score"`
	TestsPassed int              `json:"tests_passed"`
	TestsFailed int              `json:"tests_failed"`
	TestResults []TestCaseResult `json:"test_results"`
	Errors      []string         `json:"errors,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
}

// TestResult represents the result of running tests
type TestResult struct {
	Success     bool   `json:"success"`
	ExitCode    int    `json:"exit_code"`
	Output      string `json:"output"`
	ErrorOutput string `json:"error_output,omitempty"`
	TestsPassed int    `json:"tests_passed"`
	TestsFailed int    `json:"tests_failed"`
	Duration    string `json:"duration"`
}

// TestCaseResult represents the result of a single test case
type TestCaseResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // "passed", "failed", "skipped"
	Message  string `json:"message,omitempty"`
	Duration string `json:"duration"`
}
