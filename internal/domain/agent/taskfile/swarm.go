package taskfile

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// SwarmConfig controls the adaptive concurrency behavior of the swarm scheduler.
type SwarmConfig struct {
	InitialConcurrency int           `yaml:"initial_concurrency"`
	MaxConcurrency     int           `yaml:"max_concurrency"`
	ScaleUpThreshold   float64       `yaml:"scale_up_threshold"`
	ScaleDownThreshold float64       `yaml:"scale_down_threshold"`
	ScaleStep          int           `yaml:"scale_step"`
	StageTimeout       time.Duration `yaml:"stage_timeout"`
	// StaleRetryMax is the maximum number of times a stale or timed-out task
	// will be re-dispatched per layer. 0 means no retries.
	StaleRetryMax int `yaml:"stale_retry_max"`
}

// DefaultSwarmConfig returns sensible defaults for swarm execution.
func DefaultSwarmConfig() SwarmConfig {
	return SwarmConfig{
		InitialConcurrency: 5,
		MaxConcurrency:     50,
		ScaleUpThreshold:   0.9,
		ScaleDownThreshold: 0.7,
		ScaleStep:          2,
		StageTimeout:       5 * time.Minute,
		StaleRetryMax:      2,
	}
}

// SwarmScheduler executes a TaskFile using stage-batched parallel execution
// with adaptive concurrency scaling.
type SwarmScheduler struct {
	config     SwarmConfig
	dispatcher agent.BackgroundTaskDispatcher
	// current is the active concurrency limit, adjusted between layers.
	// Not goroutine-safe: only read/written from the single-threaded
	// executeSwarmValidated loop (between layers, never during dispatch).
	current int
}

// NewSwarmScheduler creates a scheduler backed by the given dispatcher.
func NewSwarmScheduler(dispatcher agent.BackgroundTaskDispatcher, cfg SwarmConfig) *SwarmScheduler {
	current := cfg.InitialConcurrency
	if current <= 0 {
		current = DefaultSwarmConfig().InitialConcurrency
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = DefaultSwarmConfig().MaxConcurrency
	}
	if cfg.StageTimeout <= 0 {
		cfg.StageTimeout = DefaultSwarmConfig().StageTimeout
	}
	if cfg.ScaleStep <= 0 {
		cfg.ScaleStep = DefaultSwarmConfig().ScaleStep
	}
	if current > cfg.MaxConcurrency {
		current = cfg.MaxConcurrency
	}
	return &SwarmScheduler{
		config:     cfg,
		dispatcher: dispatcher,
		current:    current,
	}
}

// ExecuteSwarm validates and executes tasks using stage-batched parallelism.
func (s *SwarmScheduler) ExecuteSwarm(ctx context.Context, tf *TaskFile, causationID, statusPath string) (*ExecuteResult, error) {
	if err := Validate(tf); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return s.executeSwarmValidated(ctx, tf, causationID, statusPath)
}

// executeSwarmValidated groups tasks into dependency layers, then runs each
// layer as a parallel swarm batch with adaptive concurrency. Caller must have
// validated tf already.
func (s *SwarmScheduler) executeSwarmValidated(ctx context.Context, tf *TaskFile, causationID, statusPath string) (*ExecuteResult, error) {
	layers, err := TopologicalLayers(tf.Tasks)
	if err != nil {
		return nil, fmt.Errorf("topo layers: %w", err)
	}

	resolved := ResolveDefaults(tf)
	byID := make(map[string]TaskSpec, len(resolved))
	for _, t := range resolved {
		byID[t.ID] = t
	}

	sw := NewStatusWriter(statusPath, nil)
	if err := sw.InitFromTaskFile(tf); err != nil {
		return nil, fmt.Errorf("init status: %w", err)
	}

	var allTaskIDs []string
	for _, layer := range layers {
		allTaskIDs = append(allTaskIDs, layer...)
	}

	// retryCounts tracks how many times each base task ID has been retried.
	retryCounts := make(map[string]int)

	for _, layer := range layers {
		if err := ctx.Err(); err != nil {
			sw.SyncOnce(s.dispatcher, allTaskIDs)
			return nil, err
		}
		activeIDs := append([]string(nil), layer...)

		for {
			if err := s.executeLayer(ctx, activeIDs, byID, causationID); err != nil {
				sw.SyncOnce(s.dispatcher, allTaskIDs)
				return nil, err
			}

			results := s.dispatcher.Collect(activeIDs, true, s.config.StageTimeout)
			s.adjustConcurrency(results)

			retryIDs := s.buildRetryBatch(ctx, activeIDs, results, byID, retryCounts)
			if len(retryIDs) == 0 {
				break
			}
			activeIDs = retryIDs
			allTaskIDs = append(allTaskIDs, retryIDs...)
		}
		sw.SyncOnce(s.dispatcher, allTaskIDs)
	}

	return &ExecuteResult{
		PlanID:     tf.PlanID,
		TaskIDs:    allTaskIDs,
		StatusPath: statusPath,
	}, nil
}

