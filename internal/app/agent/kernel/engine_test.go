package kernel

import (
	"context"
	"fmt"
	"sync"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

// memStore is an in-memory Store implementation for testing.
type memStore struct {
	mu          sync.Mutex
	dispatches  []kerneldomain.Dispatch
	schemaReady bool
}

func newMemStore() *memStore {
	return &memStore{}
}

func (s *memStore) EnsureSchema(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemaReady = true
	return nil
}

func (s *memStore) EnqueueDispatches(_ context.Context, kernelID, cycleID string, specs []kerneldomain.DispatchSpec) ([]kerneldomain.Dispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []kerneldomain.Dispatch
	for i, sp := range specs {
		d := kerneldomain.Dispatch{
			DispatchID: fmt.Sprintf("d-%d", len(s.dispatches)+i),
			KernelID:   kernelID,
			CycleID:    cycleID,
			AgentID:    sp.AgentID,
			Prompt:     sp.Prompt,
			Priority:   sp.Priority,
			Status:     kerneldomain.DispatchPending,
			Metadata:   sp.Metadata,
		}
		s.dispatches = append(s.dispatches, d)
		out = append(out, d)
	}
	return out, nil
}

func (s *memStore) ClaimDispatches(_ context.Context, _, _ string, _ int) ([]kerneldomain.Dispatch, error) {
	return nil, nil
}

func (s *memStore) MarkDispatchRunning(_ context.Context, dispatchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.dispatches {
		if s.dispatches[i].DispatchID == dispatchID {
			s.dispatches[i].Status = kerneldomain.DispatchRunning
		}
	}
	return nil
}

func (s *memStore) MarkDispatchDone(_ context.Context, dispatchID, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.dispatches {
		if s.dispatches[i].DispatchID == dispatchID {
			s.dispatches[i].Status = kerneldomain.DispatchDone
			s.dispatches[i].TaskID = taskID
		}
	}
	return nil
}

func (s *memStore) MarkDispatchFailed(_ context.Context, dispatchID, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.dispatches {
		if s.dispatches[i].DispatchID == dispatchID {
			s.dispatches[i].Status = kerneldomain.DispatchFailed
			s.dispatches[i].Error = errMsg
		}
	}
	return nil
}

func (s *memStore) ListActiveDispatches(_ context.Context, kernelID string) ([]kerneldomain.Dispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []kerneldomain.Dispatch
	for _, d := range s.dispatches {
		if d.KernelID == kernelID && (d.Status == kerneldomain.DispatchPending || d.Status == kerneldomain.DispatchRunning) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *memStore) ListRecentByAgent(_ context.Context, kernelID string) (map[string]kerneldomain.Dispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := map[string]kerneldomain.Dispatch{}
	for _, d := range s.dispatches {
		if d.KernelID == kernelID {
			if existing, ok := result[d.AgentID]; !ok || d.CreatedAt.After(existing.CreatedAt) {
				result[d.AgentID] = d
			}
		}
	}
	return result, nil
}

func newTestEngine(t *testing.T, exec Executor) (*Engine, *memStore) {
	t.Helper()
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("test-kernel", []AgentConfig{
		{AgentID: "agent-a", Prompt: "STATE: {STATE}\nDo A.", Priority: 10, Enabled: true},
		{AgentID: "agent-b", Prompt: "STATE: {STATE}\nDo B.", Priority: 5, Enabled: true},
	})
	cfg := KernelConfig{
		KernelID:      "test-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\n## identity\ntest\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))
	return engine, store
}

func TestEngine_RunCycle_EmptyPlan(t *testing.T) {
	exec := &mockExecutor{}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("k", nil) // no agents
	cfg := KernelConfig{
		KernelID: "k",
		Schedule: "*/10 * * * *",
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 0 {
		t.Errorf("expected 0 dispatched, got %d", result.Dispatched)
	}
}

func TestEngine_RunCycle_AllSucceed(t *testing.T) {
	exec := &mockExecutor{}
	engine, store := newTestEngine(t, exec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 2 {
		t.Errorf("expected 2 dispatched, got %d", result.Dispatched)
	}
	if result.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if exec.callCount() != 2 {
		t.Errorf("expected 2 executor calls, got %d", exec.callCount())
	}
	// All dispatches should be done.
	store.mu.Lock()
	for _, d := range store.dispatches {
		if d.Status != kerneldomain.DispatchDone {
			t.Errorf("dispatch %s should be done, got %s", d.DispatchID, d.Status)
		}
	}
	store.mu.Unlock()
}

func TestEngine_RunCycle_PartialFailure(t *testing.T) {
	failExec := &failingExecutor{
		inner:     &mockExecutor{},
		failAgent: "agent-b",
	}
	engine, _ := newTestEngine(t, failExec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CyclePartialSuccess {
		t.Errorf("expected partial_success, got %s", result.Status)
	}
	if result.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if len(result.FailedAgents) != 1 || result.FailedAgents[0] != "agent-b" {
		t.Errorf("expected failed agent-b, got %v", result.FailedAgents)
	}
}

func TestEngine_RunCycle_AllFail(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("always fail")}
	engine, _ := newTestEngine(t, exec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", result.Failed)
	}
}

func TestEngine_RunCycle_SeedState(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	// First cycle should seed the state file.
	_, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// Verify state was seeded.
	content, err := engine.stateFile.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if content != engine.config.SeedState {
		t.Errorf("state not seeded: got %q", content)
	}
}

func TestEngine_RunCycle_RunningAgentNotReDispatched(t *testing.T) {
	exec := &mockExecutor{}
	engine, store := newTestEngine(t, exec)

	// Simulate agent-a as still running.
	store.mu.Lock()
	store.dispatches = append(store.dispatches, kerneldomain.Dispatch{
		DispatchID: "prev-1",
		KernelID:   "test-kernel",
		AgentID:    "agent-a",
		Status:     kerneldomain.DispatchRunning,
	})
	store.mu.Unlock()

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Only agent-b should be dispatched.
	if result.Dispatched != 1 {
		t.Errorf("expected 1 dispatched, got %d", result.Dispatched)
	}
	calls := exec.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].AgentID != "agent-b" {
		t.Errorf("expected agent-b, got %s", calls[0].AgentID)
	}
}

func TestEngine_SetNotifier(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	if engine.notifier != nil {
		t.Fatal("notifier should be nil by default")
	}

	called := false
	engine.SetNotifier(func(_ context.Context, _ *kerneldomain.CycleResult, _ error) {
		called = true
	})
	if engine.notifier == nil {
		t.Fatal("notifier should be set after SetNotifier")
	}

	// Simulate what Run() does: call RunCycle, then invoke notifier on non-empty cycle.
	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Dispatched > 0 {
		engine.notifier(context.Background(), result, nil)
	}
	if !called {
		t.Error("notifier should have been called for non-empty cycle")
	}
}

func TestEngine_NotifierNotCalledOnEmptyCycle(t *testing.T) {
	exec := &mockExecutor{}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("k", nil) // no agents → empty cycle
	cfg := KernelConfig{
		KernelID:  "k",
		Schedule:  "*/10 * * * *",
		SeedState: "# test\n",
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	called := false
	engine.SetNotifier(func(_ context.Context, _ *kerneldomain.CycleResult, _ error) {
		called = true
	})

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Simulate Run()'s logic: only notify when dispatched > 0 or error.
	if err != nil || (result != nil && result.Dispatched > 0) {
		engine.notifier(context.Background(), result, err)
	}
	if called {
		t.Error("notifier should NOT be called for empty cycle")
	}
}

func TestEngine_NotifierCalledOnError(t *testing.T) {
	exec := &mockExecutor{}
	// Use a state file dir that doesn't exist to trigger a read error.
	store := newMemStore()
	sf := NewStateFile("/nonexistent-path-for-test")
	planner := NewStaticPlanner("k", []AgentConfig{
		{AgentID: "a", Prompt: "p", Enabled: true},
	})
	cfg := KernelConfig{
		KernelID: "k",
		Schedule: "*/10 * * * *",
		// No SeedState → state file read will return empty, then Seed will fail on bad dir.
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	var gotErr error
	engine.SetNotifier(func(_ context.Context, _ *kerneldomain.CycleResult, err error) {
		gotErr = err
	})

	result, cycleErr := engine.RunCycle(context.Background())
	// Simulate Run()'s logic.
	if cycleErr != nil || (result != nil && result.Dispatched > 0) {
		engine.notifier(context.Background(), result, cycleErr)
	}
	if cycleErr == nil {
		t.Fatal("expected RunCycle error on bad state dir")
	}
	if gotErr == nil {
		t.Error("notifier should have received the error")
	}
}

func TestEngine_StopIdempotent(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	// Should not panic when called multiple times.
	engine.Stop()
	engine.Stop()
}

// failingExecutor wraps another executor and fails for a specific agent.
type failingExecutor struct {
	inner     Executor
	failAgent string
}

func (f *failingExecutor) Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (string, error) {
	if agentID == f.failAgent {
		return "", fmt.Errorf("simulated failure for %s", agentID)
	}
	return f.inner.Execute(ctx, agentID, prompt, meta)
}
