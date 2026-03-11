package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// Planner decides which dispatches to create for a cycle.
type Planner interface {
	Plan(ctx context.Context, state string, recentByAgent map[string][]domain.Dispatch) ([]domain.DispatchSpec, error)
}

// Executor runs a single dispatch and returns a summary or error.
type Executor interface {
	Execute(ctx context.Context, spec domain.DispatchSpec) (summary string, err error)
}

// CycleNotifier receives notifications about cycle outcomes.
type CycleNotifier interface {
	NotifyCycleComplete(entry domain.CycleHistoryEntry)
	NotifyStaleRecovered(count int)
}

// StateReader reads the current kernel state (STATE.md).
type StateReader interface {
	ReadState(ctx context.Context, kernelID string) (string, error)
}

// StateWriter persists cycle history into the kernel state.
type StateWriter interface {
	WriteCycleHistory(ctx context.Context, kernelID string, entries []domain.CycleHistoryEntry) error
}

// Engine is the kernel dispatch loop that orchestrates the PODATA cycle:
// Perceive → Orient → Decide → Act → Update.
type Engine struct {
	config   EngineConfig
	store    domain.Store
	planner  Planner
	executor Executor
	notifier CycleNotifier
	stateR   StateReader
	stateW   StateWriter
	logger   logging.Logger
	now      func() time.Time

	// Mutable state protected by mu.
	mu            sync.Mutex
	cycleHistory  []domain.CycleHistoryEntry
	consecutiveFails int
	lastSuccess      time.Time
}

// EngineDeps groups the dependencies for the kernel engine.
type EngineDeps struct {
	Store    domain.Store
	Planner  Planner
	Executor Executor
	Notifier CycleNotifier
	StateR   StateReader
	StateW   StateWriter
	Logger   logging.Logger
}

// NewEngine creates a kernel dispatch engine.
func NewEngine(cfg EngineConfig, deps EngineDeps) *Engine {
	logger := deps.Logger
	if logger == nil {
		logger = logging.Nop()
	}
	return &Engine{
		config:   cfg,
		store:    deps.Store,
		planner:  deps.Planner,
		executor: deps.Executor,
		notifier: deps.Notifier,
		stateR:   deps.StateR,
		stateW:   deps.StateW,
		logger:   logger,
		now:      time.Now,
	}
}

// RunCycle executes a single PODATA cycle. Every error path is handled:
// partial dispatch failures are recorded without aborting the cycle, and
// infrastructure errors (store, state) are propagated to the caller.
func (e *Engine) RunCycle(ctx context.Context) error {
	cycleID := fmt.Sprintf("cycle-%d", e.now().Unix())

	// 1. PERCEIVE — read current state.
	state, err := e.stateR.ReadState(ctx, e.config.KernelID)
	if err != nil {
		return e.recordCycleFailure(ctx, cycleID, fmt.Errorf("perceive: read state: %w", err))
	}

	// 2. ORIENT — recover stale dispatches.
	recovered, err := e.store.RecoverStaleRunning(ctx, e.config.KernelID)
	if err != nil {
		return e.recordCycleFailure(ctx, cycleID, fmt.Errorf("orient: recover stale: %w", err))
	}
	if recovered > 0 {
		e.logger.Warn("Kernel: recovered %d stale dispatch(es)", recovered)
		if e.notifier != nil {
			e.notifier.NotifyStaleRecovered(recovered)
		}
	}

	// Fetch recent dispatches per agent for planner context.
	recentByAgent, err := e.store.ListRecentByAgent(ctx, e.config.KernelID, 5)
	if err != nil {
		return e.recordCycleFailure(ctx, cycleID, fmt.Errorf("orient: list recent: %w", err))
	}

	// 3. DECIDE — ask planner for dispatch specs.
	specs, err := e.planner.Plan(ctx, state, recentByAgent)
	if err != nil {
		return e.recordCycleFailure(ctx, cycleID, fmt.Errorf("decide: plan: %w", err))
	}

	// 4. ACT — execute dispatches with bounded concurrency.
	results := e.executeDispatches(ctx, cycleID, specs)

	// 5. UPDATE — persist cycle outcome.
	entry := e.buildCycleEntry(cycleID, results)
	if persistErr := e.persistCycleHistory(ctx, entry); persistErr != nil {
		e.logger.Warn("Kernel: persist cycle history failed: %v", persistErr)
	}

	// 6. GC — purge old terminal dispatches.
	purged, purgeErr := e.store.PurgeTerminalDispatches(ctx, e.config.KernelID)
	if purgeErr != nil {
		e.logger.Warn("Kernel: purge terminal dispatches failed: %v", purgeErr)
	} else if purged > 0 {
		e.logger.Info("Kernel: purged %d old terminal dispatch(es)", purged)
	}

	if e.notifier != nil {
		e.notifier.NotifyCycleComplete(entry)
	}

	// Track consecutive failures for alerting.
	e.mu.Lock()
	if entry.Failed > 0 && entry.Succeeded == 0 {
		e.consecutiveFails++
		if e.consecutiveFails%e.config.alertRepeat() == 0 {
			e.logger.Warn("Kernel: %d consecutive cycles with all dispatches failing", e.consecutiveFails)
		}
	} else {
		e.consecutiveFails = 0
		e.lastSuccess = e.now()
	}
	e.mu.Unlock()

	return nil
}

