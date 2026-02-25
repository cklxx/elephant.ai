package integration

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	appconfig "alex/internal/app/agent/config"
	agentcoordinator "alex/internal/app/agent/coordinator"
	agentcost "alex/internal/app/agent/cost"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/infra/llm"
	"alex/internal/infra/session/filestore"
	"alex/internal/infra/storage"
)

// newScalingCoordinator creates a real AgentCoordinator backed by mock LLM
// and a BackgroundTaskManager with unlimited concurrency for scaling tests.
func newScalingCoordinator(t *testing.T) (*agentcoordinator.AgentCoordinator, *react.BackgroundTaskManager) {
	t.Helper()

	llmFactory := llm.NewFactory()
	sessionStore := filestore.New(t.TempDir())

	costStore, err := storage.NewFileCostStore(t.TempDir() + "/costs")
	if err != nil {
		t.Fatalf("failed to create cost store: %v", err)
	}
	costTracker := agentcost.NewCostTracker(costStore)

	coordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		newTestToolRegistry(),
		sessionStore,
		newTestContextManager(),
		nil,
		newTestParser(),
		costTracker,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "test-model",
			MaxIterations: 2,
			Temperature:   0.5,
		},
	)

	bgManager := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			outCtx := agent.WithOutputContext(ctx, &agent.OutputContext{Level: agent.LevelCore})
			return coordinator.ExecuteTask(outCtx, prompt, "", listener)
		},
		SessionID:          "scaling-test",
		MaxConcurrentTasks: 0, // unlimited
	})

	return coordinator, bgManager
}

// newScalingBGManager creates a BackgroundTaskManager with a custom executor.
// Used when we need control over the executor (e.g. panic injection).
func newScalingBGManager(
	t *testing.T,
	executor func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error),
) *react.BackgroundTaskManager {
	t.Helper()
	return react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext:         context.Background(),
		Logger:             agent.NoopLogger{},
		Clock:              agent.SystemClock{},
		ExecuteTask:        executor,
		SessionID:          "scaling-test",
		MaxConcurrentTasks: 0,
	})
}

// goroutineCount returns current goroutine count for leak detection.
func goroutineCount() int {
	return runtime.NumGoroutine()
}

// --- Test 1: Independent Tasks at Scale ---

func TestScalingE2E_IndependentTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	scales := []int{10, 50, 100, 500}

	for _, n := range scales {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			_, bgManager := newScalingCoordinator(t)
			defer bgManager.Shutdown()

			baseGoroutines := goroutineCount()

			// Dispatch all tasks.
			for i := 0; i < n; i++ {
				err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
					TaskID:      fmt.Sprintf("task-%d", i),
					Description: fmt.Sprintf("independent task %d", i),
					Prompt:      fmt.Sprintf("Task %d: do something", i),
					AgentType:   "internal",
				})
				if err != nil {
					t.Fatalf("dispatch task-%d failed: %v", i, err)
				}
			}

			if bgManager.TaskCount() != n {
				t.Fatalf("expected %d tasks, got %d", n, bgManager.TaskCount())
			}

			// Wait for all to complete.
			done := bgManager.AwaitAll(60 * time.Second)
			if !done {
				t.Fatalf("timeout waiting for %d tasks", n)
			}

			// Verify DrainCompletions returns all IDs.
			completed := bgManager.DrainCompletions()
			if len(completed) != n {
				t.Fatalf("DrainCompletions returned %d IDs, expected %d", len(completed), n)
			}

			// Verify all tasks completed.
			results := bgManager.Collect(nil, false, 0)
			if len(results) != n {
				t.Fatalf("expected %d results, got %d", n, len(results))
			}
			for _, r := range results {
				if r.Status != agent.BackgroundTaskStatusCompleted {
					t.Errorf("task %s: expected completed, got %s (err: %s)", r.ID, r.Status, r.Error)
				}
			}

			// Goroutine leak check: allow some margin for runtime GC/timers.
			time.Sleep(100 * time.Millisecond)
			finalGoroutines := goroutineCount()
			leaked := finalGoroutines - baseGoroutines
			// Allow up to 10 extra goroutines for runtime overhead.
			if leaked > 10 {
				t.Errorf("potential goroutine leak: %d goroutines above baseline (base=%d, final=%d)",
					leaked, baseGoroutines, finalGoroutines)
			}

			t.Logf("N=%d: all %d tasks completed, goroutine delta=%d", n, n, leaked)
		})
	}
}

// --- Test 2: Deep Dependency Chain ---

