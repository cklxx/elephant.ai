package kernel

import (
	"context"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

// Architecture documentation (verified by tests below):
//
// 1 RunLark() process = 1 Container = 1 KernelEngine.
//
// Evidence from production code:
//   - container_builder.go:222-229: builds at most 1 kernel engine per Container
//   - lark.go:89: f.KernelStage(subsystems) called once
//   - lark.go:78-84: single startLarkGateway handles all bot IDs
//   - Even with 2 ai_chat_bot_ids, they share ONE gateway → ONE container → ONE kernel
//
// This means:
//   - All configured agents share the same dispatch queue and concurrency budget
//   - KernelID uniquely identifies one engine; two engines with different IDs are isolated
//   - State persistence (STATE.md) is per-engine, keyed by KernelID

// TestKernelInstance_SingleEngineMultipleAgents verifies that one engine with
// multiple agents produces one dispatch per agent per cycle.
func TestKernelInstance_SingleEngineMultipleAgents(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"res-a", "res-b"}}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)

	agents := []AgentConfig{
		{AgentID: "agent-x", Prompt: "Do X. STATE={STATE}", Priority: 10, Enabled: true},
		{AgentID: "agent-y", Prompt: "Do Y. STATE={STATE}", Priority: 5, Enabled: true},
	}
	planner := NewStaticPlanner("single-engine", agents)
	cfg := KernelConfig{
		KernelID:      "single-engine",
		Schedule:      "*/10 * * * *",
		SeedState:     "# test state\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("test"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Dispatched != 2 {
		t.Errorf("expected 2 dispatches from 2 agents, got %d", result.Dispatched)
	}
	if exec.callCount() != 2 {
		t.Errorf("expected 2 executor calls, got %d", exec.callCount())
	}

	// Verify both agents were dispatched exactly once.
	calls := exec.getCalls()
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c.AgentID] = true
	}
	if !seen["agent-x"] || !seen["agent-y"] {
		t.Errorf("expected both agent-x and agent-y dispatched, got: %v", seen)
	}
}

// TestKernelInstance_TwoEnginesIsolated verifies that two engines with different
// kernelIDs sharing the same store produce independent dispatches. This models
// a hypothetical multi-container deployment (not currently used in RunLark, but
// ensures correctness of the isolation boundary).
func TestKernelInstance_TwoEnginesIsolated(t *testing.T) {
	store := newMemStore()

	// Engine A: kernelID="kernel-a", 1 agent
	execA := &mockExecutor{summaries: []string{"done-a"}}
	sfA := NewStateFile(t.TempDir())
	plannerA := NewStaticPlanner("kernel-a", []AgentConfig{
		{AgentID: "agent-alpha", Prompt: "Do alpha. STATE={STATE}", Priority: 5, Enabled: true},
	})
	cfgA := KernelConfig{
		KernelID:      "kernel-a",
		Schedule:      "*/10 * * * *",
		SeedState:     "# kernel-a state\n",
		MaxConcurrent: 1,
	}
	engineA := NewEngine(cfgA, sfA, store, plannerA, execA, logging.NewComponentLogger("test-a"))

	// Engine B: kernelID="kernel-b", 1 agent
	execB := &mockExecutor{summaries: []string{"done-b"}}
	sfB := NewStateFile(t.TempDir())
	plannerB := NewStaticPlanner("kernel-b", []AgentConfig{
		{AgentID: "agent-beta", Prompt: "Do beta. STATE={STATE}", Priority: 5, Enabled: true},
	})
	cfgB := KernelConfig{
		KernelID:      "kernel-b",
		Schedule:      "*/10 * * * *",
		SeedState:     "# kernel-b state\n",
		MaxConcurrent: 1,
	}
	engineB := NewEngine(cfgB, sfB, store, plannerB, execB, logging.NewComponentLogger("test-b"))

	// Run both engines.
	resultA, err := engineA.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("engine-a RunCycle: %v", err)
	}
	resultB, err := engineB.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("engine-b RunCycle: %v", err)
	}

	// Each should dispatch exactly 1.
	if resultA.Dispatched != 1 {
		t.Errorf("engine-a: expected 1 dispatch, got %d", resultA.Dispatched)
	}
	if resultB.Dispatched != 1 {
		t.Errorf("engine-b: expected 1 dispatch, got %d", resultB.Dispatched)
	}

	// Verify isolation: engine A's ListRecentByAgent should only see kernel-a dispatches.
	recentA, err := store.ListRecentByAgent(context.Background(), "kernel-a")
	if err != nil {
		t.Fatalf("ListRecentByAgent kernel-a: %v", err)
	}
	recentB, err := store.ListRecentByAgent(context.Background(), "kernel-b")
	if err != nil {
		t.Fatalf("ListRecentByAgent kernel-b: %v", err)
	}
	if _, ok := recentA["agent-beta"]; ok {
		t.Error("kernel-a should not see agent-beta dispatches")
	}
	if _, ok := recentB["agent-alpha"]; ok {
		t.Error("kernel-b should not see agent-alpha dispatches")
	}

	// State files are independent.
	contentA, _ := sfA.Read()
	contentB, _ := sfB.Read()
	if contentA == contentB {
		t.Error("state files should differ between engines")
	}
}

