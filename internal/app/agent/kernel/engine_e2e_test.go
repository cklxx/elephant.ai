package kernel

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
	kernelinfra "alex/internal/infra/kernel"
	"alex/internal/shared/logging"
	"alex/internal/shared/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

// setupE2EEngine creates a real Engine backed by PostgresStore for E2E testing.
// It skips if ALEX_TEST_DATABASE_URL is not set.
func setupE2EEngine(t *testing.T, kernelID string, agents []AgentConfig, exec Executor) (*Engine, kerneldomain.Store) {
	t.Helper()

	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := kernelinfra.NewPostgresStore(pool, 5*time.Minute)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	sf := NewStateFile(t.TempDir())
	planner := NewStaticPlanner(kernelID, agents)
	cfg := KernelConfig{
		KernelID:      kernelID,
		Schedule:      "*/10 * * * *",
		SeedState:     "# Kernel E2E Test State\n## identity\ne2e test\n",
		MaxConcurrent: 3,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("e2e"))
	return engine, store
}

// TestE2E_RunCycle_SingleAgentSuccess exercises the full Engine→PostgresStore→Executor
// pipeline with a single agent that succeeds.
func TestE2E_RunCycle_SingleAgentSuccess(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"single agent completed successfully"}}
	agents := []AgentConfig{
		{AgentID: "e2e-agent", Prompt: "STATE: {STATE}\nPerform E2E task.", Priority: 5, Enabled: true},
	}
	engine, store := setupE2EEngine(t, "e2e-single", agents, exec)

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 1 {
		t.Errorf("expected 1 dispatched, got %d", result.Dispatched)
	}
	if result.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", result.Succeeded)
	}

	// Verify the dispatch row in Postgres is marked done.
	recent, err := store.ListRecentByAgent(context.Background(), "e2e-single")
	if err != nil {
		t.Fatalf("ListRecentByAgent: %v", err)
	}
	d, ok := recent["e2e-agent"]
	if !ok {
		t.Fatal("expected dispatch for e2e-agent in Postgres")
	}
	if d.Status != kerneldomain.DispatchDone {
		t.Errorf("expected Postgres status=done, got %s", d.Status)
	}
	if d.TaskID == "" {
		t.Error("expected non-empty task_id in Postgres")
	}
}

// TestE2E_RunCycle_TwoAgentsPartialFailure exercises partial failure: one agent
// succeeds, one fails, and Postgres records the correct state for each.
func TestE2E_RunCycle_TwoAgentsPartialFailure(t *testing.T) {
	failExec := &failingExecutor{
		inner:     &mockExecutor{summaries: []string{"agent-ok completed"}},
		failAgent: "e2e-fail",
	}
	agents := []AgentConfig{
		{AgentID: "e2e-ok", Prompt: "STATE: {STATE}\nSucceed.", Priority: 10, Enabled: true},
		{AgentID: "e2e-fail", Prompt: "STATE: {STATE}\nFail.", Priority: 5, Enabled: true},
	}
	engine, store := setupE2EEngine(t, "e2e-partial", agents, failExec)

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

	// Verify Postgres states.
	recent, err := store.ListRecentByAgent(context.Background(), "e2e-partial")
	if err != nil {
		t.Fatalf("ListRecentByAgent: %v", err)
	}
	if dOK, ok := recent["e2e-ok"]; !ok {
		t.Error("missing dispatch for e2e-ok")
	} else if dOK.Status != kerneldomain.DispatchDone {
		t.Errorf("e2e-ok: expected done, got %s", dOK.Status)
	}
	if dFail, ok := recent["e2e-fail"]; !ok {
		t.Error("missing dispatch for e2e-fail")
	} else if dFail.Status != kerneldomain.DispatchFailed {
		t.Errorf("e2e-fail: expected failed, got %s", dFail.Status)
	}
}

