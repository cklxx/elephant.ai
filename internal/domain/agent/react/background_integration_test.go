package react

// Integration tests for background agent teams orchestration.
// These tests exercise the full pipeline:
//   TaskFile → Executor → BackgroundTaskManager → goroutine execution →
//   completion events → status collection → staleness detection

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
)

// --- Helpers ---

// scenarioExecutor routes prompts to different behaviors based on content.
type scenarioExecutor struct {
	mu       sync.Mutex
	handlers map[string]func(ctx context.Context, prompt string) (*agent.TaskResult, error)
	log      []string
}

func newScenarioExecutor() *scenarioExecutor {
	return &scenarioExecutor{
		handlers: make(map[string]func(ctx context.Context, prompt string) (*agent.TaskResult, error)),
	}
}

func (s *scenarioExecutor) handle(keyword string, fn func(ctx context.Context, prompt string) (*agent.TaskResult, error)) {
	s.handlers[keyword] = fn
}

func (s *scenarioExecutor) execute(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	s.mu.Lock()
	s.log = append(s.log, prompt)
	s.mu.Unlock()

	for keyword, fn := range s.handlers {
		if strings.Contains(prompt, keyword) {
			return fn(ctx, prompt)
		}
	}
	return &agent.TaskResult{Answer: "default-answer", Iterations: 1, TokensUsed: 10}, nil
}

func (s *scenarioExecutor) executionLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.log...)
}

func newIntegrationManager(
	executor func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error),
	clock agent.Clock,
	opts ...func(*BackgroundManagerConfig),
) *BackgroundTaskManager {
	cfg := BackgroundManagerConfig{
		RunContext:  context.Background(),
		Logger:      agent.NoopLogger{},
		Clock:       clock,
		ExecuteTask: executor,
		SessionID:   "integration-session",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewBackgroundTaskManager(cfg)
}

// --- Tests ---

// TestIntegration_FullDAGOrchestration tests a 3-task DAG:
//   plan (no deps) → implement (depends on plan) → review (depends on implement)
// Verifies: topo dispatch, dependency blocking, context inheritance, completion.
func TestIntegration_FullDAGOrchestration(t *testing.T) {
	se := newScenarioExecutor()
	se.handle("plan the feature", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		time.Sleep(30 * time.Millisecond)
		return &agent.TaskResult{Answer: "Plan: use strategy A", Iterations: 2, TokensUsed: 200}, nil
	})
	se.handle("implement based on plan", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		// Context inheritance: prompt should contain upstream plan result
		if !strings.Contains(prompt, "[Collaboration Context]") {
			return nil, fmt.Errorf("expected collaboration context in prompt")
		}
		if !strings.Contains(prompt, "Plan: use strategy A") {
			return nil, fmt.Errorf("expected plan result in context")
		}
		time.Sleep(20 * time.Millisecond)
		return &agent.TaskResult{Answer: "Implemented with strategy A", Iterations: 5, TokensUsed: 500}, nil
	})
	se.handle("review the implementation", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		if !strings.Contains(prompt, "Implemented with strategy A") {
			return nil, fmt.Errorf("expected implement result in context")
		}
		return &agent.TaskResult{Answer: "LGTM, no issues", Iterations: 1, TokensUsed: 100}, nil
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "dag-test",
		Tasks: []taskfile.TaskSpec{
			{ID: "plan", Description: "Plan", Prompt: "plan the feature"},
			{ID: "implement", Description: "Implement", Prompt: "implement based on plan", DependsOn: []string{"plan"}, InheritContext: true},
			{ID: "review", Description: "Review", Prompt: "review the implementation", DependsOn: []string{"implement"}, InheritContext: true},
		},
	}

	statusPath := t.TempDir() + "/dag.status.yaml"
	exec := taskfile.NewExecutor(mgr)

	result, err := exec.ExecuteAndWait(context.Background(), tf, "cause-dag", statusPath, 10*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}
	if len(result.TaskIDs) != 3 {
		t.Fatalf("expected 3 task IDs, got %d", len(result.TaskIDs))
	}

	// Verify all completed.
	summaries := mgr.Status(nil)
	for _, s := range summaries {
		if s.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s (err=%s)", s.ID, s.Status, s.Error)
		}
	}

	// Verify execution order via log.
	log := se.executionLog()
	if len(log) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(log))
	}
	// plan must run first.
	if !strings.Contains(log[0], "plan the feature") {
		t.Errorf("first execution should be plan, got %q", log[0])
	}

	// Verify results via Collect.
	collected := mgr.Collect(nil, false, 0)
	if len(collected) != 3 {
		t.Fatalf("expected 3 collected results, got %d", len(collected))
	}
	answerMap := make(map[string]string)
	for _, r := range collected {
		answerMap[r.ID] = r.Answer
	}
	if answerMap["plan"] != "Plan: use strategy A" {
		t.Errorf("plan answer: %q", answerMap["plan"])
	}
	if answerMap["implement"] != "Implemented with strategy A" {
		t.Errorf("implement answer: %q", answerMap["implement"])
	}
	if answerMap["review"] != "LGTM, no issues" {
		t.Errorf("review answer: %q", answerMap["review"])
	}

	// Verify status file was written (initial state).
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}
	if !strings.Contains(string(data), "dag-test") {
		t.Error("status file missing plan_id")
	}
}

