package kernel

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// --- Layer 1: Panic recovery + auto-restart ---

func TestEngine_Run_RecoversPanicAndRestarts(t *testing.T) {
	var planCount atomic.Int64
	pp := &panicPlanner{callCount: &planCount, panicUntil: 1}
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}

	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	cfg := KernelConfig{
		KernelID:      "test-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\n## identity\ntest\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, pp, exec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(ctx)
	}()

	// Trigger immediate cycle (planner will panic).
	engine.TriggerNow()

	// Wait for the loop to restart and the planner to be called again.
	deadline := time.After(12 * time.Second)
	for {
		if planCount.Load() >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for planner retry; plan_count=%d, restarts=%d",
				planCount.Load(), engine.loopRestarts.Load())
		case <-time.After(100 * time.Millisecond):
		}
		// Keep triggering so the restarted loop picks up work quickly.
		engine.TriggerNow()
	}

	if engine.loopRestarts.Load() < 1 {
		t.Errorf("expected at least 1 loop restart, got %d", engine.loopRestarts.Load())
	}

	cancel()
	<-done
}

func TestEngine_Run_CleanExitOnStop(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(context.Background())
	}()

	// Stop should cause clean exit with no restarts.
	engine.Stop()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit after Stop")
	}

	if engine.loopRestarts.Load() != 0 {
		t.Errorf("expected 0 loop restarts on clean stop, got %d", engine.loopRestarts.Load())
	}
}

// --- Layer 2: Health probe ---

func TestEngine_HealthStatus_StartingState(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	h := engine.HealthStatus()
	if !h.Ready {
		t.Errorf("expected ready=true during startup, got %v (reason=%s)", h.Ready, h.Reason)
	}
	if h.Reason != "starting" {
		t.Errorf("expected reason=starting, got %q", h.Reason)
	}
	if !h.LastCycleAt.IsZero() {
		t.Errorf("expected zero LastCycleAt before any cycle")
	}
}

func TestEngine_HealthStatus_ReadyAfterCycle(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"done a", "done b"}}
	engine, _ := newTestEngine(t, exec)

	if _, err := engine.RunCycle(context.Background()); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// Simulate what runCycleWithLogging does.
	now := time.Now().UnixNano()
	engine.lastCycleAt.Store(now)
	engine.lastSuccessAt.Store(now)

	h := engine.HealthStatus()
	if !h.Ready {
		t.Errorf("expected ready=true after cycle, got %v (reason=%s)", h.Ready, h.Reason)
	}
	if h.Reason != "ok" {
		t.Errorf("expected reason=ok, got %q", h.Reason)
	}
	if h.LastCycleAt.IsZero() {
		t.Error("expected non-zero LastCycleAt after cycle")
	}
}

func TestEngine_HealthStatus_NotReadyWhenStale(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	// Simulate a cycle that completed long ago.
	engine.lastCycleAt.Store(time.Now().Add(-3 * time.Hour).UnixNano())

	h := engine.HealthStatus()
	if h.Ready {
		t.Errorf("expected ready=false when cycle is stale, got %v (reason=%s)", h.Ready, h.Reason)
	}
	if h.Reason == "" {
		t.Error("expected non-empty reason when stale")
	}
}

// --- Layer 3: Absence alert ---

func TestEngine_AbsenceAlert_FiresAfterThreshold(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	var alertFired atomic.Bool
	engine.SetNotifier(func(_ context.Context, result *kerneldomain.CycleResult, err error) {
		if result == nil && err != nil {
			alertFired.Store(true)
		}
	})

	// Simulate: last success was 3 hours ago, with one consecutive failure.
	engine.lastSuccessAt.Store(time.Now().Add(-3 * time.Hour).UnixNano())
	engine.consecutiveFailures.Store(1)

	engine.checkAbsenceAlert(context.Background())

	if !alertFired.Load() {
		t.Error("expected absence alert to fire when last success exceeds threshold")
	}
}

func TestEngine_AbsenceAlert_DoesNotFireWhenRecent(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	var alertFired atomic.Bool
	engine.SetNotifier(func(_ context.Context, result *kerneldomain.CycleResult, err error) {
		if result == nil && err != nil {
			alertFired.Store(true)
		}
	})

	// Last success was recent.
	engine.lastSuccessAt.Store(time.Now().Add(-10 * time.Minute).UnixNano())
	engine.consecutiveFailures.Store(1)

	engine.checkAbsenceAlert(context.Background())

	if alertFired.Load() {
		t.Error("absence alert should NOT fire when last success is recent")
	}
}

func TestEngine_AbsenceAlert_DoesNotSpamOnEveryFailure(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)

	var alertCount atomic.Int64
	engine.SetNotifier(func(_ context.Context, result *kerneldomain.CycleResult, err error) {
		if result == nil && err != nil {
			alertCount.Add(1)
		}
	})

	engine.lastSuccessAt.Store(time.Now().Add(-3 * time.Hour).UnixNano())

	// Fire for consecutive failure counts 1..25.
	// Should only fire for 1, 10, and 20 (first crossing + every 10th).
	for i := int64(1); i <= 25; i++ {
		engine.consecutiveFailures.Store(i)
		engine.checkAbsenceAlert(context.Background())
	}

	count := alertCount.Load()
	if count > 5 {
		t.Errorf("expected <= 5 alerts for 25 failures, got %d (too many = spam)", count)
	}
	if count < 1 {
		t.Error("expected at least 1 alert")
	}
}

// --- Layer 2: Consecutive failure tracking via runCycleWithLogging ---

func TestEngine_RunCycleWithLogging_TracksConsecutiveFailures(t *testing.T) {
	exec := &mockExecutor{err: nil, summaries: []string{"done a", "done b"}}
	engine, _ := newTestEngine(t, exec)

	// First cycle: success.
	engine.runCycleWithLogging(context.Background())
	if engine.consecutiveFailures.Load() != 0 {
		t.Errorf("expected 0 consecutive failures after success, got %d", engine.consecutiveFailures.Load())
	}
	if engine.lastSuccessAt.Load() == 0 {
		t.Error("expected lastSuccessAt to be set after success")
	}
	if engine.lastCycleAt.Load() == 0 {
		t.Error("expected lastCycleAt to be set after any cycle")
	}
}

// --- Test helpers ---

// panicPlanner panics on the first N Plan() calls, then returns normal specs.
type panicPlanner struct {
	callCount  *atomic.Int64
	panicUntil int64
}

func (p *panicPlanner) Plan(_ context.Context, _ string, _ map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
	n := p.callCount.Add(1)
	if n <= p.panicUntil {
		panic("simulated planner panic")
	}
	return []kerneldomain.DispatchSpec{
		{AgentID: "agent-a", Prompt: "do A", Priority: 10},
	}, nil
}
