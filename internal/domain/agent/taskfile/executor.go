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
// It supports team (sequential dispatch with dep-based blocking) and
// swarm (stage-batched parallel) execution modes.
type Executor struct {
	dispatcher agent.BackgroundTaskDispatcher
	mode       ExecutionMode
	swarmCfg   SwarmConfig
}

// NewExecutor creates an Executor backed by the given dispatcher. The mode
// parameter selects the execution strategy (team, swarm, or auto). When auto,
// the mode is determined by analyzing the TaskFile's DAG structure.
func NewExecutor(dispatcher agent.BackgroundTaskDispatcher, mode ExecutionMode, swarmCfg SwarmConfig) *Executor {
	return &Executor{
		dispatcher: dispatcher,
		mode:       mode,
		swarmCfg:   swarmCfg,
	}
}

// resolveMode returns the concrete mode (team or swarm) for the given TaskFile.
func (e *Executor) resolveMode(tf *TaskFile) ExecutionMode {
	if e.mode == ModeAuto {
		return AnalyzeMode(tf)
	}
	return e.mode
}

// Execute validates, resolves, and dispatches all tasks in the TaskFile.
// For swarm mode, it blocks until all stages complete (swarm is inherently
// synchronous per-stage). For team mode, it returns immediately after dispatch.
func (e *Executor) Execute(ctx context.Context, tf *TaskFile, causationID, statusPath string) (*ExecuteResult, error) {
	if err := Validate(tf); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	if e.resolveMode(tf) == ModeSwarm {
		sched := NewSwarmScheduler(e.dispatcher, e.swarmCfg)
		return sched.executeSwarmValidated(ctx, tf, causationID, statusPath)
	}
	return e.executeTeamValidated(ctx, tf, causationID, statusPath)
}

// ExecuteAndWait dispatches tasks and blocks until all complete or the timeout elapses.
func (e *Executor) ExecuteAndWait(ctx context.Context, tf *TaskFile, causationID, statusPath string, timeout time.Duration) (*ExecuteResult, error) {
	if err := Validate(tf); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	if e.resolveMode(tf) == ModeSwarm {
		// Swarm execution is already synchronous (blocks per stage).
		sched := NewSwarmScheduler(e.dispatcher, e.swarmCfg)
		return sched.executeSwarmValidated(ctx, tf, causationID, statusPath)
	}

	result, err := e.executeTeamValidated(ctx, tf, causationID, statusPath)
	if err != nil {
		return nil, err
	}

	// Block until all tasks are done.
	_ = e.dispatcher.Collect(result.TaskIDs, true, timeout)

	// Final status sync. Rehydrate existing sidecar first so SyncOnce updates
	// the initialized task rows instead of operating on an empty in-memory file.
	sw := NewStatusWriter(statusPath, nil)
	if existing, readErr := ReadStatusFile(statusPath); readErr == nil && existing != nil {
		sw.RehydrateFrom(existing)
	}
	sw.SyncOnce(e.dispatcher, result.TaskIDs)

	return result, nil
}

// executeTeamValidated dispatches all tasks in topological order, relying on
// the dispatcher's dependency blocking. Caller must have validated tf already.
func (e *Executor) executeTeamValidated(ctx context.Context, tf *TaskFile, causationID, statusPath string) (*ExecuteResult, error) {
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
	sw := NewStatusWriter(statusPath, nil)
	if err := sw.InitFromTaskFile(tf); err != nil {
		return nil, fmt.Errorf("init status: %w", err)
	}

	// Dispatch in topological order.
	var taskIDs []string
	for _, id := range order {
		if err := ctx.Err(); err != nil {
			sw.Stop()
			return nil, err
		}
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
