package kernel

import "context"

// Store is the persistence port for the dispatch queue.
// It does NOT store agent state — that lives in STATE.md.
type Store interface {
	// EnsureSchema creates the dispatch table if it does not exist.
	EnsureSchema(ctx context.Context) error

	// EnqueueDispatches inserts a batch of dispatch specs as pending rows.
	EnqueueDispatches(ctx context.Context, kernelID, cycleID string, specs []DispatchSpec) ([]Dispatch, error)

	// ClaimDispatches atomically claims up to limit pending rows for workerID.
	ClaimDispatches(ctx context.Context, kernelID, workerID string, limit int) ([]Dispatch, error)

	// MarkDispatchRunning transitions a dispatch to running status.
	MarkDispatchRunning(ctx context.Context, dispatchID string) error

	// MarkDispatchDone transitions a dispatch to done with the resulting taskID.
	MarkDispatchDone(ctx context.Context, dispatchID, taskID string) error

	// MarkDispatchFailed transitions a dispatch to failed with an error message.
	MarkDispatchFailed(ctx context.Context, dispatchID, errMsg string) error

	// ListActiveDispatches returns all non-terminal dispatches for a kernel.
	ListActiveDispatches(ctx context.Context, kernelID string) ([]Dispatch, error)

	// ListRecentByAgent returns the most recent dispatch for each agent_id.
	ListRecentByAgent(ctx context.Context, kernelID string) (map[string]Dispatch, error)
}
