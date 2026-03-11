package kernel

import "context"

// Store persists dispatch records for the kernel engine.
type Store interface {
	// Save persists or updates a dispatch record.
	Save(ctx context.Context, d Dispatch) error

	// Get returns a single dispatch by ID, or an error if not found.
	Get(ctx context.Context, dispatchID string) (Dispatch, error)

	// ListRecentByAgent returns the most recent N dispatches per agent for the
	// given kernel, ordered newest-first. Used by the planner to assess agent
	// workload and recent outcomes.
	ListRecentByAgent(ctx context.Context, kernelID string, perAgent int) (map[string][]Dispatch, error)

	// RecoverStaleRunning marks dispatches stuck in "running" state beyond the
	// lease duration as failed. Returns the number of recovered dispatches.
	RecoverStaleRunning(ctx context.Context, kernelID string) (int, error)

	// PurgeTerminalDispatches removes terminal dispatches older than the
	// configured retention period. Returns the count of purged records.
	PurgeTerminalDispatches(ctx context.Context, kernelID string) (int, error)
}
