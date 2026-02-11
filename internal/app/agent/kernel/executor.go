package kernel

import (
	"context"
	"fmt"
	"time"

	agentcoordinator "alex/internal/app/agent/coordinator"
	id "alex/internal/shared/utils/id"
)

// Executor dispatches a single agent task and returns an identifier for tracking.
type Executor interface {
	Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (taskID string, err error)
}

// CoordinatorExecutor adapts AgentCoordinator.ExecuteTask for kernel dispatch.
type CoordinatorExecutor struct {
	coordinator *agentcoordinator.AgentCoordinator
	timeout     time.Duration
}

// NewCoordinatorExecutor creates an executor backed by the given AgentCoordinator.
func NewCoordinatorExecutor(coordinator *agentcoordinator.AgentCoordinator, timeout time.Duration) *CoordinatorExecutor {
	return &CoordinatorExecutor{
		coordinator: coordinator,
		timeout:     timeout,
	}
}

// Execute runs a task through the AgentCoordinator and returns the session ID as task identifier.
func (e *CoordinatorExecutor) Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (string, error) {
	runID := id.NewRunID()
	sessionID := fmt.Sprintf("kernel-%s-%s", agentID, runID)

	execCtx := id.WithRunID(ctx, runID)
	execCtx = id.WithSessionID(execCtx, sessionID)

	if uid, ok := meta["user_id"]; ok && uid != "" {
		execCtx = id.WithUserID(execCtx, uid)
	}

	if e.timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, e.timeout)
		defer cancel()
	}

	_, err := e.coordinator.ExecuteTask(execCtx, prompt, sessionID, nil)
	return sessionID, err
}
