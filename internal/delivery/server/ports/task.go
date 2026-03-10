package ports

import (
	"context"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// TaskStatus represents the state of a task
type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "pending"
	TaskStatusRunning      TaskStatus = "running"
	TaskStatusWaitingInput TaskStatus = "waiting_input"
	TaskStatusCompleted    TaskStatus = "completed"
	TaskStatusFailed       TaskStatus = "failed"
	TaskStatusCancelled    TaskStatus = "cancelled"
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
	ID                string            `json:"task_id"`
	SessionID         string            `json:"session_id"`
	ParentTaskID      string            `json:"parent_task_id,omitempty"`
	Status            TaskStatus        `json:"status"`
	Description       string            `json:"task"`
	CreatedAt         time.Time         `json:"created_at"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty"`
	Error             string            `json:"error,omitempty"`
	Result            *agent.TaskResult `json:"result,omitempty"`
	TerminationReason TerminationReason `json:"termination_reason,omitempty"`

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

// TaskReader provides read-only access to tasks.
type TaskReader interface {
	Get(ctx context.Context, taskID string) (*Task, error)
	List(ctx context.Context, limit int, offset int) ([]*Task, int, error)
	ListBySession(ctx context.Context, sessionID string) ([]*Task, error)
	ListByStatus(ctx context.Context, statuses ...TaskStatus) ([]*Task, error)
}

// TaskWriter provides task mutation operations.
type TaskWriter interface {
	Create(ctx context.Context, sessionID string, description string, agentPreset string, toolPreset string) (*Task, error)
	Delete(ctx context.Context, taskID string) error
	SetStatus(ctx context.Context, taskID string, status TaskStatus) error
	SetError(ctx context.Context, taskID string, err error) error
	SetResult(ctx context.Context, taskID string, result *agent.TaskResult) error
	UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int) error
	SetTerminationReason(ctx context.Context, taskID string, reason TerminationReason) error
}

// TaskClaimer provides distributed task ownership operations.
type TaskClaimer interface {
	TryClaimTask(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error)
	ClaimResumableTasks(ctx context.Context, ownerID string, leaseUntil time.Time, limit int, statuses ...TaskStatus) ([]*Task, error)
	RenewTaskLease(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error)
	ReleaseTaskLease(ctx context.Context, taskID, ownerID string) error
}

// TaskStore manages task lifecycle and persistence.
// It composes TaskReader, TaskWriter, and TaskClaimer for callers that need full access.
// Prefer depending on the narrower interface that matches your actual usage.
type TaskStore interface {
	TaskReader
	TaskWriter
	TaskClaimer
}

// IsTerminal reports whether the status is a final state.
func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled:
		return true
	default:
		return false
	}
}
