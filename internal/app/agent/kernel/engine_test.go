package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	domain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

// --- mock store ---

type memStore struct {
	mu         sync.Mutex
	dispatches map[string]domain.Dispatch
	purgeCalls int
}

func newMemStore() *memStore {
	return &memStore{dispatches: make(map[string]domain.Dispatch)}
}

func (m *memStore) Save(_ context.Context, d domain.Dispatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatches[d.DispatchID] = d
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (domain.Dispatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dispatches[id]
	if !ok {
		return domain.Dispatch{}, fmt.Errorf("not found: %s", id)
	}
	return d, nil
}

func (m *memStore) ListRecentByAgent(_ context.Context, _ string, _ int) (map[string][]domain.Dispatch, error) {
	return nil, nil
}

func (m *memStore) RecoverStaleRunning(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *memStore) PurgeTerminalDispatches(_ context.Context, _ string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeCalls++
	return 0, nil
}

// --- mock planner ---

type staticPlanner struct {
	specs []domain.DispatchSpec
	err   error
}

func (p *staticPlanner) Plan(_ context.Context, _ string, _ map[string][]domain.Dispatch) ([]domain.DispatchSpec, error) {
	return p.specs, p.err
}

// --- mock executor ---

type funcExecutor struct {
	fn func(ctx context.Context, spec domain.DispatchSpec) (string, error)
}

func (e *funcExecutor) Execute(ctx context.Context, spec domain.DispatchSpec) (string, error) {
	return e.fn(ctx, spec)
}

// --- mock state ---

type staticStateReader struct{ state string }

func (r *staticStateReader) ReadState(_ context.Context, _ string) (string, error) {
	return r.state, nil
}

type noopStateWriter struct {
	mu      sync.Mutex
	entries []domain.CycleHistoryEntry
}

func (w *noopStateWriter) WriteCycleHistory(_ context.Context, _ string, entries []domain.CycleHistoryEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = entries
	return nil
}

// --- mock notifier ---

type recordingNotifier struct {
	mu     sync.Mutex
	cycles []domain.CycleHistoryEntry
	stale  int
}

func (n *recordingNotifier) NotifyCycleComplete(entry domain.CycleHistoryEntry) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cycles = append(n.cycles, entry)
}
func (n *recordingNotifier) NotifyStaleRecovered(count int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stale += count
}

// --- tests ---

func TestRunCycle_Success(t *testing.T) {
	store := newMemStore()
	notifier := &recordingNotifier{}
	stateW := &noopStateWriter{}
	engine := NewEngine(
		EngineConfig{KernelID: "k1"},
		EngineDeps{
			Store:    store,
			Planner:  &staticPlanner{specs: []domain.DispatchSpec{{AgentName: "agent-a", Prompt: "do work"}}},
			Executor: &funcExecutor{fn: func(_ context.Context, _ domain.DispatchSpec) (string, error) { return "done", nil }},
			Notifier: notifier,
			StateR:   &staticStateReader{state: "current state"},
			StateW:   stateW,
			Logger:   logging.Nop(),
		},
	)

	err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// Verify dispatch was recorded as done.
	store.mu.Lock()
	var doneCount int
	for _, d := range store.dispatches {
		if d.Status == domain.DispatchDone {
			doneCount++
		}
	}
	store.mu.Unlock()
	if doneCount != 1 {
		t.Fatalf("done dispatches = %d, want 1", doneCount)
	}

	// Verify notifier was called.
	notifier.mu.Lock()
	if len(notifier.cycles) != 1 {
		t.Fatalf("notifier cycles = %d, want 1", len(notifier.cycles))
	}
	if notifier.cycles[0].Succeeded != 1 {
		t.Fatalf("succeeded = %d, want 1", notifier.cycles[0].Succeeded)
	}
	notifier.mu.Unlock()

	// Verify cycle history was persisted.
	stateW.mu.Lock()
	if len(stateW.entries) != 1 {
		t.Fatalf("history entries = %d, want 1", len(stateW.entries))
	}
	stateW.mu.Unlock()

	// Verify purge was called.
	store.mu.Lock()
	if store.purgeCalls != 1 {
		t.Fatalf("purge calls = %d, want 1", store.purgeCalls)
	}
	store.mu.Unlock()
}

func TestRunCycle_DispatchFailureRecordedButCycleContinues(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(
		EngineConfig{KernelID: "k1"},
		EngineDeps{
			Store: store,
			Planner: &staticPlanner{specs: []domain.DispatchSpec{
				{AgentName: "agent-ok", Prompt: "ok"},
				{AgentName: "agent-fail", Prompt: "fail"},
			}},
			Executor: &funcExecutor{fn: func(_ context.Context, spec domain.DispatchSpec) (string, error) {
				if spec.AgentName == "agent-fail" {
					return "", errors.New("agent error")
				}
				return "ok", nil
			}},
			StateR: &staticStateReader{state: "state"},
			Logger: logging.Nop(),
		},
	)

	err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle should succeed even with partial dispatch failures: %v", err)
	}

	// Verify both dispatches are recorded.
	store.mu.Lock()
	var done, failed int
	for _, d := range store.dispatches {
		switch d.Status {
		case domain.DispatchDone:
			done++
		case domain.DispatchFailed:
			failed++
		}
	}
	store.mu.Unlock()

	if done != 1 || failed != 1 {
		t.Fatalf("done=%d failed=%d, want done=1 failed=1", done, failed)
	}
}

