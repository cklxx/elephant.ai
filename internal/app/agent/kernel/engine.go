package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	"github.com/robfig/cron/v3"
)

// cronParser is the standard 5-field cron parser used throughout the kernel.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

const (
	kernelRuntimeSectionStart = "<!-- KERNEL_RUNTIME:START -->"
	kernelRuntimeSectionEnd   = "<!-- KERNEL_RUNTIME:END -->"
)

// ValidateSchedule checks whether the given cron expression is valid.
// Called at build time to fail fast on misconfiguration.
func ValidateSchedule(expr string) error {
	_, err := cronParser.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
}

// Engine runs the kernel agent loop: a cron-driven cycle that reads STATE.md,
// plans dispatches, and executes them via the AgentCoordinator.
type Engine struct {
	config    KernelConfig
	stateFile *StateFile
	store     kerneldomain.Store
	planner   Planner
	executor  Executor
	logger    logging.Logger
	notifier  CycleNotifier // optional; called after non-empty cycles

	stopped  chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup // tracks in-flight RunCycle goroutines
}

type staleDispatchRecoverer interface {
	RecoverStaleRunning(ctx context.Context, kernelID string) (int, error)
}

// SetNotifier registers an optional callback invoked after each non-empty cycle.
func (e *Engine) SetNotifier(fn func(ctx context.Context, result *kerneldomain.CycleResult, err error)) {
	e.notifier = fn
}

// NewEngine creates a new kernel engine.
func NewEngine(
	config KernelConfig,
	stateFile *StateFile,
	store kerneldomain.Store,
	planner Planner,
	executor Executor,
	logger logging.Logger,
) *Engine {
	return &Engine{
		config:    config,
		stateFile: stateFile,
		store:     store,
		planner:   planner,
		executor:  executor,
		logger:    logging.OrNop(logger),
		stopped:   make(chan struct{}),
	}
}

// RunCycle executes one PERCEIVE-ORIENT-DECIDE-ACT-UPDATE cycle.
func (e *Engine) RunCycle(ctx context.Context) (result *kerneldomain.CycleResult, err error) {
	start := time.Now()
	cycleID := id.NewRunID()
	defer func() {
		e.persistCycleRuntimeState(result, err)
	}()

	// 1. Read STATE.md (opaque text).
	stateContent, err := e.stateFile.Read()
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if stateContent == "" {
		if seedErr := e.stateFile.Seed(e.config.SeedState); seedErr != nil {
			return nil, fmt.Errorf("seed state: %w", seedErr)
		}
		stateContent = e.config.SeedState
	}

	// 2. Recover stale running dispatches from previous interrupted cycles.
	if recoverer, ok := e.store.(staleDispatchRecoverer); ok {
		recovered, recoverErr := recoverer.RecoverStaleRunning(ctx, e.config.KernelID)
		if recoverErr != nil {
			e.logger.Warn("Kernel: recover stale dispatches failed: %v", recoverErr)
		} else if recovered > 0 {
			e.logger.Warn("Kernel: recovered %d stale running dispatch(es)", recovered)
		}
	}

	// 3. Query each agent's most recent dispatch status.
	recentByAgent, err := e.store.ListRecentByAgent(ctx, e.config.KernelID)
	if err != nil {
		e.logger.Warn("Kernel: list recent dispatches failed: %v", err)
		recentByAgent = map[string]kerneldomain.Dispatch{}
	}

	// 4. Plan — generate dispatch specs (STATE content injected into prompts).
	specs, err := e.planner.Plan(ctx, stateContent, recentByAgent)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	if len(specs) == 0 {
		return &kerneldomain.CycleResult{
			CycleID:  cycleID,
			KernelID: e.config.KernelID,
			Status:   kerneldomain.CycleSuccess,
			Duration: time.Since(start),
		}, nil
	}

	// 5. Enqueue dispatches to Postgres.
	dispatches, err := e.store.EnqueueDispatches(ctx, e.config.KernelID, cycleID, specs)
	if err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}

	// 6. Execute dispatches in parallel (bounded by MaxConcurrent).
	result = e.executeDispatches(ctx, cycleID, dispatches)
	result.Duration = time.Since(start)

	return result, nil
}

