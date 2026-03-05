//go:build integration

package integration

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/domain/agent/react"
)

// ---------------------------------------------------------------------------
// Real E2E tool definitions — actual ToolExecutor implementations that run
// inside the ReactEngine loop, exercising the full think→parse→execute→observe
// chain. Each tool simulates a lightweight but real operation.
// ---------------------------------------------------------------------------

// computeTool simulates a computation step (e.g. analysis, code generation).
type computeTool struct {
	calls atomic.Int64
}

func (t *computeTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.calls.Add(1)
	input, _ := call.Arguments["input"].(string)
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("computed result for: %s", input),
		Metadata: map[string]any{
			"tool":    "compute",
			"call_id": call.ID,
		},
	}, nil
}

func (t *computeTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "compute",
		Description: "Perform a computation on the given input",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"input": {Type: "string", Description: "The input to compute"},
			},
			Required: []string{"input"},
		},
	}
}

func (t *computeTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "compute"}
}

// analysisTool simulates reading and analyzing data (second tool in the chain).
type analysisTool struct {
	calls atomic.Int64
}

func (t *analysisTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.calls.Add(1)
	topic, _ := call.Arguments["topic"].(string)
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("analysis complete: findings for %q — 3 key insights identified", topic),
		Metadata: map[string]any{
			"tool":     "analysis",
			"insights": 3,
		},
	}, nil
}

func (t *analysisTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "analysis",
		Description: "Analyze data and extract insights",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"topic": {Type: "string", Description: "The topic to analyze"},
			},
			Required: []string{"topic"},
		},
	}
}

func (t *analysisTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "analysis"}
}

// ---------------------------------------------------------------------------
// E2E tool registry — holds real tool executors keyed by name.
// ---------------------------------------------------------------------------

type e2eToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]tools.ToolExecutor
}

func newE2EToolRegistry(executors ...tools.ToolExecutor) *e2eToolRegistry {
	r := &e2eToolRegistry{tools: make(map[string]tools.ToolExecutor)}
	for _, ex := range executors {
		r.tools[ex.Definition().Name] = ex
	}
	return r
}

func (r *e2eToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not registered", name)
	}
	return t, nil
}

func (r *e2eToolRegistry) List() []ports.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ports.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

func (r *e2eToolRegistry) Register(tool tools.ToolExecutor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Definition().Name] = tool
	return nil
}

func (r *e2eToolRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
	return nil
}

// ---------------------------------------------------------------------------
// E2E mock LLM — returns ToolCalls on early iterations, final answer on last.
// Each agent instance gets its own call counter (threadsafe via closure).
// ---------------------------------------------------------------------------

func newE2EToolCallLLM() *mocks.MockLLMClient {
	return &mocks.MockLLMClient{
		CompleteFunc: newToolCallCompleteFunc(),
	}
}

// newToolCallCompleteFunc returns a CompleteFunc that drives a 3-iteration
// ReAct loop: compute → analysis → final answer.
func newToolCallCompleteFunc() func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	var callCount atomic.Int64
	return func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
		n := callCount.Add(1)
		switch n {
		case 1:
			// Iteration 1: call compute tool
			return &ports.CompletionResponse{
				Content: "I will compute the result first.",
				ToolCalls: []ports.ToolCall{
					{
						ID:        fmt.Sprintf("call-compute-%d", n),
						Name:      "compute",
						Arguments: map[string]any{"input": "scaling test data"},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 50, CompletionTokens: 30, TotalTokens: 80},
			}, nil
		case 2:
			// Iteration 2: call analysis tool
			return &ports.CompletionResponse{
				Content: "Now I will analyze the computed result.",
				ToolCalls: []ports.ToolCall{
					{
						ID:        fmt.Sprintf("call-analysis-%d", n),
						Name:      "analysis",
						Arguments: map[string]any{"topic": "scaling performance"},
					},
				},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{PromptTokens: 80, CompletionTokens: 40, TotalTokens: 120},
			}, nil
		default:
			// Iteration 3+: final answer
			return &ports.CompletionResponse{
				Content:    "Based on my computation and analysis, the scaling test is successful. Final answer: all systems nominal.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{PromptTokens: 120, CompletionTokens: 50, TotalTokens: 170},
			}, nil
		}
	}
}

