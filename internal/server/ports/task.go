package ports

import (
	"context"
	"time"

	agentPorts "alex/internal/agent/ports"
)

// TaskStatus represents the state of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TerminationReason represents why a task terminated
type TerminationReason string

const (
	TerminationReasonCompleted TerminationReason = "completed"
	TerminationReasonCancelled TerminationReason = "cancelled"
	TerminationReasonTimeout   TerminationReason = "timeout"
	TerminationReasonError     TerminationReason = "error"
	TerminationReasonNone      TerminationReason = ""
)

// Task represents a task execution
type Task struct {
	ID                string                 `json:"task_id"`
	SessionID         string                 `json:"session_id"`
	ParentTaskID      string                 `json:"parent_task_id,omitempty"`
	Status            TaskStatus             `json:"status"`
	Description       string                 `json:"task"`
	CreatedAt         time.Time              `json:"created_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	CompletedAt       *time.Time             `json:"completed_at,omitempty"`
	Error             string                 `json:"error,omitempty"`
	Result            *agentPorts.TaskResult `json:"result,omitempty"`
	TerminationReason TerminationReason      `json:"termination_reason,omitempty"`

	// Progress tracking
	CurrentIteration int `json:"current_iteration"` // Current iteration during execution (no omitempty - always show)
	TotalIterations  int `json:"total_iterations"`  // Total iterations after completion
	TokensUsed       int `json:"tokens_used"`       // Tokens used so far (no omitempty - always show)
	TotalTokens      int `json:"total_tokens"`      // Total tokens after completion

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// Preset configuration
	AgentPreset string `json:"agent_preset,omitempty"` // Agent persona preset used
	ToolPreset  string `json:"tool_preset,omitempty"`  // Tool access preset used
}

// TaskStore manages task lifecycle and persistence
type TaskStore interface {
	// Create creates a new task with optional presets
	Create(ctx context.Context, sessionID string, description string, agentPreset string, toolPreset string) (*Task, error)

	// Get retrieves a task by ID
	Get(ctx context.Context, taskID string) (*Task, error)

	// Update updates task state
	Update(ctx context.Context, task *Task) error

	// List returns tasks with pagination
	List(ctx context.Context, limit int, offset int) ([]*Task, int, error)

	// ListBySession returns tasks for a specific session
	ListBySession(ctx context.Context, sessionID string) ([]*Task, error)

	// Delete removes a task
	Delete(ctx context.Context, taskID string) error

	// SetStatus updates task status
	SetStatus(ctx context.Context, taskID string, status TaskStatus) error

	// SetError records task failure
	SetError(ctx context.Context, taskID string, err error) error

	// SetResult stores task completion result
	SetResult(ctx context.Context, taskID string, result *agentPorts.TaskResult) error

	// UpdateProgress updates task execution progress
	UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int) error

	// SetTerminationReason sets the termination reason for a task
	SetTerminationReason(ctx context.Context, taskID string, reason TerminationReason) error
}

// TaskListParams represents pagination and filtering parameters
type TaskListParams struct {
	Limit     int
	Offset    int
	SessionID string
	Status    TaskStatus
}
