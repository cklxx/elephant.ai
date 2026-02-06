package react

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"

	"github.com/stretchr/testify/require"
)

type stubWorkflowTracker struct {
	mu      sync.Mutex
	ensured map[string]any
	started []string
	success map[string]any
	failure map[string]error
}

func newStubWorkflowTracker() *stubWorkflowTracker {
	return &stubWorkflowTracker{
		ensured: make(map[string]any),
		success: make(map[string]any),
		failure: make(map[string]error),
	}
}

func (s *stubWorkflowTracker) EnsureNode(id string, input any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensured[id] = input
}

func (s *stubWorkflowTracker) StartNode(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = append(s.started, id)
}

func (s *stubWorkflowTracker) CompleteNodeSuccess(id string, output any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.success[id] = output
}

func (s *stubWorkflowTracker) CompleteNodeFailure(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failure[id] = err
}

type collectingListener struct {
	mu     sync.Mutex
	events []AgentEvent
}

func (c *collectingListener) OnEvent(event AgentEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *collectingListener) collected() []AgentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]AgentEvent(nil), c.events...)
}

type stubIterationHook struct {
	memoriesInjected int
}

func (s *stubIterationHook) OnIteration(_ context.Context, _ *TaskState, _ int) agent.IterationHookResult {
	return agent.IterationHookResult{MemoriesInjected: s.memoriesInjected}
}

func TestReactRuntimeFinalizeResultDecoratesWorkflowOnce(t *testing.T) {
	tracker := newStubWorkflowTracker()
	now := time.Now()
	clock := agent.ClockFunc(func() time.Time { return now })

	engine := NewReactEngine(ReactEngineConfig{
		Logger:   agent.NoopLogger{},
		Clock:    clock,
		Workflow: tracker,
	})

	state := &TaskState{
		SessionID: "s1",
		RunID:     "t1",
	}

	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)
	now = now.Add(2 * time.Second)

	result := runtime.finalizeResult("done", &TaskResult{Answer: "ok", Iterations: 3, TokensUsed: 10}, false, nil)

	require.Equal(t, "done", result.StopReason)
	require.Equal(t, 2*time.Second, result.Duration)
	require.Contains(t, tracker.started, workflowNodeFinalize)

	finalized, ok := tracker.success[workflowNodeFinalize]
	require.True(t, ok, "expected finalize node to complete successfully")

	output, ok := finalized.(map[string]any)
	require.True(t, ok, "finalize output should be a map")
	require.Equal(t, "done", output["stop_reason"])
	require.Equal(t, 3, output["iterations"])
	require.Equal(t, 10, output["tokens_used"])
}

func TestReactRuntimeCancellationEmitsCompletionEvent(t *testing.T) {
	tracker := newStubWorkflowTracker()
	listener := &collectingListener{}
	now := time.Now()
	clock := agent.ClockFunc(func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := NewReactEngine(ReactEngineConfig{
		Logger:        agent.NoopLogger{},
		Clock:         clock,
		EventListener: listener,
		Workflow:      tracker,
	})

	state := &TaskState{
		SessionID: "s1",
		RunID:     "t1",
	}

	runtime := newReactRuntime(engine, ctx, "demo", state, Services{}, nil)

	now = now.Add(time.Second)
	result, err := runtime.run()

	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, result)
	require.Equal(t, "cancelled", result.StopReason)

	events := listener.collected()
	var completes []*domain.WorkflowResultFinalEvent
	for _, evt := range events {
		if tc, ok := evt.(*domain.WorkflowResultFinalEvent); ok {
			completes = append(completes, tc)
		}
	}

	require.Len(t, completes, 1)
	require.Equal(t, "cancelled", completes[0].StopReason)
	require.Equal(t, time.Second, completes[0].Duration)
	require.Contains(t, tracker.started, workflowNodeFinalize)
}

func TestReactRuntimeIterationHookEmitsRefreshEvent(t *testing.T) {
	listener := &collectingListener{}
	hook := &stubIterationHook{memoriesInjected: 1}
	engine := NewReactEngine(ReactEngineConfig{
		Logger:        agent.NoopLogger{},
		EventListener: listener,
		IterationHook: hook,
	})

	state := &TaskState{
		SessionID: "s1",
		RunID:     "r1",
	}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	runtime.applyIterationHook(1)

	events := listener.collected()
	found := false
	for _, ev := range events {
		if refresh, ok := ev.(*domain.ProactiveContextRefreshEvent); ok {
			found = true
			require.Equal(t, 1, refresh.Iteration)
			require.Equal(t, 1, refresh.MemoriesInjected)
		}
	}
	require.True(t, found, "expected proactive context refresh event")
}