func TestScalingE2E_DeepChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const depth = 50
	_, bgManager := newScalingCoordinator(t)
	defer bgManager.Shutdown()

	start := time.Now()

	// Dispatch chain: task-0 → task-1 → ... → task-49
	for i := 0; i < depth; i++ {
		req := agent.BackgroundDispatchRequest{
			TaskID:      fmt.Sprintf("chain-%d", i),
			Description: fmt.Sprintf("chain step %d", i),
			Prompt:      fmt.Sprintf("Chain step %d", i),
			AgentType:   "internal",
		}
		if i > 0 {
			req.DependsOn = []string{fmt.Sprintf("chain-%d", i-1)}
		}
		if err := bgManager.Dispatch(context.Background(), req); err != nil {
			t.Fatalf("dispatch chain-%d failed: %v", i, err)
		}
	}

	done := bgManager.AwaitAll(120 * time.Second)
	if !done {
		t.Fatal("timeout waiting for deep chain")
	}

	elapsed := time.Since(start)

	// All should be completed.
	results := bgManager.Collect(nil, false, 0)
	completed := 0
	for _, r := range results {
		if r.Status == agent.BackgroundTaskStatusCompleted {
			completed++
		}
	}
	if completed != depth {
		t.Fatalf("expected %d completed, got %d", depth, completed)
	}

	// With channel-based notification, the chain should complete much faster than
	// 50 * 200ms = 10s (old polling). Allow generous 30s for CI.
	t.Logf("Deep chain (%d steps) completed in %v", depth, elapsed)
	if elapsed > 30*time.Second {
		t.Errorf("deep chain too slow: %v (expected < 30s with channel notification)", elapsed)
	}
}

// --- Test 3: Wide Fan-In ---

func TestScalingE2E_WideFanIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const workers = 50
	_, bgManager := newScalingCoordinator(t)
	defer bgManager.Shutdown()

	// Dispatch worker tasks.
	workerIDs := make([]string, workers)
	for i := 0; i < workers; i++ {
		id := fmt.Sprintf("worker-%d", i)
		workerIDs[i] = id
		err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
			TaskID:      id,
			Description: fmt.Sprintf("worker %d", i),
			Prompt:      fmt.Sprintf("Worker %d: compute", i),
			AgentType:   "internal",
		})
		if err != nil {
			t.Fatalf("dispatch worker-%d failed: %v", i, err)
		}
	}

	// Dispatch aggregator that depends on ALL workers.
	err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:         "aggregator",
		Description:    "aggregate all worker results",
		Prompt:         "Aggregate results",
		AgentType:      "internal",
		DependsOn:      workerIDs,
		InheritContext: true,
	})
	if err != nil {
		t.Fatalf("dispatch aggregator failed: %v", err)
	}

	done := bgManager.AwaitAll(60 * time.Second)
	if !done {
		t.Fatal("timeout waiting for fan-in")
	}

	// All workers + aggregator should be completed.
	results := bgManager.Collect(nil, false, 0)
	if len(results) != workers+1 {
		t.Fatalf("expected %d results, got %d", workers+1, len(results))
	}

	aggResult := bgManager.Collect([]string{"aggregator"}, false, 0)
	if len(aggResult) != 1 || aggResult[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Fatalf("aggregator not completed: %v", aggResult)
	}

	t.Logf("Fan-in: %d workers + 1 aggregator completed", workers)
}

// --- Test 4: Panic Recovery at Scale ---

func TestScalingE2E_PanicRecoveryAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const total = 100
	const panicRate = 10 // 10% panic

	var callCount atomic.Int64
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		n := callCount.Add(1)
		if n%panicRate == 0 {
			panic(fmt.Sprintf("simulated panic in task at call %d", n))
		}
		return &agent.TaskResult{Answer: "ok", Iterations: 1, TokensUsed: 10}, nil
	}

	bgManager := newScalingBGManager(t, executor)
	defer bgManager.Shutdown()

	for i := 0; i < total; i++ {
		err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
			TaskID:      fmt.Sprintf("panic-task-%d", i),
			Description: fmt.Sprintf("task %d", i),
			Prompt:      fmt.Sprintf("Do work %d", i),
			AgentType:   "internal",
		})
		if err != nil {
			t.Fatalf("dispatch panic-task-%d failed: %v", i, err)
		}
	}

	done := bgManager.AwaitAll(30 * time.Second)
	if !done {
		t.Fatal("timeout waiting for panic recovery tasks")
	}

	results := bgManager.Collect(nil, false, 0)
	if len(results) != total {
		t.Fatalf("expected %d results, got %d", total, len(results))
	}

	completed := 0
	failed := 0
	for _, r := range results {
		switch r.Status {
		case agent.BackgroundTaskStatusCompleted:
			completed++
		case agent.BackgroundTaskStatusFailed:
			failed++
		default:
			t.Errorf("unexpected status %s for %s", r.Status, r.ID)
		}
	}

	// With 100 tasks, ~10 should panic. Exact count depends on goroutine scheduling.
	if failed == 0 {
		t.Error("expected some panicked tasks")
	}
	if completed == 0 {
		t.Error("expected some completed tasks")
	}
	if completed+failed != total {
		t.Errorf("completed(%d) + failed(%d) != total(%d)", completed, failed, total)
	}

	t.Logf("Panic recovery: %d completed, %d failed (panicked) out of %d", completed, failed, total)
}