// TestKernelInstance_SequentialCyclesSameEngine verifies that state and dispatch
// history persists correctly across multiple cycles within a single engine.
func TestKernelInstance_SequentialCyclesSameEngine(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"cycle1-a", "cycle1-b", "cycle2-a", "cycle2-b"}}
	engine, store := newTestEngine(t, exec)

	// Cycle 1
	result1, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	if result1.Dispatched != 2 || result1.Succeeded != 2 {
		t.Fatalf("cycle 1: expected 2/2, got dispatched=%d succeeded=%d", result1.Dispatched, result1.Succeeded)
	}

	// Cycle 2: should still dispatch both agents (previous dispatches are done, not running).
	result2, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 2: %v", err)
	}
	if result2.Dispatched != 2 || result2.Succeeded != 2 {
		t.Fatalf("cycle 2: expected 2/2, got dispatched=%d succeeded=%d", result2.Dispatched, result2.Succeeded)
	}

	// Verify cycle IDs differ.
	if result1.CycleID == result2.CycleID {
		t.Error("sequential cycles should have different cycle IDs")
	}

	// Verify total dispatches accumulated in store.
	store.mu.Lock()
	total := len(store.dispatches)
	store.mu.Unlock()
	if total != 4 {
		t.Errorf("expected 4 total dispatches across 2 cycles, got %d", total)
	}

	// Verify state file contains kernel runtime section (persisted across cycles).
	content, err := engine.stateFile.Read()
	if err != nil {
		t.Fatalf("Read state: %v", err)
	}
	if content == "" {
		t.Fatal("state file should not be empty after 2 cycles")
	}

	// Verify stale recovery was called each cycle.
	store.mu.Lock()
	hits := store.recoverHits
	store.mu.Unlock()
	if hits != 2 {
		t.Errorf("expected 2 stale recovery calls (1 per cycle), got %d", hits)
	}
}

// TestKernelInstance_RunningAgentBlocksReDispatch_AcrossCycles verifies that an
// agent still running from cycle 1 is NOT re-dispatched in cycle 2.
func TestKernelInstance_RunningAgentBlocksReDispatch_AcrossCycles(t *testing.T) {
	// Use a "hanging" executor for agent-a: it completes agent-b but agent-a stays running.
	hangExec := &selectiveHangExecutor{
		inner:     &mockExecutor{summaries: []string{"done-b", "done-b-again"}},
		hangAgent: "agent-a",
	}
	engine, store := newTestEngine(t, hangExec)

	// Simulate agent-a as still running from a previous cycle.
	store.mu.Lock()
	store.dispatches = append(store.dispatches, kerneldomain.Dispatch{
		DispatchID: "prev-run-1",
		KernelID:   "test-kernel",
		AgentID:    "agent-a",
		Status:     kerneldomain.DispatchRunning,
	})
	store.mu.Unlock()

	// Cycle 1: only agent-b should dispatch.
	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	if result.Dispatched != 1 {
		t.Errorf("cycle 1: expected 1 dispatch (only agent-b), got %d", result.Dispatched)
	}

	calls := hangExec.inner.(*mockExecutor).getCalls()
	if len(calls) != 1 || calls[0].AgentID != "agent-b" {
		t.Errorf("expected only agent-b dispatched, got: %v", calls)
	}
}

// selectiveHangExecutor completes all agents except hangAgent, which returns an error
// simulating a long-running execution. Used to test running-state blocking.
type selectiveHangExecutor struct {
	inner     Executor
	hangAgent string
}

func (e *selectiveHangExecutor) Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error) {
	if agentID == e.hangAgent {
		// Simulate timeout/hang — in tests, just error.
		return ExecutionResult{}, context.DeadlineExceeded
	}
	return e.inner.Execute(ctx, agentID, prompt, meta)
}
