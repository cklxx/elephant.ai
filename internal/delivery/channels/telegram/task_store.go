package telegram

import (
	"context"
	"time"
)

// TaskRecord captures a single task's lifecycle for persistence.
type TaskRecord struct {
	ChatID        int64
	TaskID        string
	UserID        int64
	Description   string
	Status        string // "pending" | "running" | "waiting_input" | "completed" | "failed" | "cancelled"
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   time.Time
	AnswerPreview string
	Error         string
	TokensUsed    int
}

// TaskStore persists Telegram task records.
type TaskStore interface {
	EnsureSchema(ctx context.Context) error
	SaveTask(ctx context.Context, task TaskRecord) error
	UpdateStatus(ctx context.Context, taskID, status string, opts ...TaskUpdateOption) error
	GetTask(ctx context.Context, taskID string) (TaskRecord, bool, error)
	ListByChat(ctx context.Context, chatID int64, activeOnly bool, limit int) ([]TaskRecord, error)
	DeleteExpired(ctx context.Context, before time.Time) error
	MarkStaleRunning(ctx context.Context, reason string) error
}

// TaskUpdateOption is a functional option for UpdateStatus.
type TaskUpdateOption func(*taskUpdateOptions)

type taskUpdateOptions struct {
	answerPreview string
	errorText     string
	tokensUsed    int
}

// WithAnswerPreview sets the answer preview on a status update.
func WithAnswerPreview(preview string) TaskUpdateOption {
	return func(o *taskUpdateOptions) { o.answerPreview = preview }
}

// WithErrorText sets the error text on a status update.
func WithErrorText(text string) TaskUpdateOption {
	return func(o *taskUpdateOptions) { o.errorText = text }
}

// WithTokensUsed sets token usage on a status update.
func WithTokensUsed(tokens int) TaskUpdateOption {
	return func(o *taskUpdateOptions) { o.tokensUsed = tokens }
}

func resolveTaskUpdateOptions(opts []TaskUpdateOption) taskUpdateOptions {
	var o taskUpdateOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}
