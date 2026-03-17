package react

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

// testClock is a simple real-time clock for tests.
type testClock struct{}

func (testClock) Now() time.Time { return time.Now() }

// controllableClock allows tests to advance time manually.
type controllableClock struct {
	mu  sync.Mutex
	now time.Time
}

func newControllableClock(t time.Time) *controllableClock {
	return &controllableClock{now: t}
}

func (c *controllableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *controllableClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

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
		nil,
		"test-session",
		nil,
	)
}

func newTestManagerWithMax(
	executor func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error),
	maxConcurrent int,
) *BackgroundTaskManager {
	return NewBackgroundTaskManager(BackgroundManagerConfig{
		RunContext:         context.Background(),
		Logger:             agent.NoopLogger{},
		Clock:              testClock{},
		ExecuteTask:        executor,
		SessionID:          "test-session",
		MaxConcurrentTasks: maxConcurrent,
	})
}

func dispatchTask(t *BackgroundTaskManager, taskID, description, prompt, agentType, causationID string) error {
	return t.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      taskID,
		Description: description,
		Prompt:      prompt,
		AgentType:   agentType,
		CausationID: causationID,
	})
}

func TestDispatchAndDrain(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Millisecond, "result-1"))
	defer mgr.Shutdown()

	err := dispatchTask(mgr, "task-1", "desc", "prompt", "internal", "cause-1")
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

	err := dispatchTask(mgr, "dup", "desc", "prompt", "", "")
	if err != nil {
		t.Fatalf("first dispatch should succeed: %v", err)
	}

	err = dispatchTask(mgr, "dup", "desc2", "prompt2", "", "")
	if err == nil {
		t.Fatal("expected error on duplicate ID")
	}
}

