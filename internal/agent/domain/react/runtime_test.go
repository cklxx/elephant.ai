package react

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/memory"
	id "alex/internal/utils/id"

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

type stubMemoryService struct {
	entries []memory.Entry
}

func (s *stubMemoryService) Save(_ context.Context, entry memory.Entry) (memory.Entry, error) {
	return entry, nil
}

func (s *stubMemoryService) Recall(_ context.Context, _ memory.Query) ([]memory.Entry, error) {
	return s.entries, nil
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

func TestReactRuntimeRefreshContextInjectsMemories(t *testing.T) {
	listener := &collectingListener{}
	memSvc := &stubMemoryService{
		entries: []memory.Entry{{Content: "Prefer YAML-only config paths."}},
	}
	engine := NewReactEngine(ReactEngineConfig{
		Logger:        agent.NoopLogger{},
		EventListener: listener,
		MemoryRefresh: MemoryRefreshConfig{
			Enabled:   true,
			Interval:  1,
			MaxTokens: 200,
		},
		MemoryService: memSvc,
	})

	state := &TaskState{
		SessionID:   "s1",
		RunID:       "r1",
		ToolResults: []ToolResult{{Content: "config normalization"}},
	}
	ctx := id.WithUserID(context.Background(), "u1")
	runtime := newReactRuntime(engine, ctx, "demo", state, Services{}, nil)

	runtime.refreshContext(1)

	require.Len(t, state.Messages, 1)
	require.Equal(t, "system", state.Messages[0].Role)
	require.Equal(t, ports.MessageSourceProactive, state.Messages[0].Source)
	require.Contains(t, state.Messages[0].Content, "Proactive Memory Refresh")

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
	err := dispatcher.Dispatch(context.Background(), "task-1", "desc", "prompt", "internal", "cause-1")
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

	err := runtime.bgManager.Dispatch(context.Background(), "task-1", "desc", "prompt", "internal", "cause-1")
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

	err := runtime.bgManager.Dispatch(context.Background(), "task-1", "desc", "prompt", "internal", "cause-1")
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
