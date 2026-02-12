package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	id "alex/internal/shared/utils/id"
)

// Executor dispatches a single agent task and returns an identifier for tracking.
type Executor interface {
	Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (taskID string, err error)
}

// CoordinatorTaskRunner captures the minimal coordinator dependency the kernel needs.
type CoordinatorTaskRunner interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// CoordinatorExecutor adapts AgentCoordinator.ExecuteTask for kernel dispatch.
type CoordinatorExecutor struct {
	coordinator CoordinatorTaskRunner
	timeout     time.Duration
}

var errKernelNoRealToolAction = errors.New("kernel dispatch completed without successful real tool action")

// NewCoordinatorExecutor creates an executor backed by the given AgentCoordinator.
func NewCoordinatorExecutor(coordinator CoordinatorTaskRunner, timeout time.Duration) *CoordinatorExecutor {
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

	result, err := e.coordinator.ExecuteTask(execCtx, prompt, sessionID, nil)
	if err != nil {
		return "", err
	}
	if !containsSuccessfulRealToolExecution(result) {
		return "", errKernelNoRealToolAction
	}
	return sessionID, nil
}

func containsSuccessfulRealToolExecution(result *agent.TaskResult) bool {
	if result == nil {
		return false
	}
	callNames := make(map[string]string)
	hasToolResults := false
	sawSuccessfulUnknown := false
	for _, msg := range result.Messages {
		for _, call := range msg.ToolCalls {
			callNames[call.ID] = call.Name
		}
	}
	for _, msg := range result.Messages {
		if len(msg.ToolResults) > 0 {
			hasToolResults = true
		}
		for _, toolResult := range msg.ToolResults {
			name := callNames[toolResult.CallID]
			if name == "" {
				if toolResult.Error == nil {
					sawSuccessfulUnknown = true
				}
				continue
			}
			if toolResult.Error == nil && !isOrchestrationTool(name) {
				return true
			}
		}
	}
	if hasToolResults {
		return sawSuccessfulUnknown
	}
	// Fallback for providers that expose calls without emitting structured tool results.
	for _, msg := range result.Messages {
		for _, call := range msg.ToolCalls {
			if !isOrchestrationTool(call.Name) {
				return true
			}
		}
	}
	return false
}

func isOrchestrationTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plan", "clearify", "clarify", "todo_read", "todo_update", "attention", "context_checkpoint", "request_user":
		return true
	default:
		return false
	}
}