func TestDispatchRespectsMaxConcurrentTasks(t *testing.T) {
	hold := make(chan struct{})
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		select {
		case <-hold:
			return &agent.TaskResult{Answer: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	mgr := newTestManagerWithMax(executor, 1)
	defer mgr.Shutdown()

	if err := dispatchTask(mgr, "task-1", "desc", "prompt", "", ""); err != nil {
		t.Fatalf("first dispatch should succeed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	err := dispatchTask(mgr, "task-2", "desc", "prompt", "", "")
	if err == nil {
		t.Fatal("expected limit error on second dispatch while first is active")
	}
	if !strings.Contains(err.Error(), "limit reached") {
		t.Fatalf("expected limit reached error, got %v", err)
	}

	close(hold)
	mgr.AwaitAll(2 * time.Second)

	if err := dispatchTask(mgr, "task-3", "desc", "prompt", "", ""); err != nil {
		t.Fatalf("dispatch should succeed after active task completes: %v", err)
	}
}

func TestReconcileActiveTaskCountLockedIgnoresCompletedAndSignaledTasks(t *testing.T) {
	mgr := &BackgroundTaskManager{
		tasks: map[string]*backgroundTask{
			"pending": {
				status: agent.BackgroundTaskStatusPending,
			},
			"blocked": {
				status: agent.BackgroundTaskStatusBlocked,
			},
			"running": {
				status: agent.BackgroundTaskStatusRunning,
			},
			"completed": {
				status:      agent.BackgroundTaskStatusCompleted,
				completedAt: time.Now(),
			},
			"failed": {
				status: agent.BackgroundTaskStatusFailed,
			},
			"signaled": {
				status:             agent.BackgroundTaskStatusRunning,
				completionSignaled: true,
			},
		},
	}
	mgr.activeTasks.Store(99)

	got := mgr.reconcileActiveTaskCountLocked()
	if got != 3 {
		t.Fatalf("expected 3 active tasks, got %d", got)
	}
	if stored := mgr.activeTasks.Load(); stored != 3 {
		t.Fatalf("expected reconciled activeTasks=3, got %d", stored)
	}
}

func TestCollectWait(t *testing.T) {
	mgr := newTestManager(blockingExecutor(50*time.Millisecond, "waited-result"))
	defer mgr.Shutdown()

	_ = dispatchTask(mgr, "wait-1", "desc", "prompt", "", "")

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

	_ = dispatchTask(mgr, "slow-1", "desc", "prompt", "", "")

	// Collect immediately without waiting.
	results := mgr.Collect([]string{"slow-1"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should still be running or pending.
	if results[0].Status == agent.BackgroundTaskStatusCompleted {
		t.Fatal("task should not be completed yet")
	}
	if results[0].Duration != 0 {
		t.Fatalf("expected duration to be 0 for pending task, got %v", results[0].Duration)
	}
}

func TestShutdown(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Second, "never"))

	_ = dispatchTask(mgr, "cancel-me", "desc", "prompt", "", "")

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

	_ = dispatchTask(mgr, "fail-1", "desc", "prompt", "", "")
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
		err := dispatchTask(mgr, fmt.Sprintf("task-%d", i), "desc", "prompt", "", "")
		if err != nil {
			t.Fatalf("dispatch %d failed: %v", i, err)
		}
	}

	if mgr.TaskCount() != 5 {
		t.Fatalf("expected 5 tasks, got %d", mgr.TaskCount())
	}

	results := mgr.Collect(nil, true, 10*time.Second)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	deadline := time.Now().Add(2 * time.Second)
	completed := make(map[string]struct{}, 5)
	for len(completed) < 5 && time.Now().Before(deadline) {
		for _, taskID := range mgr.DrainCompletions() {
			completed[taskID] = struct{}{}
		}
		if len(completed) == 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(completed) != 5 {
		t.Fatalf("expected 5 completions, got %d", len(completed))
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

	_ = dispatchTask(mgr, "a", "desc-a", "p", "", "")
	_ = dispatchTask(mgr, "b", "desc-b", "p", "", "")
	_ = dispatchTask(mgr, "c", "desc-c", "p", "", "")

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



func TestTaskDependenciesInheritContext(t *testing.T) {
	var mu sync.Mutex
	var prompts []string
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		// Keep task "b" in blocked state long enough to observe it.
		if strings.TrimSpace(prompt) == "alpha" {
			time.Sleep(50 * time.Millisecond)
		}
		mu.Lock()
		prompts = append(prompts, prompt)
		mu.Unlock()
		return &agent.TaskResult{Answer: fmt.Sprintf("result-%s", prompt)}, nil
	}

	mgr := newBackgroundTaskManager(
		context.Background(),
		agent.NoopLogger{},
		testClock{},
		executor,
		nil,
		nil,
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	err := mgr.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:      "a",
		Description: "first",
		Prompt:      "alpha",
		AgentType:   "internal",
	})
	if err != nil {
		t.Fatalf("dispatch a failed: %v", err)
	}

	err = mgr.Dispatch(context.Background(), agent.BackgroundDispatchRequest{
		TaskID:         "b",
		Description:    "second",
		Prompt:         "beta",
		AgentType:      "internal",
		DependsOn:      []string{"a"},
		InheritContext: true,
	})
	if err != nil {
		t.Fatalf("dispatch b failed: %v", err)
	}

	summaries := mgr.Status([]string{"b"})
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Status != agent.BackgroundTaskStatusBlocked {
		t.Fatalf("expected blocked, got %s", summaries[0].Status)
	}

	mgr.AwaitAll(2 * time.Second)

	summaries = mgr.Status([]string{"b"})
	if summaries[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Fatalf("expected completed, got %s", summaries[0].Status)
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, prompt := range prompts {
		if strings.Contains(prompt, "[Collaboration Context]") && strings.Contains(prompt, "alpha") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dependency context in prompts, got %#v", prompts)
	}
}


func TestManagerEmitsCompletionEventImmediately(t *testing.T) {
	var mu sync.Mutex
	var events []agent.AgentEvent

	mgr := newBackgroundTaskManager(
		context.Background(),
		agent.NoopLogger{},
		testClock{},
		blockingExecutor(10*time.Millisecond, "result-1"),
		func(event agent.AgentEvent) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, event)
		},
		func(ctx context.Context) domain.BaseEvent {
			return domain.NewBaseEvent(agent.LevelCore, "s1", "r1", "", time.Now())
		},
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	if err := dispatchTask(mgr, "task-1", "desc", "prompt", "internal", ""); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	mgr.AwaitAll(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, evt := range events {
		completed, ok := evt.(*domain.Event)
		if !ok || completed.Kind != types.EventBackgroundTaskCompleted {
			continue
		}
		found = true
		if completed.Data.TaskID != "task-1" {
			t.Fatalf("unexpected task id: %q", completed.Data.TaskID)
		}
		if completed.Data.Status != "completed" {
			t.Fatalf("unexpected status: %q", completed.Data.Status)
		}
		if completed.Data.Answer != "result-1" {
			t.Fatalf("unexpected answer: %q", completed.Data.Answer)
		}
	}
	if !found {
		t.Fatalf("expected BackgroundTaskCompletedEvent")
	}
}

func TestTryAutoMerge_CodingSuccess(t *testing.T) {
	ws := &mockWorkspaceManager{
		mergeResult: &agent.MergeResult{
			Success: true,
			Branch:  "elephant/bg-123",
		},
	}
	mgr := &BackgroundTaskManager{workspaceMgr: ws}
	bt := &backgroundTask{
		config: map[string]string{
			"task_kind":        "coding",
			"merge_on_success": "true",
			"verify":           "true",
		},
		workspace: &agent.WorkspaceAllocation{
			Mode:   agent.WorkspaceModeWorktree,
			Branch: "elephant/bg-123",
		},
	}
	result := &agent.TaskResult{Answer: "done"}

	if err := mgr.tryAutoMerge(context.Background(), bt, result); err != nil {
		t.Fatalf("unexpected merge error: %v", err)
	}
	if ws.mergeCalls != 1 {
		t.Fatalf("expected 1 merge call, got %d", ws.mergeCalls)
	}
	if !strings.Contains(result.Answer, "[Auto Merge]") {
		t.Fatalf("expected auto merge marker in answer, got %q", result.Answer)
	}
	if bt.mergeStatus != agent.MergeStatusMerged {
		t.Fatalf("expected merge status %q, got %q", agent.MergeStatusMerged, bt.mergeStatus)
	}
}

func TestTryAutoMerge_RequiresVerifyForCoding(t *testing.T) {
	ws := &mockWorkspaceManager{}
	mgr := &BackgroundTaskManager{workspaceMgr: ws}
	bt := &backgroundTask{
		config: map[string]string{
			"task_kind":        "coding",
			"merge_on_success": "true",
			"verify":           "false",
		},
		workspace: &agent.WorkspaceAllocation{
			Mode: agent.WorkspaceModeWorktree,
		},
	}

	err := mgr.tryAutoMerge(context.Background(), bt, &agent.TaskResult{Answer: "done"})
	if err == nil {
		t.Fatal("expected error when verify is disabled for auto merge")
	}
	if ws.mergeCalls != 0 {
		t.Fatalf("expected no merge calls, got %d", ws.mergeCalls)
	}
	if bt.mergeStatus != agent.MergeStatusFailed {
		t.Fatalf("expected merge status %q, got %q", agent.MergeStatusFailed, bt.mergeStatus)
	}
}








// --- Batch 2: CompletionNotifier ---

// TestCompletionNotifierCalledOnCompletion verifies that BackgroundTaskManager
// calls the CompletionNotifier from context after task completion.
func TestCompletionNotifierCalledOnCompletion(t *testing.T) {
	var mu sync.Mutex
	var notifications []completionNotification

	notifier := &mockCompletionNotifier{
		onNotify: func(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
			mu.Lock()
			defer mu.Unlock()
			notifications = append(notifications, completionNotification{
				taskID: taskID, status: status, answer: answer, errText: errText, mergeStatus: mergeStatus, tokensUsed: tokensUsed,
			})
		},
	}

	ctx := agent.WithCompletionNotifier(context.Background(), notifier)

	mgr := newBackgroundTaskManager(
		ctx,
		agent.NoopLogger{},
		testClock{},
		blockingExecutor(10*time.Millisecond, "notified-result"),
		nil,
		nil,
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	if err := mgr.Dispatch(ctx, agent.BackgroundDispatchRequest{
		TaskID:      "notify-1",
		Description: "test notify",
		Prompt:      "prompt",
		AgentType:   "internal",
	}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	mgr.AwaitAll(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	n := notifications[0]
	if n.taskID != "notify-1" {
		t.Errorf("unexpected taskID: %q", n.taskID)
	}
	if n.status != "completed" {
		t.Errorf("unexpected status: %q", n.status)
	}
	if n.answer != "notified-result" {
		t.Errorf("unexpected answer: %q", n.answer)
	}
	if n.mergeStatus != agent.MergeStatusNotMerged {
		t.Errorf("unexpected merge status: %q", n.mergeStatus)
	}
}

// TestCompletionNotifierCalledOnFailure verifies that CompletionNotifier
// receives the error text when a task fails.
func TestCompletionNotifierCalledOnFailure(t *testing.T) {
	var mu sync.Mutex
	var notifications []completionNotification

	notifier := &mockCompletionNotifier{
		onNotify: func(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
			mu.Lock()
			defer mu.Unlock()
			notifications = append(notifications, completionNotification{
				taskID: taskID, status: status, answer: answer, errText: errText, mergeStatus: mergeStatus, tokensUsed: tokensUsed,
			})
		},
	}

	ctx := agent.WithCompletionNotifier(context.Background(), notifier)

	mgr := newBackgroundTaskManager(
		ctx,
		agent.NoopLogger{},
		testClock{},
		failingExecutor("task exploded"),
		nil,
		nil,
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	if err := mgr.Dispatch(ctx, agent.BackgroundDispatchRequest{
		TaskID:      "fail-notify",
		Description: "test fail notify",
		Prompt:      "prompt",
		AgentType:   "internal",
	}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	mgr.AwaitAll(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	n := notifications[0]
	if n.status != "failed" {
		t.Errorf("unexpected status: %q", n.status)
	}
	if n.errText != "task exploded" {
		t.Errorf("unexpected error: %q", n.errText)
	}
	if n.mergeStatus != agent.MergeStatusNotMerged {
		t.Errorf("unexpected merge status: %q", n.mergeStatus)
	}
}

// --- Batch 4: Heartbeat ---

// TestHeartbeatEmitsDuringExternalExecution verifies that the heartbeat goroutine
// long-running external agent execution.

type completionNotification struct {
	taskID      string
	status      string
	answer      string
	errText     string
	mergeStatus string
	tokensUsed  int
}

type mockCompletionNotifier struct {
	onNotify func(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int)
}

func (m *mockCompletionNotifier) NotifyCompletion(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
	if m.onNotify != nil {
		m.onNotify(ctx, taskID, status, answer, errText, mergeStatus, tokensUsed)
	}
}




type mockWorkspaceManager struct {
	mergeCalls  int
	mergeAlloc  *agent.WorkspaceAllocation
	mergeMode   agent.MergeStrategy
	mergeResult *agent.MergeResult
	mergeErr    error
}

func (m *mockWorkspaceManager) Allocate(ctx context.Context, taskID string, mode agent.WorkspaceMode, fileScope []string) (*agent.WorkspaceAllocation, error) {
	return &agent.WorkspaceAllocation{
		Mode:   mode,
		Branch: taskID,
	}, nil
}

func (m *mockWorkspaceManager) Merge(ctx context.Context, alloc *agent.WorkspaceAllocation, strategy agent.MergeStrategy) (*agent.MergeResult, error) {
	m.mergeCalls++
	m.mergeAlloc = alloc
	m.mergeMode = strategy
	if m.mergeErr != nil {
		return nil, m.mergeErr
	}
	if m.mergeResult != nil {
		return m.mergeResult, nil
	}
	return &agent.MergeResult{Success: true, Branch: alloc.Branch, Strategy: strategy}, nil
}

func TestCancelTask(t *testing.T) {
	mgr := newTestManager(blockingExecutor(5*time.Second, "should-be-cancelled"))
	defer mgr.Shutdown()

	err := dispatchTask(mgr, "cancel-me", "cancel test", "cancel test prompt", "internal", "")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// Wait for the task to start running.
	time.Sleep(50 * time.Millisecond)

	if err := mgr.CancelTask(context.Background(), "cancel-me"); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	results := mgr.Collect([]string{"cancel-me"}, true, 2*time.Second)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusCancelled {
		t.Errorf("expected cancelled, got %s", results[0].Status)
	}
}

func TestCancelTask_NotFound(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Millisecond, "ok"))
	defer mgr.Shutdown()

	err := mgr.CancelTask(context.Background(), "nonexistent")
	if err == nil || !errors.Is(err, ErrBackgroundTaskNotFound) {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestCancelTask_AlreadyCompleted(t *testing.T) {
	mgr := newTestManager(blockingExecutor(10*time.Millisecond, "done"))
	defer mgr.Shutdown()

	err := dispatchTask(mgr, "done-task", "done test", "done prompt", "internal", "")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mgr.Collect([]string{"done-task"}, true, 2*time.Second)

	err = mgr.CancelTask(context.Background(), "done-task")
	if err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("expected already-completed error, got: %v", err)
	}
}

func TestRunTaskPanicRecovery(t *testing.T) {
	panicExecutor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		panic("executor exploded")
	}

	mgr := newTestManager(panicExecutor)
	defer mgr.Shutdown()

	err := dispatchTask(mgr, "panic-1", "panic test", "trigger panic", "internal", "")
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	mgr.AwaitAll(2 * time.Second)

	results := mgr.Collect([]string{"panic-1"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusFailed {
		t.Fatalf("expected failed, got %s", results[0].Status)
	}
	if !strings.Contains(results[0].Error, "panicked") {
		t.Fatalf("expected panic error message, got %q", results[0].Error)
	}
	if !strings.Contains(results[0].Error, "executor exploded") {
		t.Fatalf("expected panic value in error, got %q", results[0].Error)
	}
}

func TestRunTaskPanicRecoveryWithCompletionNotifier(t *testing.T) {
	panicExecutor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		panic("boom")
	}

	var mu sync.Mutex
	var notifications []completionNotification

	notifier := &mockCompletionNotifier{
		onNotify: func(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
			mu.Lock()
			defer mu.Unlock()
			notifications = append(notifications, completionNotification{
				taskID: taskID, status: status, answer: answer, errText: errText, mergeStatus: mergeStatus, tokensUsed: tokensUsed,
			})
		},
	}

	ctx := agent.WithCompletionNotifier(context.Background(), notifier)

	mgr := newBackgroundTaskManager(
		ctx,
		agent.NoopLogger{},
		testClock{},
		panicExecutor,
		nil,
		nil,
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	if err := mgr.Dispatch(ctx, agent.BackgroundDispatchRequest{
		TaskID:      "panic-notify",
		Description: "panic with notifier",
		Prompt:      "trigger panic",
		AgentType:   "internal",
	}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	mgr.AwaitAll(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	n := notifications[0]
	if n.taskID != "panic-notify" {
		t.Errorf("unexpected taskID: %q", n.taskID)
	}
	if n.status != "failed" {
		t.Errorf("unexpected status: %q", n.status)
	}
	if !strings.Contains(n.errText, "panicked") {
		t.Errorf("expected panic in error text, got %q", n.errText)
	}
}


func TestAwaitAllReturnsTimeout(t *testing.T) {
	hold := make(chan struct{})
	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		select {
		case <-hold:
			return &agent.TaskResult{Answer: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	mgr := newTestManager(executor)
	defer mgr.Shutdown()

	if err := dispatchTask(mgr, "timeout-1", "timeout test", "prompt", "internal", ""); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Short timeout — task is still running.
	done := mgr.AwaitAll(50 * time.Millisecond)
	if done {
		t.Fatal("expected false (timeout), got true")
	}

	// Let task finish.
	close(hold)

	// Longer timeout — task should complete.
	done = mgr.AwaitAll(5 * time.Second)
	if !done {
		t.Fatal("expected true (all done), got false")
	}
}

func TestDispatchContextPropagators(t *testing.T) {
	type ctxKey struct{}
	var captured string

	executor := func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
		if v, ok := ctx.Value(ctxKey{}).(string); ok {
			captured = v
		}
		return &agent.TaskResult{Answer: "done"}, nil
	}

	mgr := NewBackgroundTaskManager(BackgroundManagerConfig{
		RunContext:  context.Background(),
		Logger:      agent.NoopLogger{},
		Clock:       testClock{},
		ExecuteTask: executor,
		SessionID:   "test-session",
		ContextPropagators: []agent.ContextPropagatorFunc{
			func(from, to context.Context) context.Context {
				if v, ok := from.Value(ctxKey{}).(string); ok {
					return context.WithValue(to, ctxKey{}, v)
				}
				return to
			},
		},
	})
	defer mgr.Shutdown()

	dispatchCtx := context.WithValue(context.Background(), ctxKey{}, "propagated-value")
	err := mgr.Dispatch(dispatchCtx, agent.BackgroundDispatchRequest{
		TaskID:      "prop-task",
		Description: "test propagation",
		Prompt:      "do something",
	})
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	done := mgr.AwaitAll(5 * time.Second)
	if !done {
		t.Fatal("expected all tasks to complete")
	}

	if captured != "propagated-value" {
		t.Fatalf("expected propagated context value, got %q", captured)
	}
}



// mockTmuxSender is a test double for agent.TmuxSender.
type mockTmuxSender struct {
	mu       sync.Mutex
	calls    []mockTmuxCall
	failOnce bool // if true, first SendKeys call returns an error
}

type mockTmuxCall struct {
	pane string
	data string
}

func (m *mockTmuxSender) SendKeys(_ context.Context, pane string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockTmuxCall{pane: pane, data: data})
	if m.failOnce && len(m.calls) == 1 {
		return fmt.Errorf("inject input to pane %s: denied: exit status 9", pane)
	}
	return nil
}

// testEventAppender is a simple in-test EventAppender that writes to disk.
type testEventAppender struct{}

func (testEventAppender) AppendLine(path string, line string) {
	p := strings.TrimSpace(path)
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(strings.TrimSpace(line) + "\n")
}


func readJSONLinesForTest(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jsonl %s: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(trimmed), &item); err != nil {
			t.Fatalf("unmarshal jsonl line %q: %v", trimmed, err)
		}
		out = append(out, item)
	}
	return out
}
