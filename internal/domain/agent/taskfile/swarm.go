package taskfile

import (
	"context"
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
	}
}

// SwarmScheduler executes a TaskFile using stage-batched parallel execution
// with adaptive concurrency scaling.
type SwarmScheduler struct {
	config     SwarmConfig
	dispatcher agent.BackgroundTaskDispatcher
	current    int
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
	return &SwarmScheduler{
		config:     cfg,
		dispatcher: dispatcher,
		current:    current,
	}
}

// ExecuteSwarm groups tasks into dependency layers, then runs each layer as a
// parallel swarm batch with adaptive concurrency. Returns after all layers
// have been executed.
func (s *SwarmScheduler) ExecuteSwarm(ctx context.Context, tf *TaskFile, causationID, statusPath string) (*ExecuteResult, error) {
	if err := Validate(tf); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	layers, err := TopologicalLayers(tf.Tasks)
	if err != nil {
		return nil, fmt.Errorf("topo layers: %w", err)
	}

	resolved := ResolveDefaults(tf)
	byID := make(map[string]TaskSpec, len(resolved))
	for _, t := range resolved {
		byID[t.ID] = t
	}

	sw := NewStatusWriter(statusPath)
	if err := sw.InitFromTaskFile(tf); err != nil {
		return nil, fmt.Errorf("init status: %w", err)
	}

	var allTaskIDs []string
	for _, layer := range layers {
		allTaskIDs = append(allTaskIDs, layer...)
	}

	for _, layer := range layers {
		if err := s.executeLayer(ctx, layer, byID, causationID); err != nil {
			sw.SyncOnce(s.dispatcher, allTaskIDs)
			return nil, err
		}

		layerResults := s.dispatcher.Collect(layer, true, s.config.StageTimeout)
		s.adjustConcurrency(layerResults)
		sw.SyncOnce(s.dispatcher, allTaskIDs)
	}

	return &ExecuteResult{
		PlanID:     tf.PlanID,
		TaskIDs:    allTaskIDs,
		StatusPath: statusPath,
	}, nil
}

// executeLayer dispatches all tasks in a layer concurrently, bounded by the
// current concurrency limit. DependsOn fields are cleared since the layer
// ordering already guarantees dependencies are satisfied.
func (s *SwarmScheduler) executeLayer(ctx context.Context, layer []string, byID map[string]TaskSpec, causationID string) error {
	sem := make(chan struct{}, s.current)
	var mu sync.Mutex
	var firstErr error

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
				if firstErr == nil {
					firstErr = fmt.Errorf("dispatch task %q: %w", taskID, err)
				}
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	return firstErr
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