// buildRetryBatch identifies stale or timed-out tasks from a completed layer
// and returns a list of retry task IDs to dispatch in the next iteration.
// It updates byID and retryCounts in place.
func (s *SwarmScheduler) buildRetryBatch(
	ctx context.Context,
	layerIDs []string,
	results []agent.BackgroundTaskResult,
	byID map[string]TaskSpec,
	retryCounts map[string]int,
) []string {
	if s.config.StaleRetryMax <= 0 {
		return nil
	}

	resultByID := make(map[string]agent.BackgroundTaskResult, len(results))
	for _, r := range results {
		resultByID[r.ID] = r
	}
	statusByID := make(map[string]agent.BackgroundTaskSummary, len(layerIDs))
	for _, summary := range s.dispatcher.Status(layerIDs) {
		statusByID[summary.ID] = summary
	}

	var retryIDs []string
	for _, origID := range layerIDs {
		baseID := BaseTaskID(origID)

		// Skip if already terminal in collect results.
		if r, ok := resultByID[origID]; ok {
			switch r.Status {
			case agent.BackgroundTaskStatusCompleted,
				agent.BackgroundTaskStatusFailed,
				agent.BackgroundTaskStatusCancelled:
				continue
			}
		}

		// For non-terminal tasks: consult Status to distinguish stale from
		// legitimately running tasks that just hit the stage timeout.
		needsRetry := false
		if sum, ok := statusByID[origID]; ok {
			switch sum.Status {
			case agent.BackgroundTaskStatusCompleted,
				agent.BackgroundTaskStatusFailed,
				agent.BackgroundTaskStatusCancelled:
				// Terminal — no retry needed.
			default:
				needsRetry = true
			}
			if sum.Stale {
				needsRetry = true
			}
		}
		if !needsRetry {
			continue
		}

		// Enforce retry cap.
		if retryCounts[baseID] >= s.config.StaleRetryMax {
			continue
		}

		// Best-effort cancel (no-op when not supported).
		if canceller, ok := s.dispatcher.(agent.BackgroundTaskCanceller); ok {
			_ = canceller.CancelBackgroundTask(ctx, origID)
		}

		retryCounts[baseID]++
		retryID := fmt.Sprintf("%s-retry-%d", baseID, retryCounts[baseID])
		if origSpec, ok := byID[origID]; ok {
			retrySpec := origSpec
			retrySpec.ID = retryID
			byID[retryID] = retrySpec
		} else if baseSpec, ok := byID[baseID]; ok {
			retrySpec := baseSpec
			retrySpec.ID = retryID
			byID[retryID] = retrySpec
		}
		retryIDs = append(retryIDs, retryID)
	}
	return retryIDs
}

// executeLayer dispatches all tasks in a layer concurrently, bounded by the
// current concurrency limit. DependsOn fields are cleared since the layer
// ordering already guarantees dependencies are satisfied.
func (s *SwarmScheduler) executeLayer(ctx context.Context, layer []string, byID map[string]TaskSpec, causationID string) error {
	sem := make(chan struct{}, s.current)
	var mu sync.Mutex
	var errs []error

	var wg sync.WaitGroup
	for _, id := range layer {
		wg.Add(1)
		go func(taskID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			spec := byID[taskID]
			req := SpecToDispatchRequest(spec, causationID)
			// Clear DependsOn — layer ordering already handles deps, and the
			// dispatcher would otherwise block on them.
			req.DependsOn = nil

			if err := s.dispatcher.Dispatch(ctx, req); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("dispatch task %q: %w", taskID, err))
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	return errors.Join(errs...)
}

// adjustConcurrency scales the concurrency limit up or down based on the
// success rate of the completed layer.
func (s *SwarmScheduler) adjustConcurrency(results []agent.BackgroundTaskResult) {
	if len(results) == 0 {
		return
	}

	succeeded := 0
	for _, r := range results {
		if r.Status == agent.BackgroundTaskStatusCompleted {
			succeeded++
		}
	}
	rate := float64(succeeded) / float64(len(results))

	if rate >= s.config.ScaleUpThreshold && s.current < s.config.MaxConcurrency {
		s.current += s.config.ScaleStep
		if s.current > s.config.MaxConcurrency {
			s.current = s.config.MaxConcurrency
		}
	} else if rate < s.config.ScaleDownThreshold && s.current > 1 {
		s.current -= s.config.ScaleStep
		if s.current < 1 {
			s.current = 1
		}
	}
}
