// Package task defines the unified task domain model and store port.
//
// It replaces the disjoint server InMemoryTaskStore and Lark TaskPostgresStore
// with a single source of truth that persists task state durably across all
// channels (web, CLI, Lark) and supports subprocess resilience.
package task

import (
	"context"
	"encoding/json"
	"time"
)

// Status represents the lifecycle state of a task.
type Status string

const (
	StatusPending      Status = "pending"
	StatusRunning      Status = "running"
	StatusWaitingInput Status = "waiting_input"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
	StatusCancelled    Status = "cancelled"
)

// IsTerminal reports whether the status is a final state.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// TerminationReason explains why a task reached a terminal state.
type TerminationReason string

const (
	TerminationNone      TerminationReason = ""
	TerminationCompleted TerminationReason = "completed"
	TerminationCancelled TerminationReason = "cancelled"
	TerminationTimeout   TerminationReason = "timeout"
	TerminationError     TerminationReason = "error"
)

// BridgeMeta stores checkpoint data for subprocess bridge resilience.
type BridgeMeta struct {
	PID           int        `json:"pid,omitempty"`
	OutputFile    string     `json:"output_file,omitempty"`
	LastOffset    int64      `json:"last_offset,omitempty"`
	LastIteration int        `json:"last_iteration,omitempty"`
	TokensUsed    int        `json:"tokens_used,omitempty"`
	FilesTouched  []string   `json:"files_touched,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
}

// Task is the unified task record shared across all channels.
type Task struct {
	// Identity
	TaskID       string `json:"task_id"`
	SessionID    string `json:"session_id"`
	ParentTaskID string `json:"parent_task_id,omitempty"`

	// Channel binding
	Channel string `json:"channel"` // "lark", "web", "cli"
	ChatID  string `json:"chat_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`

	// Task content
	Description string `json:"description"`
	Prompt      string `json:"prompt,omitempty"` // Full original prompt for retry

	// Agent configuration
	AgentType     string `json:"agent_type"` // "internal", "claude_code", "codex"
	AgentPreset   string `json:"agent_preset,omitempty"`
	ToolPreset    string `json:"tool_preset,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"` // "execute" | "plan"
	AutonomyLevel string `json:"autonomy_level,omitempty"` // "controlled" | "semi" | "full"
	WorkspaceMode string `json:"workspace_mode,omitempty"` // "shared", "branch", "worktree"
	WorkingDir    string `json:"working_dir,omitempty"`

	// Config overrides (agent-specific)
	Config json.RawMessage `json:"config,omitempty"`

	// Lifecycle
	Status            Status            `json:"status"`
	TerminationReason TerminationReason `json:"termination_reason,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Progress tracking
	CurrentIteration int     `json:"current_iteration"`
	TotalIterations  int     `json:"total_iterations"`
	TokensUsed       int     `json:"tokens_used"`
	CostUSD          float64 `json:"cost_usd,omitempty"`

	// Results
	AnswerPreview    string          `json:"answer_preview,omitempty"`
	ResultJSON       json.RawMessage `json:"result_json,omitempty"`
	PlanJSON         json.RawMessage `json:"plan_json,omitempty"`
	RetryAttempt     int             `json:"retry_attempt,omitempty"`
	ParentPlanTaskID string          `json:"parent_plan_task_id,omitempty"`
	Error            string          `json:"error,omitempty"`

	// Dependencies
	DependsOn []string `json:"depends_on,omitempty"`

	// Bridge resilience
	BridgeMeta *BridgeMeta `json:"bridge_meta,omitempty"`

	// Extensible metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Transition records a state change in the task lifecycle.
type Transition struct {
	ID           int64           `json:"id"`
	TaskID       string          `json:"task_id"`
	FromStatus   Status          `json:"from_status"`
	ToStatus     Status          `json:"to_status"`
	Reason       string          `json:"reason,omitempty"`
	MetadataJSON json.RawMessage `json:"metadata_json,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// TransitionParams holds optional fields for a SetStatus call.
// Populated by TransitionOption functions.
type TransitionParams struct {
	Reason        string
	Metadata      map[string]any
	AnswerPreview *string
	ErrorText     *string
	TokensUsed    *int
}

// TransitionOption customises a SetStatus call.
type TransitionOption func(*TransitionParams)

// WithTransitionReason records why the status changed.
func WithTransitionReason(reason string) TransitionOption {
	return func(p *TransitionParams) { p.Reason = reason }
}

// WithTransitionMeta attaches arbitrary metadata to the transition record.
func WithTransitionMeta(meta map[string]any) TransitionOption {
	return func(p *TransitionParams) { p.Metadata = meta }
}

// WithTransitionAnswerPreview updates the answer preview alongside status.
func WithTransitionAnswerPreview(preview string) TransitionOption {
	return func(p *TransitionParams) { p.AnswerPreview = &preview }
}

// WithTransitionError updates the error text alongside status.
func WithTransitionError(errText string) TransitionOption {
	return func(p *TransitionParams) { p.ErrorText = &errText }
}

// WithTransitionTokens updates token count alongside status.
func WithTransitionTokens(tokens int) TransitionOption {
	return func(p *TransitionParams) { p.TokensUsed = &tokens }
}

// ApplyTransitionOptions collects all options into a TransitionParams.
func ApplyTransitionOptions(opts []TransitionOption) TransitionParams {
	var p TransitionParams
	for _, fn := range opts {
		fn(&p)
	}
	return p
}

// Store is the unified task persistence port.
type Store interface {
	// EnsureSchema creates or migrates the schema.
	EnsureSchema(ctx context.Context) error

	// Create persists a new task.
	Create(ctx context.Context, task *Task) error

	// Get retrieves a task by ID.
	Get(ctx context.Context, taskID string) (*Task, error)

	// SetStatus updates the task status and writes a transition record atomically.
	SetStatus(ctx context.Context, taskID string, status Status, opts ...TransitionOption) error

	// UpdateProgress updates iteration and token counts.
	UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int, costUSD float64) error

	// SetResult stores the completion result.
	SetResult(ctx context.Context, taskID string, answer string, resultJSON json.RawMessage, tokensUsed int) error

	// SetError records a task failure.
	SetError(ctx context.Context, taskID string, errText string) error

	// SetBridgeMeta persists bridge checkpoint data.
	SetBridgeMeta(ctx context.Context, taskID string, meta BridgeMeta) error

	// TryClaimTask tries to claim task ownership for execution. Returns true when
	// the claim succeeds, false when the task is owned by another live worker.
	TryClaimTask(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error)

	// ClaimResumableTasks atomically claims tasks in the given statuses for
	// resume execution and returns the claimed rows.
	ClaimResumableTasks(ctx context.Context, ownerID string, leaseUntil time.Time, limit int, statuses ...Status) ([]*Task, error)

	// RenewTaskLease extends the lease for a task owned by ownerID.
	RenewTaskLease(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error)

	// ReleaseTaskLease releases ownership for a task when execution exits.
	ReleaseTaskLease(ctx context.Context, taskID, ownerID string) error

	// ListBySession returns tasks for a session, newest first.
	ListBySession(ctx context.Context, sessionID string, limit int) ([]*Task, error)

	// ListByChat returns tasks for a chat, optionally filtered to active-only.
	ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]*Task, error)

	// ListByStatus returns tasks matching any of the given statuses.
	ListByStatus(ctx context.Context, statuses ...Status) ([]*Task, error)

	// ListActive returns all non-terminal tasks.
	ListActive(ctx context.Context) ([]*Task, error)

	// List returns paginated tasks, newest first.
	List(ctx context.Context, limit int, offset int) ([]*Task, int, error)

	// Delete removes a task.
	Delete(ctx context.Context, taskID string) error

	// Transitions returns the audit trail for a task.
	Transitions(ctx context.Context, taskID string) ([]Transition, error)

	// MarkStaleRunning marks all running/pending tasks as failed with the given reason.
	MarkStaleRunning(ctx context.Context, reason string) error

	// DeleteExpired removes tasks completed before the given time.
	DeleteExpired(ctx context.Context, before time.Time) error
}
