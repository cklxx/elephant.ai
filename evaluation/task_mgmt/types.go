package task_mgmt

import (
	"time"
)

// TaskStatus represents the lifecycle state of an eval task definition.
type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"
	TaskStatusArchived TaskStatus = "archived"
	TaskStatusDraft    TaskStatus = "draft"
)

// RunStatus represents the status of a batch run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

// EvalTaskDefinition describes a reusable evaluation task template.
type EvalTaskDefinition struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Status      TaskStatus        `json:"status"`
	DatasetPath string            `json:"dataset_path,omitempty"`
	DatasetType string            `json:"dataset_type,omitempty"`
	Config      TaskConfig        `json:"config"`
	Tags        []string          `json:"tags,omitempty"`
	Schedule    *Schedule         `json:"schedule,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TaskConfig holds execution parameters for a task.
type TaskConfig struct {
	InstanceLimit  int           `json:"instance_limit,omitempty"`
	MaxWorkers     int           `json:"max_workers,omitempty"`
	TimeoutPerTask time.Duration `json:"timeout_per_task,omitempty"`
	AgentID        string        `json:"agent_id,omitempty"`
	EnableMetrics  bool          `json:"enable_metrics"`
	ExtractRL      bool          `json:"extract_rl"`
}

// Schedule defines optional periodic execution.
type Schedule struct {
	CronExpr string `json:"cron_expr,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// BatchRun records one execution of a task definition.
type BatchRun struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	EvalJobID   string    `json:"eval_job_id,omitempty"`
	Status      RunStatus `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
	ResultCount int       `json:"result_count,omitempty"`
}

// CreateTaskRequest is the API request body for creating a new eval task.
type CreateTaskRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	DatasetPath string            `json:"dataset_path,omitempty"`
	DatasetType string            `json:"dataset_type,omitempty"`
	Config      TaskConfig        `json:"config"`
	Tags        []string          `json:"tags,omitempty"`
	Schedule    *Schedule         `json:"schedule,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateTaskRequest is the API request body for updating an eval task.
type UpdateTaskRequest struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Status      *TaskStatus       `json:"status,omitempty"`
	DatasetPath *string           `json:"dataset_path,omitempty"`
	Config      *TaskConfig       `json:"config,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Schedule    *Schedule         `json:"schedule,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
