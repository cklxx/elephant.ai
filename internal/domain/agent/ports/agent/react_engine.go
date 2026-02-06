package agent

import "context"

// ReactiveExecutor defines the contract for orchestrating agent workflows.
type ReactiveExecutor interface {
	SolveTask(ctx context.Context, task string, state *TaskState, services ServiceBundle) (*TaskResult, error)
	SetEventListener(listener EventListener)
	GetEventListener() EventListener
}