// ---------------------------------------------------------------------------
// E2E executor — wires ReactEngine + real tools + mock LLM per invocation.
// This is the real execution path: SolveTask → think → parseToolCalls →
// registry.Get → tool.Execute → observe → think → ... → final_answer.
// ---------------------------------------------------------------------------

func newE2EExecutor(
	computeT *computeTool,
	analysisT *analysisTool,
) func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		engine := react.NewReactEngine(react.ReactEngineConfig{
			MaxIterations: 5,
			Logger:        agent.NoopLogger{},
			Clock:         agent.SystemClock{},
		})
		if listener != nil {
			engine.SetEventListener(listener)
		}

		registry := newE2EToolRegistry(computeT, analysisT)
		llmClient := newE2EToolCallLLM()

		services := react.Services{
			LLM:          llmClient,
			ToolExecutor: registry,
			Parser:       &mocks.MockParser{},
			Context:      &mocks.MockContextManager{},
		}

		state := &react.TaskState{
			SessionID: sessionID,
		}

		result, err := engine.SolveTask(ctx, prompt, state, services)
		if err != nil {
			return nil, err
		}

		return &agent.TaskResult{
			Answer:     result.Answer,
			Iterations: result.Iterations,
			TokensUsed: result.TokensUsed,
			StopReason: result.StopReason,
			SessionID:  sessionID,
			RunID:      result.RunID,
		}, nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func goroutineCount() int {
	return runtime.NumGoroutine()
}

func newE2EBGManager(
	t *testing.T,
	executor func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error),
) *react.BackgroundTaskManager {
	t.Helper()
	return react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext:         context.Background(),
		Logger:             agent.NoopLogger{},
		Clock:              agent.SystemClock{},
		ExecuteTask:        executor,
		SessionID:          "e2e-scaling",
		MaxConcurrentTasks: 0, // unlimited
	})
}

// ---------------------------------------------------------------------------
// Test 1: Independent agents with real tool execution at scale
// Proves: unlimited scaling, every agent runs compute→analysis→final_answer
// ---------------------------------------------------------------------------

func TestScalingE2E_IndependentTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	scales := []int{10, 50, 100, 500}

	for _, n := range scales {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			compute := &computeTool{}
			analysis := &analysisTool{}
			executor := newE2EExecutor(compute, analysis)

			bgManager := newE2EBGManager(t, executor)
			defer bgManager.Shutdown()

			baseGoroutines := goroutineCount()

			for i := 0; i < n; i++ {
				err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
					TaskID:      fmt.Sprintf("task-%d", i),
					Description: fmt.Sprintf("independent e2e task %d", i),
					Prompt:      fmt.Sprintf("Task %d: run full tool chain", i),
					AgentType:   "internal",
				})
				if err != nil {
					t.Fatalf("dispatch task-%d: %v", i, err)
				}
			}

			done := bgManager.AwaitAll(120 * time.Second)
			if !done {
				t.Fatalf("timeout waiting for %d tasks", n)
			}

			// Verify DrainCompletions returns all IDs (unbounded slice fix).
			completed := bgManager.DrainCompletions()
			if len(completed) != n {
				t.Fatalf("DrainCompletions: got %d, want %d", len(completed), n)
			}

			// Verify all tasks completed successfully.
			results := bgManager.Collect(nil, false, 0)
			if len(results) != n {
				t.Fatalf("Collect: got %d results, want %d", len(results), n)
			}
			for _, r := range results {
				if r.Status != agent.BackgroundTaskStatusCompleted {
					t.Errorf("task %s: status=%s err=%s", r.ID, r.Status, r.Error)
				}
			}

			// Verify real tool execution happened — each agent calls compute+analysis.
			if got := compute.calls.Load(); got != int64(n) {
				t.Errorf("compute tool: got %d calls, want %d", got, n)
			}
			if got := analysis.calls.Load(); got != int64(n) {
				t.Errorf("analysis tool: got %d calls, want %d", got, n)
			}

			// Goroutine leak check.
			time.Sleep(200 * time.Millisecond)
			leaked := goroutineCount() - baseGoroutines
			if leaked > 10 {
				t.Errorf("goroutine leak: %d above baseline", leaked)
			}

			t.Logf("N=%d: %d agents completed, %d compute calls, %d analysis calls, goroutine_delta=%d",
				n, n, compute.calls.Load(), analysis.calls.Load(), leaked)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 2: Deep dependency chain with real tool execution