func TestBackgroundDispatchEmitsEvent(t *testing.T) {
	listener := &collectingListener{}
	engine := NewReactEngine(ReactEngineConfig{
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(time.Now),
		EventListener: listener,
		BackgroundExecutor: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "ok"}, nil
		},
	})
	state := &TaskState{
		SessionID: "s1",
		RunID:     "t1",
	}

	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)
	require.NotNil(t, runtime.bgManager)

	dispatcher := newBackgroundDispatcherWithEvents(runtime, runtime.bgManager)
	err := dispatcher.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "task-1",
		Description: "desc",
		Prompt:      "prompt",
		AgentType:   "internal",
		CausationID: "cause-1",
	})
	require.NoError(t, err)

	var dispatched []*domain.BackgroundTaskDispatchedEvent
	for _, evt := range listener.collected() {
		if d, ok := evt.(*domain.BackgroundTaskDispatchedEvent); ok {
			dispatched = append(dispatched, d)
		}
	}
	require.Len(t, dispatched, 1)
	require.Equal(t, "task-1", dispatched[0].TaskID)
	require.Equal(t, "desc", dispatched[0].Description)
	require.Equal(t, "prompt", dispatched[0].Prompt)
	require.Equal(t, "internal", dispatched[0].AgentType)
}

func TestCleanupEmitsBackgroundCompletionEvents(t *testing.T) {
	listener := &collectingListener{}
	engine := NewReactEngine(ReactEngineConfig{
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(time.Now),
		EventListener: listener,
		BackgroundExecutor: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "done"}, nil
		},
	})
	state := &TaskState{
		SessionID: "s1",
		RunID:     "t1",
	}

	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)
	require.NotNil(t, runtime.bgManager)

	err := runtime.bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "task-1",
		Description: "desc",
		Prompt:      "prompt",
		AgentType:   "internal",
		CausationID: "cause-1",
	})
	require.NoError(t, err)

	runtime.bgManager.AwaitAll(2 * time.Second)
	runtime.cleanupBackgroundTasks()

	var completed []*domain.BackgroundTaskCompletedEvent
	for _, evt := range listener.collected() {
		if c, ok := evt.(*domain.BackgroundTaskCompletedEvent); ok {
			completed = append(completed, c)
		}
	}
	require.Len(t, completed, 1)
	require.Equal(t, "task-1", completed[0].TaskID)
	require.Equal(t, "completed", completed[0].Status)
}

func TestCleanupSkipsAlreadyEmittedBackgroundCompletions(t *testing.T) {
	listener := &collectingListener{}
	engine := NewReactEngine(ReactEngineConfig{
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(time.Now),
		EventListener: listener,
		BackgroundExecutor: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "done"}, nil
		},
	})
	state := &TaskState{
		SessionID: "s1",
		RunID:     "t1",
	}

	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)
	require.NotNil(t, runtime.bgManager)

	err := runtime.bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "task-1",
		Description: "desc",
		Prompt:      "prompt",
		AgentType:   "internal",
		CausationID: "cause-1",
	})
	require.NoError(t, err)

	runtime.bgManager.AwaitAll(2 * time.Second)
	runtime.injectBackgroundNotifications()
	runtime.cleanupBackgroundTasks()

	var completed []*domain.BackgroundTaskCompletedEvent
	for _, evt := range listener.collected() {
		if c, ok := evt.(*domain.BackgroundTaskCompletedEvent); ok {
			completed = append(completed, c)
		}
	}
	require.Len(t, completed, 1)
}

func TestSharedBackgroundManagerDoesNotCancelOnCleanup(t *testing.T) {
	mgr := newBackgroundTaskManager(
		context.Background(),
		agent.NoopLogger{},
		testClock{},
		blockingExecutor(2*time.Second, "late"),
		nil,
		nil,
		nil,
		"s1",
		nil,
	)
	engine := NewReactEngine(ReactEngineConfig{
		Logger:             agent.NoopLogger{},
		Clock:              testClock{},
		BackgroundExecutor: blockingExecutor(2*time.Second, "late"),
		BackgroundManager:  mgr,
	})
	state := &TaskState{
		SessionID: "s1",
		RunID:     "t1",
	}

	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)
	require.NotNil(t, runtime.bgManager)

	err := runtime.bgManager.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "task-1",
		Description: "desc",
		Prompt:      "prompt",
		AgentType:   "internal",
	})
	require.NoError(t, err)

	runtime.cleanupBackgroundTasks()

	summaries := runtime.bgManager.Status([]string{"task-1"})
	require.Len(t, summaries, 1)
	require.NotEqual(t, agent.BackgroundTaskStatusCancelled, summaries[0].Status)
}

func TestRecordThoughtAppendsThinkingOnlyMessage(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	runtime := &reactRuntime{engine: engine, state: state}
	iteration := &reactIteration{runtime: runtime}

	iteration.recordThought(&Message{
		Role:     "assistant",
		Thinking: ports.Thinking{Parts: []ports.ThinkingPart{{Kind: "reasoning", Text: "plan"}}},
	})

	require.Len(t, state.Messages, 1)
	require.Len(t, state.Messages[0].Thinking.Parts, 1)
}