// --- Test 5: Cancel Propagation ---

func TestScalingE2E_CancelPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const roots = 10
	const childrenPerRoot = 9

	// Executor that blocks until context cancelled — simulates long-running tasks.
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		select {
		case <-time.After(30 * time.Second):
			return &agent.TaskResult{Answer: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	bgManager := newScalingBGManager(t, executor)
	defer bgManager.Shutdown()

	// Dispatch root tasks.
	for i := 0; i < roots; i++ {
		err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
			TaskID:      fmt.Sprintf("root-%d", i),
			Description: fmt.Sprintf("root %d", i),
			Prompt:      "long running root",
			AgentType:   "internal",
		})
		if err != nil {
			t.Fatalf("dispatch root-%d: %v", i, err)
		}
	}

	// Dispatch children depending on roots.
	for i := 0; i < roots; i++ {
		for j := 0; j < childrenPerRoot; j++ {
			err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
				TaskID:      fmt.Sprintf("child-%d-%d", i, j),
				Description: fmt.Sprintf("child %d-%d", i, j),
				Prompt:      "child task",
				AgentType:   "internal",
				DependsOn:   []string{fmt.Sprintf("root-%d", i)},
			})
			if err != nil {
				t.Fatalf("dispatch child-%d-%d: %v", i, j, err)
			}
		}
	}

	// Let tasks start.
	time.Sleep(100 * time.Millisecond)

	// Cancel all root tasks.
	for i := 0; i < roots; i++ {
		if err := bgManager.CancelTask(context.Background(), fmt.Sprintf("root-%d", i)); err != nil {
			t.Errorf("cancel root-%d: %v", i, err)
		}
	}

	done := bgManager.AwaitAll(15 * time.Second)
	if !done {
		t.Fatal("timeout waiting for cancel propagation")
	}

	// Verify: all roots cancelled, all children failed (dep failed).
	results := bgManager.Collect(nil, false, 0)
	rootCancelled := 0
	childFailed := 0
	for _, r := range results {
		if r.Status == agent.BackgroundTaskStatusCancelled {
			rootCancelled++
		} else if r.Status == agent.BackgroundTaskStatusFailed {
			childFailed++
		}
	}

	if rootCancelled != roots {
		t.Errorf("expected %d cancelled roots, got %d", roots, rootCancelled)
	}
	if childFailed != roots*childrenPerRoot {
		t.Errorf("expected %d failed children, got %d", roots*childrenPerRoot, childFailed)
	}

	t.Logf("Cancel propagation: %d roots cancelled, %d children failed", rootCancelled, childFailed)
}

// --- Test 6: Resource Measurement ---

func TestScalingE2E_ResourceMeasurement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	scales := []int{10, 50, 100, 500}

	for _, n := range scales {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
				return &agent.TaskResult{Answer: "ok", Iterations: 1, TokensUsed: 10}, nil
			}

			bgManager := newScalingBGManager(t, executor)
			defer bgManager.Shutdown()

			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)
			baseGoroutines := goroutineCount()
			start := time.Now()

			// Track peak goroutines during execution.
			var peakGoroutines atomic.Int64
			peakGoroutines.Store(int64(baseGoroutines))

			// Dispatch all tasks.
			for i := 0; i < n; i++ {
				err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
					TaskID:      fmt.Sprintf("res-%d", i),
					Description: fmt.Sprintf("resource task %d", i),
					Prompt:      fmt.Sprintf("Resource task %d", i),
					AgentType:   "internal",
				})
				if err != nil {
					t.Fatalf("dispatch res-%d: %v", i, err)
				}

				// Sample goroutine count periodically.
				if i%10 == 0 {
					current := int64(goroutineCount())
					for {
						old := peakGoroutines.Load()
						if current <= old || peakGoroutines.CompareAndSwap(old, current) {
							break
						}
					}
				}
			}

			done := bgManager.AwaitAll(60 * time.Second)
			if !done {
				t.Fatalf("timeout at N=%d", n)
			}

			wallTime := time.Since(start)

			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			time.Sleep(100 * time.Millisecond)
			finalGoroutines := goroutineCount()
			leaked := finalGoroutines - baseGoroutines

			memDelta := int64(memAfter.TotalAlloc) - int64(memBefore.TotalAlloc)

			t.Logf("N=%-4d | wall=%-8v | peak_goroutines=%-4d | goroutine_leak=%-3d | mem_alloc=%.1fMB",
				n, wallTime.Round(time.Millisecond), peakGoroutines.Load(), leaked, float64(memDelta)/(1024*1024))

			// Goroutine leak check.
			if leaked > 10 {
				t.Errorf("goroutine leak at N=%d: %d goroutines above baseline", n, leaked)
			}
		})
	}
}