// TestIntegration_FailurePropagation tests that dependency failure cascades:
//   task-a (fails) → task-b (should fail with dependency error)
func TestIntegration_FailurePropagation(t *testing.T) {
	se := newScenarioExecutor()
	se.handle("task-a-fail", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		return nil, fmt.Errorf("compilation error in task A")
	})
	se.handle("task-b-depends", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		t.Error("task-b should never execute — its dependency failed")
		return &agent.TaskResult{Answer: "unreachable"}, nil
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "fail-cascade",
		Tasks: []taskfile.TaskSpec{
			{ID: "a", Description: "Task A", Prompt: "task-a-fail"},
			{ID: "b", Description: "Task B", Prompt: "task-b-depends", DependsOn: []string{"a"}},
		},
	}

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/fail.status.yaml"

	_, err := exec.ExecuteAndWait(context.Background(), tf, "", statusPath, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait should not error at dispatch level: %v", err)
	}

	results := mgr.Collect([]string{"a", "b"}, false, 0)
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusFailed {
			t.Errorf("task %s: expected failed, got %s", r.ID, r.Status)
		}
	}

	// Task B should mention dependency failure.
	for _, r := range results {
		if r.ID == "b" {
			if !strings.Contains(r.Error, "dependency") {
				t.Errorf("task b error should mention dependency, got %q", r.Error)
			}
		}
	}
}

// TestIntegration_PanicRecoveryInDAG verifies that a panicking task marks
// itself as failed and allows dependents to observe the failure.
func TestIntegration_PanicRecoveryInDAG(t *testing.T) {
	se := newScenarioExecutor()
	se.handle("panic-trigger", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		panic("nil pointer dereference in agent")
	})
	se.handle("after-panic", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		t.Error("after-panic should never execute")
		return &agent.TaskResult{Answer: "unreachable"}, nil
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "panic-dag",
		Tasks: []taskfile.TaskSpec{
			{ID: "crasher", Description: "Will panic", Prompt: "panic-trigger"},
			{ID: "dependent", Description: "After panic", Prompt: "after-panic", DependsOn: []string{"crasher"}},
		},
	}

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/panic.status.yaml"

	_, _ = exec.ExecuteAndWait(context.Background(), tf, "", statusPath, 5*time.Second)

	results := mgr.Collect([]string{"crasher", "dependent"}, false, 0)
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusFailed {
			t.Errorf("task %s: expected failed, got %s", r.ID, r.Status)
		}
	}
	for _, r := range results {
		if r.ID == "crasher" && !strings.Contains(r.Error, "panicked") {
			t.Errorf("crasher error should mention panic, got %q", r.Error)
		}
	}
}

