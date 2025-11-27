package swe_bench

import (
	"time"

	"alex/internal/workflow"
)

// Instance represents a single SWE-Bench problem instance
type Instance struct {
	ID               string            `json:"instance_id"`
	RepoURL          string            `json:"repo"`
	BaseCommit       string            `json:"base_commit"`
	ProblemStatement string            `json:"problem_statement"`
	Hints            string            `json:"hints_text,omitempty"`
	CreatedAt        string            `json:"created_at"`
	PatchText        string            `json:"patch,omitempty"`
	TestPatch        string            `json:"test_patch,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
}

// DatasetConfig represents dataset configuration
type DatasetConfig struct {
	Type   string `json:"type" yaml:"type"`     // "swe_bench", "file", "huggingface"
	Subset string `json:"subset" yaml:"subset"` // "lite", "full", "verified"
	Split  string `json:"split" yaml:"split"`   // "dev", "test", "train"

	// For file-based datasets
	FilePath string `json:"file_path,omitempty" yaml:"file_path,omitempty"`

	// For Hugging Face datasets
	HFDataset string `json:"hf_dataset,omitempty" yaml:"hf_dataset,omitempty"`

	// Instance filtering
	InstanceLimit int      `json:"instance_limit,omitempty" yaml:"instance_limit,omitempty"`
	InstanceSlice []int    `json:"instance_slice,omitempty" yaml:"instance_slice,omitempty"`
	InstanceIDs   []string `json:"instance_ids,omitempty" yaml:"instance_ids,omitempty"`
	Shuffle       bool     `json:"shuffle,omitempty" yaml:"shuffle,omitempty"`
}

// BatchConfig represents batch processing configuration
type BatchConfig struct {
	// Agent configuration
	Agent struct {
		Model struct {
			Name        string  `json:"name" yaml:"name"`
			Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
			MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
		} `json:"model" yaml:"model"`

		MaxTurns  int     `json:"max_turns,omitempty" yaml:"max_turns,omitempty"`
		CostLimit float64 `json:"cost_limit,omitempty" yaml:"cost_limit,omitempty"`
		Timeout   int     `json:"timeout,omitempty" yaml:"timeout,omitempty"` // seconds
	} `json:"agent" yaml:"agent"`

	// Dataset configuration
	Instances DatasetConfig `json:"instances" yaml:"instances"`

	// Execution configuration
	NumWorkers    int    `json:"num_workers,omitempty" yaml:"num_workers,omitempty"`
	OutputPath    string `json:"output_path,omitempty" yaml:"output_path,omitempty"`
	ResumeFrom    string `json:"resume_from,omitempty" yaml:"resume_from,omitempty"`
	EnableLogging bool   `json:"enable_logging,omitempty" yaml:"enable_logging,omitempty"`

	// Execution control
	MaxDelay   time.Duration `json:"max_delay,omitempty" yaml:"max_delay,omitempty"`
	FailFast   bool          `json:"fail_fast,omitempty" yaml:"fail_fast,omitempty"`
	MaxRetries int           `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
}

// WorkerTask represents a task assigned to a worker
type WorkerTask struct {
	ID         string       `json:"id"`
	Instance   Instance     `json:"instance"`
	Config     *BatchConfig `json:"config"`
	RetryCount int          `json:"retry_count"`
	CreatedAt  time.Time    `json:"created_at"`
}

// WorkerResult represents the result of processing a task
type WorkerResult struct {
	TaskID     string       `json:"task_id"`
	InstanceID string       `json:"instance_id"`
	Status     ResultStatus `json:"status"`

	// Results
	Solution     string   `json:"solution,omitempty"`
	Explanation  string   `json:"explanation,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
	Commands     []string `json:"commands,omitempty"`

	// Execution metadata
	StartTime  time.Time                  `json:"start_time"`
	EndTime    time.Time                  `json:"end_time"`
	Duration   time.Duration              `json:"duration"`
	TokensUsed int                        `json:"tokens_used,omitempty"`
	Cost       float64                    `json:"cost,omitempty"`
	Workflow   *workflow.WorkflowSnapshot `json:"workflow,omitempty"`

	// Error information
	Error      string `json:"error,omitempty"`
	ErrorType  string `json:"error_type,omitempty"`
	RetryCount int    `json:"retry_count"`

	// Agent interaction trace
	Trace []TraceStep `json:"trace,omitempty"`
}

// ResultStatus represents the status of a task result
type ResultStatus string

const (
	StatusPending   ResultStatus = "pending"
	StatusRunning   ResultStatus = "running"
	StatusCompleted ResultStatus = "completed"
	StatusFailed    ResultStatus = "failed"
	StatusTimeout   ResultStatus = "timeout"
	StatusCanceled  ResultStatus = "canceled"
)

// TraceStep represents a single step in the agent execution trace
type TraceStep struct {
	Step        int       `json:"step"`
	Action      string    `json:"action"`
	Observation string    `json:"observation"`
	Thought     string    `json:"thought,omitempty"`
	ToolCall    *ToolCall `json:"tool_call,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ToolCall represents a tool call made by the agent
type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Result    any            `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  time.Duration  `json:"duration"`
}

// BatchResult represents the overall result of batch processing
type BatchResult struct {
	Config    *BatchConfig  `json:"config"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// Statistics
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	FailedTasks    int     `json:"failed_tasks"`
	SuccessRate    float64 `json:"success_rate"`

	// Resource usage
	TotalTokens int           `json:"total_tokens"`
	TotalCost   float64       `json:"total_cost"`
	AvgDuration time.Duration `json:"avg_duration"`

	// Results
	Results []WorkerResult `json:"results"`

	// Error summary
	ErrorSummary map[string]int `json:"error_summary,omitempty"`
}

// ProgressUpdate represents a progress update from the batch processor
type ProgressUpdate struct {
	Timestamp    time.Time     `json:"timestamp"`
	Total        int           `json:"total"`
	Completed    int           `json:"completed"`
	Failed       int           `json:"failed"`
	Running      int           `json:"running"`
	Remaining    int           `json:"remaining"`
	SuccessRate  float64       `json:"success_rate"`
	AvgDuration  time.Duration `json:"avg_duration"`
	EstimatedETA time.Duration `json:"estimated_eta"`
}

// DefaultBatchConfig returns a default batch configuration
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		Agent: struct {
			Model struct {
				Name        string  `json:"name" yaml:"name"`
				Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
				MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
			} `json:"model" yaml:"model"`
			MaxTurns  int     `json:"max_turns,omitempty" yaml:"max_turns,omitempty"`
			CostLimit float64 `json:"cost_limit,omitempty" yaml:"cost_limit,omitempty"`
			Timeout   int     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
		}{
			Model: struct {
				Name        string  `json:"name" yaml:"name"`
				Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
				MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
			}{
				Name:        "deepseek/deepseek-chat-v3-0324:free",
				Temperature: 0.1,
				MaxTokens:   4000,
			},
			MaxTurns:  20,
			CostLimit: 10.0,
			Timeout:   300,
		},
		Instances: DatasetConfig{
			Type:   "swe_bench",
			Subset: "lite",
			Split:  "dev",
		},
		NumWorkers:    3,
		OutputPath:    "./batch_results",
		EnableLogging: true,
		MaxDelay:      5 * time.Second,
		FailFast:      false,
		MaxRetries:    2,
	}
}