// TestE2E_RunCycle_StaleRecovery forces a stale running dispatch and verifies
// the next cycle recovers it via RecoverStaleRunning.
func TestE2E_RunCycle_StaleRecovery(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"recovered agent done"}}
	agents := []AgentConfig{
		{AgentID: "e2e-stale-agent", Prompt: "STATE: {STATE}\nRecover.", Priority: 5, Enabled: true},
	}

	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	// Use a very short lease (1 second) so we can expire it immediately.
	store := kernelinfra.NewPostgresStore(pool, 1*time.Second)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	kernelID := "e2e-stale"
	sf := NewStateFile(t.TempDir())
	planner := NewStaticPlanner(kernelID, agents)
	cfg := KernelConfig{
		KernelID:      kernelID,
		Schedule:      "*/10 * * * *",
		SeedState:     "# stale test\n",
		MaxConcurrent: 1,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("e2e-stale"))

	// Manually insert a "running" dispatch with an expired lease.
	insertStaleDispatch(t, pool, kernelID, "e2e-stale-agent")

	// Wait for lease to expire (lease_duration=1s).
	time.Sleep(1500 * time.Millisecond)

	// Verify the stale dispatch exists as running.
	active, err := store.ListActiveDispatches(context.Background(), kernelID)
	if err != nil {
		t.Fatalf("ListActiveDispatches: %v", err)
	}
	foundRunning := false
	for _, d := range active {
		if d.Status == kerneldomain.DispatchRunning {
			foundRunning = true
		}
	}
	if !foundRunning {
		t.Fatal("expected to find stale running dispatch before recovery")
	}

	// Run a cycle — should recover the stale dispatch then dispatch fresh.
	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// The stale dispatch should have been recovered (marked failed),
	// and a new dispatch should succeed.
	if result.Dispatched != 1 {
		t.Errorf("expected 1 new dispatch, got %d", result.Dispatched)
	}
	if result.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", result.Succeeded)
	}

	// Verify no active (running/pending) dispatches remain.
	active, err = store.ListActiveDispatches(context.Background(), kernelID)
	if err != nil {
		t.Fatalf("ListActiveDispatches after recovery: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active dispatches after recovery, got %d", len(active))
	}
}