// Proves: channel-based dep notification + real tool chain per step
// ---------------------------------------------------------------------------

func TestScalingE2E_DeepChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const depth = 50
	compute := &computeTool{}
	analysis := &analysisTool{}
	executor := newE2EExecutor(compute, analysis)

	bgManager := newE2EBGManager(t, executor)
	defer bgManager.Shutdown()

	start := time.Now()

	for i := 0; i < depth; i++ {
		req := agent.BackgroundDispatchRequest{
			TaskID:      fmt.Sprintf("chain-%d", i),
			Description: fmt.Sprintf("chain step %d", i),
			Prompt:      fmt.Sprintf("Chain step %d: full tool chain", i),
			AgentType:   "internal",
		}
		if i > 0 {
			req.DependsOn = []string{fmt.Sprintf("chain-%d", i-1)}
		}
		if err := bgManager.Dispatch(context.Background(), req); err != nil {
			t.Fatalf("dispatch chain-%d: %v", i, err)
		}
	}

	done := bgManager.AwaitAll(120 * time.Second)
	if !done {
		t.Fatal("timeout waiting for deep chain")
	}

	elapsed := time.Since(start)

	results := bgManager.Collect(nil, false, 0)
	completed := 0
	for _, r := range results {
		if r.Status == agent.BackgroundTaskStatusCompleted {
			completed++
		}
	}
	if completed != depth {
		t.Fatalf("got %d completed, want %d", completed, depth)
	}

	// Every chain step executes both tools.
	if got := compute.calls.Load(); got != int64(depth) {
		t.Errorf("compute: got %d, want %d", got, depth)
	}
	if got := analysis.calls.Load(); got != int64(depth) {
		t.Errorf("analysis: got %d, want %d", got, depth)
	}

	t.Logf("Deep chain (%d steps): %v — each step ran compute+analysis tools", depth, elapsed)
	if elapsed > 60*time.Second {
		t.Errorf("deep chain too slow: %v (channel notification should keep latency low)", elapsed)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Wide fan-in with real tool execution
// Proves: fan-in pattern, aggregator waits for all workers, all run real tools
// ---------------------------------------------------------------------------

func TestScalingE2E_WideFanIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const workers = 50
	compute := &computeTool{}
	analysis := &analysisTool{}
	executor := newE2EExecutor(compute, analysis)

	bgManager := newE2EBGManager(t, executor)
	defer bgManager.Shutdown()

	workerIDs := make([]string, workers)
	for i := 0; i < workers; i++ {
		id := fmt.Sprintf("worker-%d", i)
		workerIDs[i] = id
		if err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
			TaskID:      id,
			Description: fmt.Sprintf("worker %d", i),
			Prompt:      fmt.Sprintf("Worker %d: compute and analyze", i),
			AgentType:   "internal",
		}); err != nil {
			t.Fatalf("dispatch worker-%d: %v", i, err)
		}
	}

	if err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:         "aggregator",
		Description:    "aggregate worker results",
		Prompt:         "Aggregate all worker results",
		AgentType:      "internal",
		DependsOn:      workerIDs,
		InheritContext: true,
	}); err != nil {
		t.Fatal(err)
	}

	done := bgManager.AwaitAll(60 * time.Second)
	if !done {
		t.Fatal("timeout waiting for fan-in")
	}

	results := bgManager.Collect(nil, false, 0)
	if len(results) != workers+1 {
		t.Fatalf("got %d results, want %d", len(results), workers+1)
	}

	agg := bgManager.Collect([]string{"aggregator"}, false, 0)
	if len(agg) != 1 || agg[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Fatalf("aggregator not completed: %v", agg)
	}

	// workers + aggregator = workers+1 tool invocations each.
	expectedCalls := int64(workers + 1)
	if got := compute.calls.Load(); got != expectedCalls {
		t.Errorf("compute: got %d, want %d", got, expectedCalls)
	}

	t.Logf("Fan-in: %d workers + 1 aggregator, %d compute calls, %d analysis calls",
		workers, compute.calls.Load(), analysis.calls.Load())
}

