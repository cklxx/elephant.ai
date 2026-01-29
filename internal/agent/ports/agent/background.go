package agent

import (
	"context"
	"time"
)

// BackgroundTaskStatus represents the lifecycle state of a background task.
type BackgroundTaskStatus string

const (
	BackgroundTaskStatusPending   BackgroundTaskStatus = "pending"
	BackgroundTaskStatusRunning   BackgroundTaskStatus = "running"
	BackgroundTaskStatusCompleted BackgroundTaskStatus = "completed"
	BackgroundTaskStatusFailed    BackgroundTaskStatus = "failed"
	BackgroundTaskStatusCancelled BackgroundTaskStatus = "cancelled"
)

// BackgroundTaskSummary provides a lightweight status view of a background task.
type BackgroundTaskSummary struct {
	ID          string
	Description string
	Status      BackgroundTaskStatus
	AgentType   string
	StartedAt   time.Time
	CompletedAt time.Time
	Error       string
}

// BackgroundTaskResult contains the full result of a completed background task.
type BackgroundTaskResult struct {
	ID          string
	Description string
	Status      BackgroundTaskStatus
	AgentType   string
	Answer      string
	Error       string
	RunID       string
	Iterations  int
	TokensUsed  int
	Duration    time.Duration
}

// BackgroundTaskDispatcher allows tools to dispatch, query, and collect
// background tasks without importing the domain layer.
type BackgroundTaskDispatcher interface {
	// Dispatch starts a new background task. Returns an error if the task ID
	// is already in use.
	Dispatch(ctx context.Context, taskID, description, prompt, agentType, causationID string) error

	// Status returns lightweight summaries for the requested task IDs.
	// Pass nil or empty slice to query all tasks.
	Status(ids []string) []BackgroundTaskSummary

	// Collect returns full results for completed tasks. When wait is true the
	// call blocks until the requested tasks finish or the timeout elapses.
	Collect(ids []string, wait bool, timeout time.Duration) []BackgroundTaskResult
}

// backgroundDispatcherKey is the context key for BackgroundTaskDispatcher.
type backgroundDispatcherKey struct{}

// WithBackgroundDispatcher returns a context carrying a BackgroundTaskDispatcher.
func WithBackgroundDispatcher(ctx context.Context, d BackgroundTaskDispatcher) context.Context {
	return context.WithValue(ctx, backgroundDispatcherKey{}, d)
}

// GetBackgroundDispatcher retrieves the BackgroundTaskDispatcher from ctx.
// Returns nil when no dispatcher is available.
func GetBackgroundDispatcher(ctx context.Context) BackgroundTaskDispatcher {
	if ctx == nil {
		return nil
	}
	d, _ := ctx.Value(backgroundDispatcherKey{}).(BackgroundTaskDispatcher)
	return d
}