// insertStaleDispatch inserts a running dispatch with a now-expired lease.
func insertStaleDispatch(t *testing.T, pool *pgxpool.Pool, kernelID, agentID string) {
	t.Helper()
	ctx := context.Background()
	expiredLease := time.Now().UTC().Add(-10 * time.Second)
	_, err := pool.Exec(ctx,
		`INSERT INTO kernel_dispatch_tasks
		 (dispatch_id, kernel_id, cycle_id, agent_id, prompt, priority, status, lease_until, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		fmt.Sprintf("stale-%d", time.Now().UnixNano()),
		kernelID,
		"stale-cycle",
		agentID,
		"stale prompt",
		5,
		string(kerneldomain.DispatchRunning),
		expiredLease,
		time.Now().UTC(),
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert stale dispatch: %v", err)
	}
}

// TestE2E_RunCycle_ConcurrentDispatches verifies that 3 agents with MaxConcurrent=2
// all complete without deadlock or data races.
func TestE2E_RunCycle_ConcurrentDispatches(t *testing.T) {
	var concurrentCount atomic.Int32
	var maxConcurrent atomic.Int32

	// slowExec tracks concurrent execution to verify semaphore.
	slowExec := &concurrencyTrackingExecutor{
		inner:       &mockExecutor{summaries: []string{"a-done", "b-done", "c-done"}},
		concurrent:  &concurrentCount,
		maxObserved: &maxConcurrent,
		delay:       50 * time.Millisecond,
	}

	agents := []AgentConfig{
		{AgentID: "e2e-conc-a", Prompt: "Do A. STATE={STATE}", Priority: 10, Enabled: true},
		{AgentID: "e2e-conc-b", Prompt: "Do B. STATE={STATE}", Priority: 5, Enabled: true},
		{AgentID: "e2e-conc-c", Prompt: "Do C. STATE={STATE}", Priority: 3, Enabled: true},
	}

	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := kernelinfra.NewPostgresStore(pool, 5*time.Minute)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	sf := NewStateFile(t.TempDir())
	planner := NewStaticPlanner("e2e-conc", agents)
	cfg := KernelConfig{
		KernelID:      "e2e-conc",
		Schedule:      "*/10 * * * *",
		SeedState:     "# concurrent test\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, slowExec, logging.NewComponentLogger("e2e-conc"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Dispatched != 3 {
		t.Errorf("expected 3 dispatched, got %d", result.Dispatched)
	}
	if result.Succeeded != 3 {
		t.Errorf("expected 3 succeeded, got %d", result.Succeeded)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}

	// Verify concurrency was bounded by MaxConcurrent=2.
	observed := maxConcurrent.Load()
	if observed > 2 {
		t.Errorf("expected max 2 concurrent, observed %d", observed)
	}
	if observed < 2 {
		t.Logf("note: observed max concurrency=%d (may be <2 under CI load)", observed)
	}

	// All dispatches should be done in Postgres.
	recent, err := store.ListRecentByAgent(context.Background(), "e2e-conc")
	if err != nil {
		t.Fatalf("ListRecentByAgent: %v", err)
	}
	for _, agentID := range []string{"e2e-conc-a", "e2e-conc-b", "e2e-conc-c"} {
		d, ok := recent[agentID]
		if !ok {
			t.Errorf("missing dispatch for %s in Postgres", agentID)
			continue
		}
		if d.Status != kerneldomain.DispatchDone {
			t.Errorf("%s: expected done, got %s", agentID, d.Status)
		}
	}
}

// concurrencyTrackingExecutor wraps an executor and tracks max concurrent invocations.
type concurrencyTrackingExecutor struct {
	inner       Executor
	concurrent  *atomic.Int32
	maxObserved *atomic.Int32
	delay       time.Duration
}

func (e *concurrencyTrackingExecutor) Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error) {
	cur := e.concurrent.Add(1)
	defer e.concurrent.Add(-1)

	// Update max observed.
	for {
		old := e.maxObserved.Load()
		if cur <= old || e.maxObserved.CompareAndSwap(old, cur) {
			break
		}
	}

	time.Sleep(e.delay)
	return e.inner.Execute(ctx, agentID, prompt, meta)
}

// TestE2E_Notifier_Integration verifies the notifier receives correct CycleResult
// through the real store path.
func TestE2E_Notifier_Integration(t *testing.T) {
	exec := &mockExecutor{summaries: []string{"notify-a", "notify-b"}}
	agents := []AgentConfig{
		{AgentID: "e2e-notify-a", Prompt: "STATE: {STATE}\nNotify A.", Priority: 10, Enabled: true},
		{AgentID: "e2e-notify-b", Prompt: "STATE: {STATE}\nNotify B.", Priority: 5, Enabled: true},
	}
	engine, _ := setupE2EEngine(t, "e2e-notify", agents, exec)

	var notifiedResult *kerneldomain.CycleResult
	var notifiedErr error
	engine.SetNotifier(func(_ context.Context, result *kerneldomain.CycleResult, err error) {
		notifiedResult = result
		notifiedErr = err
	})

	// RunCycle + simulate Run()'s notification logic.
	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if err != nil || (result != nil && result.Dispatched > 0) {
		engine.notifier(context.Background(), result, err)
	}

	if notifiedErr != nil {
		t.Errorf("expected no error in notification, got: %v", notifiedErr)
	}
	if notifiedResult == nil {
		t.Fatal("expected notifier to be called with result")
	}
	if notifiedResult.KernelID != "e2e-notify" {
		t.Errorf("expected kernelID=e2e-notify, got %s", notifiedResult.KernelID)
	}
	if notifiedResult.Dispatched != 2 {
		t.Errorf("expected 2 dispatched in notification, got %d", notifiedResult.Dispatched)
	}
	if notifiedResult.Succeeded != 2 {
		t.Errorf("expected 2 succeeded in notification, got %d", notifiedResult.Succeeded)
	}
	if len(notifiedResult.AgentSummary) != 2 {
		t.Errorf("expected 2 agent summaries, got %d", len(notifiedResult.AgentSummary))
	}

	// Verify notification content matches FormatCycleNotification output.
	formatted := FormatCycleNotification("e2e-notify", notifiedResult, nil)
	if formatted == "" {
		t.Error("expected non-empty formatted notification")
	}
}