func TestReactRuntimeAllowsActionWithoutPlan(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		RunID: "run-no-plan",
	}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	// Simulate action tool calls without prior plan() — should not be blocked.
	calls := []ToolCall{
		{Name: "web_search", Arguments: map[string]any{"query": "test"}},
		{Name: "file_read", Arguments: map[string]any{"path": "/tmp/test"}},
	}

	blocked, msg := runtime.enforceOrchestratorGates(calls)
	require.False(t, blocked, "action tools should NOT be blocked without prior plan()")
	require.Empty(t, msg)
}

func TestReactRuntimeBlocksParallelPlanCalls(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{RunID: "run-parallel"}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	calls := []ToolCall{
		{Name: "plan", Arguments: map[string]any{"overall_goal_ui": "goal", "complexity": "simple"}},
		{Name: "web_search", Arguments: map[string]any{"query": "test"}},
	}

	blocked, msg := runtime.enforceOrchestratorGates(calls)
	require.True(t, blocked, "plan() in parallel with other tools should be blocked")
	require.Contains(t, msg, "plan()")
}

func TestReactRuntimeBlocksParallelClarifyCalls(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{RunID: "run-parallel"}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	calls := []ToolCall{
		{Name: "clarify", Arguments: map[string]any{"task_goal_ui": "sub"}},
		{Name: "web_search", Arguments: map[string]any{"query": "test"}},
	}

	blocked, msg := runtime.enforceOrchestratorGates(calls)
	require.True(t, blocked, "clarify() in parallel with other tools should be blocked")
	require.Contains(t, msg, "clarify()")
}

func TestPlanReviewTriggersPauseAndMarker(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		RunID:             "run-123",
		PlanReviewEnabled: true,
	}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	calls := []ToolCall{{
		Name: "plan",
		Arguments: map[string]any{
			"complexity":      "complex",
			"overall_goal_ui": "ship feature",
			"internal_plan":   map[string]any{"steps": []any{"a", "b"}},
		},
	}}
	results := []ToolResult{{
		Metadata: map[string]any{
			"complexity":      "complex",
			"overall_goal_ui": "ship feature",
			"internal_plan":   map[string]any{"steps": []any{"a", "b"}},
		},
	}}

	runtime.updateOrchestratorState(calls, results)

	require.True(t, runtime.pauseRequested, "expected pauseRequested for plan review")
	found := false
	for _, msg := range state.Messages {
		if strings.Contains(msg.Content, "<plan_review_pending>") {
			found = true
			break
		}
	}
	require.True(t, found, "expected plan review marker in messages")
}

func TestClarifyCreatesPlanNode(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{RunID: "run-plan"}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	calls := []ToolCall{{Name: "clarify"}}
	results := []ToolResult{{
		Metadata: map[string]any{
			"task_id":          "task-1",
			"task_goal_ui":     "Ship feature",
			"success_criteria": []string{"tests pass", "docs updated"},
		},
	}}

	runtime.updateOrchestratorState(calls, results)

	require.Len(t, state.Plans, 1)
	require.Equal(t, "task-1", state.Plans[0].ID)
	require.Equal(t, "Ship feature", state.Plans[0].Title)
	require.Equal(t, planStatusInProgress, state.Plans[0].Status)
	require.Contains(t, state.Plans[0].Description, "tests pass")
}

func TestClarifyCompletesPreviousPlanNode(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{RunID: "run-plan"}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)

	runtime.updateOrchestratorState([]ToolCall{{Name: "clarify"}}, []ToolResult{{
		Metadata: map[string]any{
			"task_id":      "task-1",
			"task_goal_ui": "First task",
		},
	}})

	runtime.updateOrchestratorState([]ToolCall{{Name: "clarify"}}, []ToolResult{{
		Metadata: map[string]any{
			"task_id":      "task-2",
			"task_goal_ui": "Second task",
		},
	}})

	require.Len(t, state.Plans, 2)
	require.Equal(t, planStatusCompleted, state.Plans[0].Status)
	require.Equal(t, planStatusInProgress, state.Plans[1].Status)
}

func TestToolErrorBlocksPlanNodeAndRequestsReplan(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		RunID: "run-plan",
		Plans: []agent.PlanNode{{
			ID:     "task-1",
			Title:  "Failure task",
			Status: planStatusInProgress,
		}},
	}
	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{}, nil)
	runtime.currentTaskID = "task-1"

	runtime.updateOrchestratorState([]ToolCall{{Name: "web_search"}}, []ToolResult{{
		Error: errors.New("boom"),
	}})

	require.Equal(t, planStatusBlocked, state.Plans[0].Status)
	found := false
	for _, msg := range state.Messages {
		if strings.Contains(msg.Content, "重新规划") {
			found = true
			break
		}
	}
	require.True(t, found, "expected replan prompt to be injected")
}