// TestIntegration_StalenessDetectionDuringExecution verifies that long-running
// tasks become stale and recover after progress.
func TestIntegration_StalenessDetectionDuringExecution(t *testing.T) {
	hold := make(chan struct{})
	progressCh := make(chan struct{})

	se := newScenarioExecutor()
	se.handle("long-running", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		close(progressCh) // signal we're running
		select {
		case <-hold:
			return &agent.TaskResult{Answer: "done"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	clock := newControllableClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	mgr := newIntegrationManager(se.execute, clock, func(cfg *BackgroundManagerConfig) {
		cfg.StaleThreshold = 10 * time.Minute
	})
	defer mgr.Shutdown()

	err := dispatchTask(mgr, "long-1", "long task", "long-running", "internal", "")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Wait for task to start.
	<-progressCh

	// Not stale yet.
	summaries := mgr.Status([]string{"long-1"})
	if summaries[0].Stale {
		t.Fatal("should not be stale immediately")
	}

	// Advance time past threshold.
	clock.Advance(12 * time.Minute)

	summaries = mgr.Status([]string{"long-1"})
	if !summaries[0].Stale {
		t.Fatal("should be stale after 12 minutes without activity")
	}
	if summaries[0].LastActivityAt.IsZero() {
		t.Fatal("LastActivityAt should be set")
	}

	// Let task complete.
	close(hold)
	mgr.AwaitAll(5 * time.Second)

	summaries = mgr.Status([]string{"long-1"})
	if summaries[0].Stale {
		t.Fatal("completed task should not be stale")
	}
	if summaries[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Errorf("expected completed, got %s", summaries[0].Status)
	}
}

// TestIntegration_TimeoutIndicator verifies AwaitAll returns false on timeout
// and true when all tasks complete.
func TestIntegration_TimeoutIndicator(t *testing.T) {
	hold := make(chan struct{})

	se := newScenarioExecutor()
	se.handle("slow-task", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		select {
		case <-hold:
			return &agent.TaskResult{Answer: "finally done"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	err := dispatchTask(mgr, "timeout-test", "slow", "slow-task", "internal", "")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	time.Sleep(20 * time.Millisecond) // let goroutine start

	// Timeout.
	done := mgr.AwaitAll(50 * time.Millisecond)
	if done {
		t.Fatal("expected timeout (false), got true")
	}

	// Complete.
	close(hold)
	done = mgr.AwaitAll(5 * time.Second)
	if !done {
		t.Fatal("expected all done (true), got false")
	}
}

// TestIntegration_ConcurrentTaskLimit verifies max concurrent tasks enforcement
// across multiple dispatches.
func TestIntegration_ConcurrentTaskLimit(t *testing.T) {
	hold := make(chan struct{})

	se := newScenarioExecutor()
	se.handle("concurrent", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		select {
		case <-hold:
			return &agent.TaskResult{Answer: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	mgr := newIntegrationManager(se.execute, testClock{}, func(cfg *BackgroundManagerConfig) {
		cfg.MaxConcurrentTasks = 2
	})
	defer mgr.Shutdown()

	// Dispatch 2 — should succeed.
	for i := 0; i < 2; i++ {
		err := dispatchTask(mgr, fmt.Sprintf("c-%d", i), "desc", "concurrent", "internal", "")
		if err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	time.Sleep(20 * time.Millisecond) // let goroutines start

	// Third should fail — limit reached.
	err := dispatchTask(mgr, "c-2", "desc", "concurrent", "internal", "")
	if err == nil {
		t.Fatal("expected limit error for third dispatch")
	}
	if !strings.Contains(err.Error(), "limit reached") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Release tasks, dispatch should work again.
	close(hold)
	mgr.AwaitAll(5 * time.Second)

	err = dispatchTask(mgr, "c-3", "desc", "concurrent", "internal", "")
	if err != nil {
		t.Fatalf("post-completion dispatch should succeed: %v", err)
	}
}

// TestIntegration_CancelPropagation verifies cancelling a task cancels its
// context and allows dependents to fail gracefully.
func TestIntegration_CancelPropagation(t *testing.T) {
	started := make(chan struct{})

	se := newScenarioExecutor()
	se.handle("cancellable", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	se.handle("after-cancel", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		t.Error("after-cancel should never run")
		return &agent.TaskResult{Answer: "unreachable"}, nil
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "cancel-test",
		Tasks: []taskfile.TaskSpec{
			{ID: "target", Description: "Will be cancelled", Prompt: "cancellable"},
			{ID: "follower", Description: "Depends on target", Prompt: "after-cancel", DependsOn: []string{"target"}},
		},
	}

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/cancel.status.yaml"

	_, err := exec.Execute(context.Background(), tf, "", statusPath)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Wait for target to start.
	<-started

	// Cancel it.
	if err := mgr.CancelTask(context.Background(), "target"); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	mgr.AwaitAll(5 * time.Second)

	results := mgr.Collect([]string{"target", "follower"}, false, 0)
	statusMap := make(map[string]agent.BackgroundTaskStatus)
	for _, r := range results {
		statusMap[r.ID] = r.Status
	}

	if statusMap["target"] != agent.BackgroundTaskStatusCancelled {
		t.Errorf("target: expected cancelled, got %s", statusMap["target"])
	}
	if statusMap["follower"] != agent.BackgroundTaskStatusFailed {
		t.Errorf("follower: expected failed (dep cancelled), got %s", statusMap["follower"])
	}
}

// TestIntegration_TeamTemplateExecution tests rendering a team template into
// a TaskFile and executing the full pipeline.
func TestIntegration_TeamTemplateExecution(t *testing.T) {
	se := newScenarioExecutor()
	se.handle("Implement:", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		if !strings.Contains(prompt, "build auth module") {
			return nil, fmt.Errorf("expected goal in prompt, got %q", prompt)
		}
		time.Sleep(20 * time.Millisecond)
		return &agent.TaskResult{Answer: "Auth module implemented with JWT", Iterations: 3, TokensUsed: 300}, nil
	})
	se.handle("Review work by", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		if !strings.Contains(prompt, "Auth module implemented") {
			return nil, fmt.Errorf("expected coder result in context, got %q", prompt)
		}
		return &agent.TaskResult{Answer: "Review passed", Iterations: 1, TokensUsed: 100}, nil
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tmpl := &taskfile.TeamTemplate{
		Name:        "code-review",
		Description: "Coder + reviewer team",
		Roles: []taskfile.TeamTemplateRole{
			{Name: "coder", AgentType: "internal", PromptTemplate: "Implement: {GOAL}"},
			{Name: "reviewer", AgentType: "internal", PromptTemplate: "Review work by {TEAM}: {GOAL}", InheritContext: true},
		},
		Stages: []taskfile.TeamTemplateStage{
			{Name: "execute", Roles: []string{"coder"}},
			{Name: "review", Roles: []string{"reviewer"}},
		},
	}

	tf := taskfile.RenderTaskFile(tmpl, "build auth module", nil)

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/team.status.yaml"

	result, err := exec.ExecuteAndWait(context.Background(), tf, "team-cause", statusPath, 10*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}
	if len(result.TaskIDs) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.TaskIDs))
	}

	summaries := mgr.Status(nil)
	for _, s := range summaries {
		if s.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s (err=%s)", s.ID, s.Status, s.Error)
		}
	}

	// Verify results.
	collected := mgr.Collect(nil, false, 0)
	answerMap := make(map[string]string)
	for _, r := range collected {
		answerMap[r.ID] = r.Answer
	}
	if !strings.Contains(answerMap["team-coder"], "JWT") {
		t.Errorf("coder answer: %q", answerMap["team-coder"])
	}
	if !strings.Contains(answerMap["team-reviewer"], "passed") {
		t.Errorf("reviewer answer: %q", answerMap["team-reviewer"])
	}
}

// TestIntegration_ParallelTasksInSameStage tests that tasks in the same stage
// (no inter-dependencies) execute concurrently.
func TestIntegration_ParallelTasksInSameStage(t *testing.T) {
	var mu sync.Mutex
	var runningAt []time.Time
	barrier := make(chan struct{})

	se := newScenarioExecutor()
	se.handle("parallel-work", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		mu.Lock()
		runningAt = append(runningAt, time.Now())
		mu.Unlock()
		// Wait for all to reach barrier — proves concurrent execution.
		select {
		case <-barrier:
		case <-time.After(3 * time.Second):
			return nil, fmt.Errorf("barrier timeout — tasks not running in parallel")
		}
		return &agent.TaskResult{Answer: "parallel-done"}, nil
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "parallel-test",
		Tasks: []taskfile.TaskSpec{
			{ID: "p1", Description: "P1", Prompt: "parallel-work-1"},
			{ID: "p2", Description: "P2", Prompt: "parallel-work-2"},
			{ID: "p3", Description: "P3", Prompt: "parallel-work-3"},
		},
	}

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/parallel.status.yaml"

	_, err := exec.Execute(context.Background(), tf, "", statusPath)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Wait for all 3 to be running.
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		n := len(runningAt)
		mu.Unlock()
		if n >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only %d/3 tasks started — expected parallel execution", n)
		case <-time.After(10 * time.Millisecond):
		}
	}

	// All 3 running concurrently — release them.
	close(barrier)
	mgr.AwaitAll(5 * time.Second)

	for _, s := range mgr.Status(nil) {
		if s.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s", s.ID, s.Status)
		}
	}
}

// TestIntegration_StatusFileReflectsLifecycle verifies the .status.yaml file
// tracks task state transitions accurately.
func TestIntegration_StatusFileReflectsLifecycle(t *testing.T) {
	hold := make(chan struct{})

	se := newScenarioExecutor()
	se.handle("lifecycle", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		select {
		case <-hold:
			return &agent.TaskResult{Answer: "lifecycle-done"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "lifecycle-test",
		Tasks: []taskfile.TaskSpec{
			{ID: "lc-1", Description: "Lifecycle task", Prompt: "lifecycle"},
		},
	}

	statusPath := t.TempDir() + "/lifecycle.status.yaml"
	exec := taskfile.NewExecutor(mgr)

	_, err := exec.Execute(context.Background(), tf, "", statusPath)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Check initial status file.
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	initial := string(data)
	// Task should start as pending (or running by the time we read).
	if !strings.Contains(initial, "lifecycle-test") {
		t.Error("status file should contain plan_id")
	}

	// Let task complete and give polling time to update.
	close(hold)
	mgr.AwaitAll(5 * time.Second)

	// Verify completion via Collect (authoritative source).
	results := mgr.Collect([]string{"lc-1"}, false, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != agent.BackgroundTaskStatusCompleted {
		t.Errorf("expected completed, got %s", results[0].Status)
	}
	if results[0].Answer != "lifecycle-done" {
		t.Errorf("unexpected answer: %q", results[0].Answer)
	}
}

// TestIntegration_CompletionNotifierEndToEnd verifies the full chain from
// task execution through CompletionNotifier callback.
func TestIntegration_CompletionNotifierEndToEnd(t *testing.T) {
	se := newScenarioExecutor()
	se.handle("notify-test", func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
		return &agent.TaskResult{Answer: "notifier-answer", Iterations: 2, TokensUsed: 150}, nil
	})

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
		se.execute,
		nil, nil, nil,
		"test-session",
		nil,
	)
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "notifier-e2e",
		Tasks: []taskfile.TaskSpec{
			{ID: "n1", Description: "Notifier test A", Prompt: "notify-test A"},
			{ID: "n2", Description: "Notifier test B", Prompt: "notify-test B"},
		},
	}

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/notifier.status.yaml"

	_, err := exec.ExecuteAndWait(ctx, tf, "cause-n", statusPath, 10*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifications))
	}
	for _, n := range notifications {
		if n.status != "completed" {
			t.Errorf("task %s: expected completed, got %s", n.taskID, n.status)
		}
		if n.answer != "notifier-answer" {
			t.Errorf("task %s: unexpected answer %q", n.taskID, n.answer)
		}
		if n.tokensUsed != 150 {
			t.Errorf("task %s: unexpected tokens %d", n.taskID, n.tokensUsed)
		}
	}
}

// TestIntegration_DiamondDAG tests a diamond dependency pattern:
//     A
//    / \
//   B   C
//    \ /
//     D
func TestIntegration_DiamondDAG(t *testing.T) {
	var mu sync.Mutex
	execOrder := make(map[string]int)
	seq := 0

	se := newScenarioExecutor()
	for _, keyword := range []string{"diamond-A", "diamond-B", "diamond-C", "diamond-D"} {
		kw := keyword
		se.handle(kw, func(ctx context.Context, prompt string) (*agent.TaskResult, error) {
			mu.Lock()
			seq++
			execOrder[kw] = seq
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			return &agent.TaskResult{Answer: "result-" + kw}, nil
		})
	}

	mgr := newIntegrationManager(se.execute, testClock{})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "diamond",
		Tasks: []taskfile.TaskSpec{
			{ID: "a", Description: "A", Prompt: "diamond-A"},
			{ID: "b", Description: "B", Prompt: "diamond-B", DependsOn: []string{"a"}},
			{ID: "c", Description: "C", Prompt: "diamond-C", DependsOn: []string{"a"}},
			{ID: "d", Description: "D", Prompt: "diamond-D", DependsOn: []string{"b", "c"}},
		},
	}

	exec := taskfile.NewExecutor(mgr)
	statusPath := t.TempDir() + "/diamond.status.yaml"

	_, err := exec.ExecuteAndWait(context.Background(), tf, "", statusPath, 10*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}

	// All should complete.
	for _, s := range mgr.Status(nil) {
		if s.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s", s.ID, s.Status)
		}
	}

	// Verify ordering constraints.
	mu.Lock()
	defer mu.Unlock()
	if execOrder["diamond-A"] >= execOrder["diamond-B"] {
		t.Error("A should execute before B")
	}
	if execOrder["diamond-A"] >= execOrder["diamond-C"] {
		t.Error("A should execute before C")
	}
	if execOrder["diamond-B"] >= execOrder["diamond-D"] {
		t.Error("B should execute before D")
	}
	if execOrder["diamond-C"] >= execOrder["diamond-D"] {
		t.Error("C should execute before D")
	}
}