func TestRunCycle_PlannerError(t *testing.T) {
	stateW := &noopStateWriter{}
	engine := NewEngine(
		EngineConfig{KernelID: "k1"},
		EngineDeps{
			Store:   newMemStore(),
			Planner: &staticPlanner{err: errors.New("planner down")},
			StateR:  &staticStateReader{state: "state"},
			StateW:  stateW,
			Logger:  logging.Nop(),
		},
	)

	err := engine.RunCycle(context.Background())
	if err == nil {
		t.Fatal("expected error from planner failure")
	}
	if !errors.Is(err, err) {
		t.Fatalf("unexpected error type: %v", err)
	}

	// Verify failure was recorded in cycle history.
	stateW.mu.Lock()
	if len(stateW.entries) != 1 {
		t.Fatalf("history entries = %d, want 1", len(stateW.entries))
	}
	if stateW.entries[0].ErrorSummary == "" {
		t.Fatal("expected error summary in cycle history")
	}
	stateW.mu.Unlock()
}

func TestRunCycle_StateReadError(t *testing.T) {
	engine := NewEngine(
		EngineConfig{KernelID: "k1"},
		EngineDeps{
			Store:   newMemStore(),
			Planner: &staticPlanner{},
			StateR: &staticStateReader{},
			Logger:  logging.Nop(),
		},
	)
	// Override state reader to return error.
	engine.stateR = &failingStateReader{err: errors.New("state read failed")}

	err := engine.RunCycle(context.Background())
	if err == nil {
		t.Fatal("expected error from state read failure")
	}
}

type failingStateReader struct{ err error }

func (r *failingStateReader) ReadState(_ context.Context, _ string) (string, error) {
	return "", r.err
}

func TestRunCycle_EmptyPlan(t *testing.T) {
	engine := NewEngine(
		EngineConfig{KernelID: "k1"},
		EngineDeps{
			Store:   newMemStore(),
			Planner: &staticPlanner{specs: nil},
			StateR:  &staticStateReader{state: "state"},
			Logger:  logging.Nop(),
		},
	)

	err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("empty plan should not error: %v", err)
	}
}

func TestRunCycle_CycleHistoryTrimmed(t *testing.T) {
	stateW := &noopStateWriter{}
	engine := NewEngine(
		EngineConfig{KernelID: "k1", MaxCycleHistory: 3},
		EngineDeps{
			Store:   newMemStore(),
			Planner: &staticPlanner{specs: nil},
			StateR:  &staticStateReader{state: "state"},
			StateW:  stateW,
			Logger:  logging.Nop(),
		},
	)

	// Run 5 cycles — history should be trimmed to 3.
	for i := 0; i < 5; i++ {
		if err := engine.RunCycle(context.Background()); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}

	stateW.mu.Lock()
	if len(stateW.entries) != 3 {
		t.Fatalf("history entries = %d, want 3", len(stateW.entries))
	}
	stateW.mu.Unlock()
}

func TestEngineConfig_Defaults(t *testing.T) {
	cfg := EngineConfig{}

	if cfg.LeaseDuration() != 30*time.Minute {
		t.Fatalf("default lease = %v, want 30m", cfg.LeaseDuration())
	}
	if cfg.RetentionPeriod() != 24*time.Hour {
		t.Fatalf("default retention = %v, want 24h", cfg.RetentionPeriod())
	}
	if cfg.maxCycleHistory() != DefaultMaxCycleHistory {
		t.Fatalf("default max history = %d, want %d", cfg.maxCycleHistory(), DefaultMaxCycleHistory)
	}
	if cfg.maxConcurrent() != DefaultMaxConcurrentDispatches {
		t.Fatalf("default max concurrent = %d, want %d", cfg.maxConcurrent(), DefaultMaxConcurrentDispatches)
	}
	if cfg.minBackoff() != DefaultMinRestartBackoff {
		t.Fatalf("default min backoff = %v, want %v", cfg.minBackoff(), DefaultMinRestartBackoff)
	}
	if cfg.maxBackoff() != DefaultMaxRestartBackoff {
		t.Fatalf("default max backoff = %v, want %v", cfg.maxBackoff(), DefaultMaxRestartBackoff)
	}
	if cfg.absenceAlert() != DefaultAbsenceAlertThreshold {
		t.Fatalf("default absence alert = %v, want %v", cfg.absenceAlert(), DefaultAbsenceAlertThreshold)
	}
	if cfg.alertRepeat() != DefaultAlertRepeatInterval {
		t.Fatalf("default alert repeat = %d, want %d", cfg.alertRepeat(), DefaultAlertRepeatInterval)
	}
}

func TestEngineConfig_Overrides(t *testing.T) {
	cfg := EngineConfig{
		LeaseSeconds:            60,
		DispatchRetentionHours:  48,
		MaxCycleHistory:         10,
		MaxConcurrentDispatches: 5,
		MinRestartBackoff:       time.Second,
		MaxRestartBackoff:       time.Minute,
		AbsenceAlertThreshold:   time.Hour,
		AlertRepeatInterval:     5,
	}
	if cfg.LeaseDuration() != time.Minute {
		t.Fatalf("lease = %v, want 1m", cfg.LeaseDuration())
	}
	if cfg.RetentionPeriod() != 48*time.Hour {
		t.Fatalf("retention = %v, want 48h", cfg.RetentionPeriod())
	}
	if cfg.maxCycleHistory() != 10 {
		t.Fatalf("max history = %d, want 10", cfg.maxCycleHistory())
	}
	if cfg.maxConcurrent() != 5 {
		t.Fatalf("max concurrent = %d, want 5", cfg.maxConcurrent())
	}
}