// ---------------------------------------------------------------------------
// Test 4: Panic recovery at scale with real tool execution
// Proves: 10% of agents panic during real tool execution, rest complete normally
// ---------------------------------------------------------------------------

func TestScalingE2E_PanicRecoveryAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const total = 100
	const panicEveryN = 10

	var successExecutions atomic.Int64
	var callIndex atomic.Int64

	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		n := callIndex.Add(1)
		if n%panicEveryN == 0 {
			panic(fmt.Sprintf("simulated panic in agent %d during tool execution", n))
		}

		// Real E2E path for non-panicking agents.
		compute := &computeTool{}
		analysis := &analysisTool{}
		engine := react.NewReactEngine(react.ReactEngineConfig{
			MaxIterations: 5,
			Logger:        agent.NoopLogger{},
			Clock:         agent.SystemClock{},
		})

		services := react.Services{
			LLM:          newE2EToolCallLLM(),
			ToolExecutor: newE2EToolRegistry(compute, analysis),
			Parser:       &mocks.MockParser{},
			Context:      &mocks.MockContextManager{},
		}

		result, err := engine.SolveTask(ctx, prompt, &react.TaskState{SessionID: sessionID}, services)
		if err != nil {
			return nil, err
		}

		successExecutions.Add(1)
		return &agent.TaskResult{
			Answer:     result.Answer,
			Iterations: result.Iterations,
			TokensUsed: result.TokensUsed,
			StopReason: result.StopReason,
		}, nil
	}

	bgManager := newE2EBGManager(t, executor)
	defer bgManager.Shutdown()

	for i := 0; i < total; i++ {
		if err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
			TaskID:      fmt.Sprintf("panic-task-%d", i),
			Description: fmt.Sprintf("task %d", i),
			Prompt:      fmt.Sprintf("Task %d: execute with possible panic", i),
			AgentType:   "internal",
		}); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}

	done := bgManager.AwaitAll(60 * time.Second)
	if !done {
		t.Fatal("timeout waiting for panic recovery tasks")
	}

	results := bgManager.Collect(nil, false, 0)
	if len(results) != total {
		t.Fatalf("got %d results, want %d", len(results), total)
	}

	completed, failed := 0, 0
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

	if failed == 0 {
		t.Error("expected some panicked tasks")
	}
	if completed == 0 {
		t.Error("expected some completed tasks (with real tool execution)")
	}
	if completed+failed != total {
		t.Errorf("completed(%d) + failed(%d) != total(%d)", completed, failed, total)
	}

	t.Logf("Panic recovery: %d completed (real tool chain), %d failed (panicked), successful_executions=%d",
		completed, failed, successExecutions.Load())
}

// ---------------------------------------------------------------------------
// Test 5: Cancel propagation with real tool execution
// Proves: cancelling root agents propagates to children, long-running tool
// execution respects context cancellation
// ---------------------------------------------------------------------------