// dispatchResult tracks the outcome of a single dispatch execution.
type dispatchResult struct {
	spec    domain.DispatchSpec
	summary string
	err     error
}

// executeDispatches runs dispatch specs with bounded concurrency and records
// results in the store. Partial failures do not abort the cycle.
func (e *Engine) executeDispatches(ctx context.Context, cycleID string, specs []domain.DispatchSpec) []dispatchResult {
	if len(specs) == 0 {
		return nil
	}

	maxConc := e.config.maxConcurrent()
	sem := make(chan struct{}, maxConc)
	results := make([]dispatchResult, len(specs))
	var wg sync.WaitGroup

	for i, spec := range specs {
		// Create and persist the dispatch record as pending.
		d := domain.Dispatch{
			DispatchID: fmt.Sprintf("%s-d%d", cycleID, i),
			KernelID:   e.config.KernelID,
			CycleID:    cycleID,
			AgentName:  spec.AgentName,
			Prompt:     spec.Prompt,
			Status:     domain.DispatchPending,
			CreatedAt:  e.now(),
			UpdatedAt:  e.now(),
		}
		if saveErr := e.store.Save(ctx, d); saveErr != nil {
			e.logger.Warn("Kernel: save pending dispatch %s: %v", d.DispatchID, saveErr)
			results[i] = dispatchResult{spec: spec, err: saveErr}
			continue
		}

		wg.Add(1)
		idx := i
		s := spec
		did := d.DispatchID

		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Mark running.
			e.updateDispatchStatus(ctx, did, domain.DispatchRunning, "", "")

			summary, execErr := e.executor.Execute(ctx, s)
			if execErr != nil {
				errMsg := utils.TruncateWithEllipsis(execErr.Error(), 500)
				e.updateDispatchStatus(ctx, did, domain.DispatchFailed, "", errMsg)
				results[idx] = dispatchResult{spec: s, err: execErr}
			} else {
				truncSummary := utils.TruncateWithEllipsis(summary, 500)
				e.updateDispatchStatus(ctx, did, domain.DispatchDone, truncSummary, "")
				results[idx] = dispatchResult{spec: s, summary: summary}
			}
		}()
	}
	wg.Wait()
	return results
}

func (e *Engine) updateDispatchStatus(ctx context.Context, dispatchID string, status domain.DispatchStatus, summary, errMsg string) {
	d, getErr := e.store.Get(ctx, dispatchID)
	if getErr != nil {
		e.logger.Warn("Kernel: get dispatch %s for status update: %v", dispatchID, getErr)
		return
	}
	d.Status = status
	d.Summary = summary
	d.Error = errMsg
	d.UpdatedAt = e.now()
	if saveErr := e.store.Save(ctx, d); saveErr != nil {
		e.logger.Warn("Kernel: save dispatch %s status %s: %v", dispatchID, status, saveErr)
	}
}

// buildCycleEntry summarises dispatch results into a cycle history entry.
func (e *Engine) buildCycleEntry(cycleID string, results []dispatchResult) domain.CycleHistoryEntry {
	entry := domain.CycleHistoryEntry{
		CycleID:    cycleID,
		Timestamp:  e.now(),
		Dispatched: len(results),
	}
	var errParts []string
	for _, r := range results {
		if r.err != nil {
			entry.Failed++
			errParts = append(errParts, fmt.Sprintf("%s: %s", r.spec.AgentName, utils.TruncateWithEllipsis(r.err.Error(), 80)))
		} else {
			entry.Succeeded++
		}
	}
	if len(errParts) > 0 {
		entry.ErrorSummary = utils.TruncateWithEllipsis(strings.Join(errParts, "; "), 200)
	}
	return entry
}

// persistCycleHistory appends the entry, trims to MaxCycleHistory, and writes
// to state. This is the simplified cycle_history persistence: a single append-
// and-trim operation with no markdown parsing.
func (e *Engine) persistCycleHistory(ctx context.Context, entry domain.CycleHistoryEntry) error {
	e.mu.Lock()
	e.cycleHistory = append([]domain.CycleHistoryEntry{entry}, e.cycleHistory...)
	max := e.config.maxCycleHistory()
	if len(e.cycleHistory) > max {
		e.cycleHistory = e.cycleHistory[:max]
	}
	snapshot := make([]domain.CycleHistoryEntry, len(e.cycleHistory))
	copy(snapshot, e.cycleHistory)
	e.mu.Unlock()

	if e.stateW == nil {
		return nil
	}
	return e.stateW.WriteCycleHistory(ctx, e.config.KernelID, snapshot)
}

// recordCycleFailure logs the error, records a failed cycle entry, and returns
// the original error. This ensures every RunCycle error path leaves a history
// trace.
func (e *Engine) recordCycleFailure(ctx context.Context, cycleID string, err error) error {
	e.logger.Error("Kernel cycle %s failed: %v", cycleID, err)
	entry := domain.CycleHistoryEntry{
		CycleID:      cycleID,
		Timestamp:    e.now(),
		ErrorSummary: utils.TruncateWithEllipsis(err.Error(), 200),
	}
	if persistErr := e.persistCycleHistory(ctx, entry); persistErr != nil {
		e.logger.Warn("Kernel: persist failure entry: %v", persistErr)
	}
	return err
}
