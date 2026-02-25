package taskfile

import (
	"context"
	"fmt"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// ExecuteResult captures the outcome of a TaskFile execution.
type ExecuteResult struct {
	PlanID     string
	TaskIDs    []string
	StatusPath string
}

// Executor orchestrates TaskFile execution by translating specs into
// BackgroundDispatchRequests and delegating to the existing dispatcher.
type Executor struct {
	dispatcher agent.BackgroundTaskDispatcher
}

// NewExecutor creates an Executor backed by the given dispatcher.
func NewExecutor(dispatcher agent.BackgroundTaskDispatcher) *Executor {
	return &Executor{dispatcher: dispatcher}
}

// Execute validates, resolves, and dispatches all tasks in the TaskFile.
// It writes initial status to statusPath and starts a polling goroutine.
// Returns immediately after dispatch (async).
func (e *Executor) Execute(ctx context.Context, tf *TaskFile, causationID, statusPath string) (*ExecuteResult, error) {
	if err := Validate(tf); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	order, err := TopologicalOrder(tf.Tasks)
	if err != nil {
		return nil, fmt.Errorf("topo sort: %w", err)
	}

	resolved := ResolveDefaults(tf)

	// Build index for topo-ordered dispatch.
	byID := make(map[string]TaskSpec, len(resolved))
	for _, t := range resolved {
		byID[t.ID] = t
	}

	// Init status file.
	sw := NewStatusWriter(statusPath)
	if err := sw.InitFromTaskFile(tf); err != nil {
		return nil, fmt.Errorf("init status: %w", err)
	}

	// Dispatch in topological order.
	var taskIDs []string
	for _, id := range order {
		spec := byID[id]
		req := SpecToDispatchRequest(spec, causationID)
		if err := e.dispatcher.Dispatch(ctx, req); err != nil {
			sw.Stop()
			return nil, fmt.Errorf("dispatch task %q: %w", id, err)
		}
		taskIDs = append(taskIDs, id)
	}

	// Start status polling.
	sw.StartPolling(e.dispatcher, taskIDs, 2*time.Second)

	return &ExecuteResult{
		PlanID:     tf.PlanID,
		TaskIDs:    taskIDs,
		StatusPath: statusPath,
	}, nil
}

// ExecuteAndWait dispatches tasks and blocks until all complete or the timeout elapses.
func (e *Executor) ExecuteAndWait(ctx context.Context, tf *TaskFile, causationID, statusPath string, timeout time.Duration) (*ExecuteResult, error) {
	result, err := e.Execute(ctx, tf, causationID, statusPath)
	if err != nil {
		return nil, err
	}

	// Block until all tasks are done.
	_ = e.dispatcher.Collect(result.TaskIDs, true, timeout)

	// Final status sync.
	sw := NewStatusWriter(statusPath)
	sw.SyncOnce(e.dispatcher, result.TaskIDs)

	return result, nil
}