func (e *Engine) persistCycleRuntimeState(result *kerneldomain.CycleResult, cycleErr error) {
	stateContent, err := e.stateFile.Read()
	if err != nil {
		e.logger.Warn("Kernel: read state for runtime persistence failed: %v", err)
		return
	}
	if strings.TrimSpace(stateContent) == "" {
		stateContent = e.config.SeedState
	}

	runtimeBlock := renderKernelRuntimeBlock(result, cycleErr, time.Now())
	updated := upsertKernelRuntimeBlock(stateContent, runtimeBlock)
	if err := e.stateFile.Write(updated); err != nil {
		e.logger.Warn("Kernel: persist runtime state failed: %v", err)
	}
}

func renderKernelRuntimeBlock(result *kerneldomain.CycleResult, cycleErr error, now time.Time) string {
	lines := []string{
		"## kernel_runtime",
		fmt.Sprintf("- updated_at: %s", now.UTC().Format(time.RFC3339)),
	}
	if result == nil {
		lines = append(lines,
			"- cycle_id: (none)",
			"- status: error",
			"- dispatched: 0",
			"- succeeded: 0",
			"- failed: 0",
			"- failed_agents: (none)",
			"- duration_ms: 0",
		)
	} else {
		failedAgents := "(none)"
		if len(result.FailedAgents) > 0 {
			failedAgents = strings.Join(result.FailedAgents, ", ")
		}
		lines = append(lines,
			fmt.Sprintf("- cycle_id: %s", result.CycleID),
			fmt.Sprintf("- status: %s", result.Status),
			fmt.Sprintf("- dispatched: %d", result.Dispatched),
			fmt.Sprintf("- succeeded: %d", result.Succeeded),
			fmt.Sprintf("- failed: %d", result.Failed),
			fmt.Sprintf("- failed_agents: %s", failedAgents),
			fmt.Sprintf("- duration_ms: %d", result.Duration.Milliseconds()),
		)
	}
	if cycleErr != nil {
		lines = append(lines, fmt.Sprintf("- error: %s", cycleErr.Error()))
	} else {
		lines = append(lines, "- error: (none)")
	}
	return strings.Join(lines, "\n")
}

func upsertKernelRuntimeBlock(content, runtimeBlock string) string {
	block := kernelRuntimeSectionStart + "\n" + runtimeBlock + "\n" + kernelRuntimeSectionEnd

	start := strings.Index(content, kernelRuntimeSectionStart)
	end := strings.Index(content, kernelRuntimeSectionEnd)
	if start >= 0 && end > start {
		end += len(kernelRuntimeSectionEnd)
		prefix := strings.TrimRight(content[:start], "\n")
		suffix := strings.TrimLeft(content[end:], "\n")

		var out strings.Builder
		if prefix != "" {
			out.WriteString(prefix)
			out.WriteString("\n\n")
		}
		out.WriteString(block)
		if suffix != "" {
			out.WriteString("\n\n")
			out.WriteString(suffix)
		}
		out.WriteString("\n")
		return out.String()
	}

	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return block + "\n"
	}
	return trimmed + "\n\n" + block + "\n"
}

