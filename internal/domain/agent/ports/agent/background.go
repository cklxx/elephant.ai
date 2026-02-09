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
	BackgroundTaskStatusBlocked   BackgroundTaskStatus = "blocked"
	BackgroundTaskStatusCompleted BackgroundTaskStatus = "completed"
	BackgroundTaskStatusFailed    BackgroundTaskStatus = "failed"
	BackgroundTaskStatusCancelled BackgroundTaskStatus = "cancelled"
)

// BackgroundDispatchRequest captures inputs for background task dispatch.
type BackgroundDispatchRequest struct {
	TaskID         string
	Description    string
	Prompt         string
	AgentType      string
	CausationID    string
	Config         map[string]string
	DependsOn      []string
	WorkspaceMode  WorkspaceMode
	FileScope      []string
	InheritContext bool
}

// BackgroundTaskSummary provides a lightweight status view of a background task.
type BackgroundTaskSummary struct {
	ID           string
	Description  string
	Status       BackgroundTaskStatus
	AgentType    string
	StartedAt    time.Time
	CompletedAt  time.Time
	Error        string
	Progress     *ExternalAgentProgress
	PendingInput *InputRequestSummary
	Elapsed      time.Duration
	Workspace    *WorkspaceAllocation
	FileScope    []string
	DependsOn    []string
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
	Dispatch(ctx context.Context, req BackgroundDispatchRequest) error

	// Status returns lightweight summaries for the requested task IDs.
	// Pass nil or empty slice to query all tasks.
	Status(ids []string) []BackgroundTaskSummary

	// Collect returns full results for completed tasks. When wait is true the
	// call blocks until the requested tasks finish or the timeout elapses.
	Collect(ids []string, wait bool, timeout time.Duration) []BackgroundTaskResult
}

// BackgroundTaskCanceller allows callers to cancel individual background tasks.
type BackgroundTaskCanceller interface {
	CancelBackgroundTask(ctx context.Context, taskID string) error
}

// ExternalInputResponder allows tools to reply to external agent input requests.
type ExternalInputResponder interface {
	ReplyExternalInput(ctx context.Context, resp InputResponse) error
}

// ExternalWorkspaceMerger allows tools to merge external agent workspaces.
type ExternalWorkspaceMerger interface {
	MergeExternalWorkspace(ctx context.Context, taskID string, strategy MergeStrategy) (*MergeResult, error)
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
