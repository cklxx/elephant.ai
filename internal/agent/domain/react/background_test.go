package react

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
)

// testClock is a simple real-time clock for tests.
type testClock struct{}

func (testClock) Now() time.Time { return time.Now() }

// blockingExecutor returns a task result after the given delay.
func blockingExecutor(delay time.Duration, answer string) func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		select {
		case <-time.After(delay):
			return &agent.TaskResult{
				Answer:     answer,
				Iterations: 1,
				TokensUsed: 100,
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// failingExecutor returns an error immediately.
func failingExecutor(errMsg string) func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return func(ctx context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
		return nil, fmt.Errorf("%s", errMsg)
	}
}

func newTestManager(executor func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)) *BackgroundTaskManager {
	return newBackgroundTaskManager(
		context.Background(),
		agent.NoopLogger{},
		testClock{},
		executor,
		nil,
		"test-session",
		nil,
	)
}

func TestDispatchAndDrain(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Millisecond, "result-1"))
	defer mgr.Shutdown()

	err := mgr.Dispatch(context.Background(), "task-1", "desc", "prompt", "internal", "cause-1")
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if mgr.TaskCount() != 1 {
		t.Fatalf("expected 1 task, got %d", mgr.TaskCount())
	}

	// Wait for completion.
	mgr.AwaitAll(2 * time.Second)

	completed := mgr.DrainCompletions()
	if len(completed) != 1 || completed[0] != "task-1" {
		t.Fatalf("expected [task-1], got %v", completed)
	}

	// Verify status.
	summaries := mgr.Status(nil)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Fatalf("expected completed, got %s", summaries[0].Status)
	}
}

func TestDuplicateID(t *testing.T) {
	mgr := newTestManager(blockingExecutor(50*time.Millisecond, "ok"))
	defer mgr.Shutdown()

	err := mgr.Dispatch(context.Background(), "dup", "desc", "prompt", "", "")
	if err != nil {
		t.Fatalf("first dispatch should succeed: %v", err)
	}

	err = mgr.Dispatch(context.Background(), "dup", "desc2", "prompt2", "", "")
	if err == nil {
		t.Fatal("expected error on duplicate ID")
	}
}

func TestCollectWait(t *testing.T) {
	mgr := newTestManager(blockingExecutor(50*time.Millisecond, "waited-result"))
	defer mgr.Shutdown()

	_ = mgr.Dispatch(context.Background(), "wait-1", "desc", "prompt", "", "")

	results := mgr.Collect([]string{"wait-1"}, true, 5*time.Second)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Fatalf("expected completed, got %s", results[0].Status)
	}
	if results[0].Answer != "waited-result" {
		t.Fatalf("expected 'waited-result', got %q", results[0].Answer)
	}
}

func TestCollectNoWait(t *testing.T) {
	mgr := newTestManager(blockingExecutor(500*time.Millisecond, "slow"))
	defer mgr.Shutdown()

	_ = mgr.Dispatch(context.Background(), "slow-1", "desc", "prompt", "", "")

	// Collect immediately without waiting.
	results := mgr.Collect([]string{"slow-1"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should still be running or pending.
	if results[0].Status == agent.BackgroundTaskStatusCompleted {
		t.Fatal("task should not be completed yet")
	}
}

func TestShutdown(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Second, "never"))

	_ = mgr.Dispatch(context.Background(), "cancel-me", "desc", "prompt", "", "")

	// Give goroutine time to start.
	time.Sleep(20 * time.Millisecond)

	mgr.Shutdown()

	// Wait briefly for cancellation to propagate.
	mgr.AwaitAll(2 * time.Second)

	results := mgr.Collect([]string{"cancel-me"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusCancelled {
		t.Fatalf("expected cancelled, got %s", results[0].Status)
	}
}

func TestFailedTask(t *testing.T) {
	mgr := newTestManager(failingExecutor("something broke"))
	defer mgr.Shutdown()

	_ = mgr.Dispatch(context.Background(), "fail-1", "desc", "prompt", "", "")
	mgr.AwaitAll(2 * time.Second)

	results := mgr.Collect([]string{"fail-1"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusFailed {
		t.Fatalf("expected failed, got %s", results[0].Status)
	}
	if results[0].Error != "something broke" {
		t.Fatalf("expected error message 'something broke', got %q", results[0].Error)
	}
}

func TestMultipleTasks(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return &agent.TaskResult{Answer: fmt.Sprintf("answer-%d", n)}, nil
	}

	mgr := newTestManager(executor)
	defer mgr.Shutdown()

	for i := 0; i < 5; i++ {
		err := mgr.Dispatch(context.Background(), fmt.Sprintf("task-%d", i), "desc", "prompt", "", "")
		if err != nil {
			t.Fatalf("dispatch %d failed: %v", i, err)
		}
	}

	if mgr.TaskCount() != 5 {
		t.Fatalf("expected 5 tasks, got %d", mgr.TaskCount())
	}

	mgr.AwaitAll(5 * time.Second)

	completed := mgr.DrainCompletions()
	if len(completed) != 5 {
		t.Fatalf("expected 5 completions, got %d", len(completed))
	}

	results := mgr.Collect(nil, false, 0)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s", r.ID, r.Status)
		}
	}
}

func TestStatusFilterByIDs(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Millisecond, "ok"))
	defer mgr.Shutdown()

	_ = mgr.Dispatch(context.Background(), "a", "desc-a", "p", "", "")
	_ = mgr.Dispatch(context.Background(), "b", "desc-b", "p", "", "")
	_ = mgr.Dispatch(context.Background(), "c", "desc-c", "p", "", "")

	mgr.AwaitAll(2 * time.Second)

	// Filter to only a and c.
	summaries := mgr.Status([]string{"a", "c"})
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// Query non-existent ID.
	summaries = mgr.Status([]string{"nonexistent"})
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries for non-existent ID, got %d", len(summaries))
	}
}

func TestExternalExecutor(t *testing.T) {
	ext := &mockExternalExecutor{
		result: &agent.ExternalAgentResult{
			Answer:     "external-result",
			Iterations: 3,
			TokensUsed: 500,
		},
	}
	mgr := newBackgroundTaskManager(
		context.Background(),
		agent.NoopLogger{},
		testClock{},
		blockingExecutor(10*time.Millisecond, "internal"),
		ext,
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	err := mgr.Dispatch(context.Background(), "ext-1", "desc", "prompt", "claude_code", "")
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	mgr.AwaitAll(2 * time.Second)

	results := mgr.Collect([]string{"ext-1"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Answer != "external-result" {
		t.Fatalf("expected 'external-result', got %q", results[0].Answer)
	}
}

func TestExternalExecutorNotConfigured(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Millisecond, "internal"))
	defer mgr.Shutdown()

	err := mgr.Dispatch(context.Background(), "ext-fail", "desc", "prompt", "claude_code", "")
	if err != nil {
		t.Fatalf("dispatch should not fail at dispatch time: %v", err)
	}

	mgr.AwaitAll(2 * time.Second)

	results := mgr.Collect([]string{"ext-fail"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusFailed {
		t.Fatalf("expected failed, got %s", results[0].Status)
	}
}

// mockExternalExecutor implements agent.ExternalAgentExecutor for testing.
type mockExternalExecutor struct {
	result *agent.ExternalAgentResult
	err    error
}

func (m *mockExternalExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockExternalExecutor) SupportedTypes() []string {
	return []string{"claude_code", "cursor"}
}