// executeDispatches runs dispatches concurrently with a bounded semaphore.
func (e *Engine) executeDispatches(ctx context.Context, cycleID string, dispatches []kerneldomain.Dispatch) *kerneldomain.CycleResult {
	result := &kerneldomain.CycleResult{
		CycleID:    cycleID,
		KernelID:   e.config.KernelID,
		Dispatched: len(dispatches),
	}

	maxConcurrent := e.config.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	sem := make(chan struct{}, maxConcurrent)

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, d := range dispatches {
		wg.Add(1)
		go func(d kerneldomain.Dispatch) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := e.store.MarkDispatchRunning(ctx, d.DispatchID); err != nil {
				e.logger.Warn("Kernel: mark running %s: %v", d.DispatchID, err)
			}

			// Copy metadata to avoid concurrent mutation of the shared map.
			meta := make(map[string]string, len(d.Metadata)+2)
			for k, v := range d.Metadata {
				meta[k] = v
			}
			if e.config.UserID != "" {
				meta["user_id"] = e.config.UserID
			}
			if e.config.Channel != "" {
				meta["channel"] = e.config.Channel
			}

			taskID, execErr := e.executor.Execute(ctx, d.AgentID, d.Prompt, meta)

			mu.Lock()
			defer mu.Unlock()

			if execErr != nil {
				result.Failed++
				result.FailedAgents = append(result.FailedAgents, d.AgentID)
				if markErr := e.store.MarkDispatchFailed(ctx, d.DispatchID, execErr.Error()); markErr != nil {
					e.logger.Warn("Kernel: mark failed %s: %v", d.DispatchID, markErr)
				}
				e.logger.Warn("Kernel: dispatch %s (agent=%s) failed: %v", d.DispatchID, d.AgentID, execErr)
			} else {
				result.Succeeded++
				if markErr := e.store.MarkDispatchDone(ctx, d.DispatchID, taskID); markErr != nil {
					e.logger.Warn("Kernel: mark done %s: %v", d.DispatchID, markErr)
				}
			}
		}(d)
	}

	wg.Wait()

	switch {
	case result.Failed == 0:
		result.Status = kerneldomain.CycleSuccess
	case result.Succeeded > 0:
		result.Status = kerneldomain.CyclePartialSuccess
	default:
		result.Status = kerneldomain.CycleFailed
	}

	return result
}

// Run starts the main loop, scheduling RunCycle according to the cron expression.
// It blocks until the context is cancelled or Stop is called.
func (e *Engine) Run(ctx context.Context) {
	sched, err := cronParser.Parse(e.config.Schedule)
	if err != nil {
		// Schedule was already validated at build time; this is defensive.
		e.logger.Warn("Kernel: invalid schedule %q: %v", e.config.Schedule, err)
		return
	}

	e.logger.Info("Kernel[%s] starting (schedule=%s)", e.config.KernelID, e.config.Schedule)

	for {
		nextRun := sched.Next(time.Now())
		timer := time.NewTimer(time.Until(nextRun))

		select {
		case <-ctx.Done():
			timer.Stop()
			e.logger.Info("Kernel[%s] stopped (context cancelled)", e.config.KernelID)
			return
		case <-e.stopped:
			timer.Stop()
			e.logger.Info("Kernel[%s] stopped", e.config.KernelID)
			return
		case <-timer.C:
			e.wg.Add(1)
			func() {
				defer e.wg.Done()
				result, cycleErr := e.RunCycle(ctx)
				if cycleErr != nil {
					e.logger.Warn("Kernel[%s] RunCycle error: %v", e.config.KernelID, cycleErr)
				} else {
					e.logger.Info("Kernel[%s] cycle %s: %s (dispatched=%d ok=%d fail=%d %s)",
						e.config.KernelID, result.CycleID, result.Status,
						result.Dispatched, result.Succeeded, result.Failed, result.Duration)
				}
				// Notify on non-empty cycles or errors.
				if e.notifier != nil {
					if cycleErr != nil || (result != nil && result.Dispatched > 0) {
						e.notifier(ctx, result, cycleErr)
					}
				}
			}()
		}
	}
}

// Stop signals the engine to exit the Run loop.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() { close(e.stopped) })
}

// Name returns the subsystem name for lifecycle management.
func (e *Engine) Name() string { return "kernel" }

// Drain gracefully stops the engine and waits for any in-flight cycle to finish.
func (e *Engine) Drain(_ context.Context) error {
	e.Stop()
	e.wg.Wait()
	return nil
}
