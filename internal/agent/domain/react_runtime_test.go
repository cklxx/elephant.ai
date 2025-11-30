package domain

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/ports"

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

func TestReactRuntimeFinalizeResultDecoratesWorkflowOnce(t *testing.T) {
	tracker := newStubWorkflowTracker()
	now := time.Now()
	clock := ports.ClockFunc(func() time.Time { return now })

	engine := NewReactEngine(ReactEngineConfig{
		Logger:   ports.NoopLogger{},
		Clock:    clock,
		Workflow: tracker,
	})

	state := &TaskState{
		SessionID: "s1",
		TaskID:    "t1",
	}

	runtime := newReactRuntime(engine, context.Background(), "demo", state, Services{})
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
	clock := ports.ClockFunc(func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := NewReactEngine(ReactEngineConfig{
		Logger:        ports.NoopLogger{},
		Clock:         clock,
		EventListener: listener,
		Workflow:      tracker,
	})

	state := &TaskState{
		SessionID: "s1",
		TaskID:    "t1",
	}

	runtime := newReactRuntime(engine, ctx, "demo", state, Services{})

	now = now.Add(time.Second)
	result, err := runtime.run()

	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, result)
	require.Equal(t, "cancelled", result.StopReason)

	events := listener.collected()
	var completes []*WorkflowResultFinalEvent
	for _, evt := range events {
		if tc, ok := evt.(*WorkflowResultFinalEvent); ok {
			completes = append(completes, tc)
		}
	}

	require.Len(t, completes, 1)
	require.Equal(t, "cancelled", completes[0].StopReason)
	require.Equal(t, time.Second, completes[0].Duration)
	require.Contains(t, tracker.started, workflowNodeFinalize)
}
