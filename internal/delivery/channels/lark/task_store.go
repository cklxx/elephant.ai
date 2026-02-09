package lark

import (
	"context"
	"time"
)

// TaskRecord represents a dispatched task tracked by the Lark gateway.
type TaskRecord struct {
	ChatID        string
	TaskID        string
	UserID        string
	AgentType     string // "claude_code" | "codex" | "internal"
	Description   string
	Status        string // "pending" | "running" | "waiting_input" | "completed" | "failed" | "cancelled"
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   time.Time
	AnswerPreview string
	Error         string
	TokensUsed    int
}

// TaskStore persists task records for the Lark gateway.
type TaskStore interface {
	EnsureSchema(ctx context.Context) error
	SaveTask(ctx context.Context, task TaskRecord) error
	UpdateStatus(ctx context.Context, taskID, status string, opts ...TaskUpdateOption) error
	GetTask(ctx context.Context, taskID string) (TaskRecord, bool, error)
	ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]TaskRecord, error)
	DeleteExpired(ctx context.Context, before time.Time) error
	MarkStaleRunning(ctx context.Context, reason string) error
}

// TaskUpdateOption is a functional option for UpdateStatus.
type TaskUpdateOption func(*taskUpdateOptions)

type taskUpdateOptions struct {
	answerPreview *string
	errorText     *string
	tokensUsed    *int
}

// WithAnswerPreview sets the answer preview on status update.
func WithAnswerPreview(preview string) TaskUpdateOption {
	return func(o *taskUpdateOptions) { o.answerPreview = &preview }
}

// WithErrorText sets the error text on status update.
func WithErrorText(text string) TaskUpdateOption {
	return func(o *taskUpdateOptions) { o.errorText = &text }
}

// WithTokensUsed sets the tokens used on status update.
func WithTokensUsed(tokens int) TaskUpdateOption {
	return func(o *taskUpdateOptions) { o.tokensUsed = &tokens }
}