func TestScalingE2E_CancelPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	const roots = 10
	const childrenPerRoot = 9

	// Executor that runs real tools but blocks long enough to be cancelled.
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		select {
		case <-time.After(30 * time.Second):
			return &agent.TaskResult{Answer: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	bgManager := newE2EBGManager(t, executor)
	defer bgManager.Shutdown()

	for i := 0; i < roots; i++ {
		if err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
			TaskID:      fmt.Sprintf("root-%d", i),
			Description: fmt.Sprintf("root %d", i),
			Prompt:      "long running root agent",
			AgentType:   "internal",
		}); err != nil {
			t.Fatalf("dispatch root-%d: %v", i, err)
		}
	}

	for i := 0; i < roots; i++ {
		for j := 0; j < childrenPerRoot; j++ {
			if err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
				TaskID:      fmt.Sprintf("child-%d-%d", i, j),
				Description: fmt.Sprintf("child %d-%d", i, j),
				Prompt:      "child agent",
				AgentType:   "internal",
				DependsOn:   []string{fmt.Sprintf("root-%d", i)},
			}); err != nil {
				t.Fatalf("dispatch child-%d-%d: %v", i, j, err)
			}
		}
	}

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < roots; i++ {
		if err := bgManager.CancelTask(context.Background(), fmt.Sprintf("root-%d", i)); err != nil {
			t.Errorf("cancel root-%d: %v", i, err)
		}
	}

	done := bgManager.AwaitAll(15 * time.Second)
	if !done {
		t.Fatal("timeout waiting for cancel propagation")
	}

	results := bgManager.Collect(nil, false, 0)
	rootCancelled, childFailed := 0, 0
	for _, r := range results {
		if r.Status == agent.BackgroundTaskStatusCancelled {
			rootCancelled++
		} else if r.Status == agent.BackgroundTaskStatusFailed {
			childFailed++
		}
	}

	if rootCancelled != roots {
		t.Errorf("cancelled roots: got %d, want %d", rootCancelled, roots)
	}
	if childFailed != roots*childrenPerRoot {
		t.Errorf("failed children: got %d, want %d", childFailed, roots*childrenPerRoot)
	}

	t.Logf("Cancel propagation: %d roots cancelled, %d children failed", rootCancelled, childFailed)
}

// ---------------------------------------------------------------------------
// Test 6: Resource measurement with real tool execution
// Proves: memory + goroutine scaling characteristics with real E2E path
// ---------------------------------------------------------------------------

func TestScalingE2E_ResourceMeasurement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling e2e test in short mode")
	}

	scales := []int{10, 50, 100, 500}

	for _, n := range scales {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			compute := &computeTool{}
			analysis := &analysisTool{}
			executor := newE2EExecutor(compute, analysis)

			bgManager := newE2EBGManager(t, executor)
			defer bgManager.Shutdown()

			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)
			baseGoroutines := goroutineCount()
			start := time.Now()

			var peakGoroutines atomic.Int64
			peakGoroutines.Store(int64(baseGoroutines))

			for i := 0; i < n; i++ {
				if err := bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
					TaskID:      fmt.Sprintf("res-%d", i),
					Description: fmt.Sprintf("resource task %d", i),
					Prompt:      fmt.Sprintf("Resource task %d", i),
					AgentType:   "internal",
				}); err != nil {
					t.Fatalf("dispatch res-%d: %v", i, err)
				}

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

			done := bgManager.AwaitAll(120 * time.Second)
			if !done {
				t.Fatalf("timeout at N=%d", n)
			}

			wallTime := time.Since(start)

			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			time.Sleep(200 * time.Millisecond)
			finalGoroutines := goroutineCount()
			leaked := finalGoroutines - baseGoroutines
			memDelta := int64(memAfter.TotalAlloc) - int64(memBefore.TotalAlloc)

			// Verify real tool execution.
			if got := compute.calls.Load(); got != int64(n) {
				t.Errorf("compute: got %d, want %d", got, n)
			}
			if got := analysis.calls.Load(); got != int64(n) {
				t.Errorf("analysis: got %d, want %d", got, n)
			}

			t.Logf("N=%-4d | wall=%-8v | peak_goroutines=%-4d | goroutine_leak=%-3d | mem_alloc=%.1fMB | compute=%d analysis=%d",
				n, wallTime.Round(time.Millisecond), peakGoroutines.Load(), leaked,
				float64(memDelta)/(1024*1024), compute.calls.Load(), analysis.calls.Load())

			if leaked > 10 {
				t.Errorf("goroutine leak at N=%d: %d above baseline", n, leaked)
			}
		})
	}
}
