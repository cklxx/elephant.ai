package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/config"
	"alex/internal/tools/builtin/okr"
)

// mockCoordinator records calls to ExecuteTask.
type mockCoordinator struct {
	mu       sync.Mutex
	calls    []string
	sessions []string
	answer   string
	err      error
}

func (m *mockCoordinator) ExecuteTask(_ context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, task)
	m.sessions = append(m.sessions, sessionID)
	if m.err != nil {
		return nil, m.err
	}
	return &agent.TaskResult{Answer: m.answer}, nil
}

func (m *mockCoordinator) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

type blockingCoordinator struct {
	mu        sync.Mutex
	calls     int
	started   chan struct{}
	release   chan struct{}
	done      chan struct{}
	startOnce sync.Once
	doneOnce  sync.Once
}

func newBlockingCoordinator() *blockingCoordinator {
	return &blockingCoordinator{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (b *blockingCoordinator) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	b.startOnce.Do(func() { close(b.started) })
	<-b.release
	b.doneOnce.Do(func() { close(b.done) })
	return &agent.TaskResult{Answer: "ok"}, nil
}

func (b *blockingCoordinator) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

// mockNotifier records Lark messages.
type mockNotifier struct {
	mu       sync.Mutex
	messages []larkMessage
}

type larkMessage struct {
	ChatID  string
	Content string
}

func (m *mockNotifier) SendLark(_ context.Context, chatID string, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, larkMessage{ChatID: chatID, Content: content})
	return nil
}

func (m *mockNotifier) SendMoltbook(_ context.Context, _ string) error {
	return nil
}

func (m *mockNotifier) messageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func TestScheduler_Disabled(t *testing.T) {
	sched := New(Config{Enabled: false}, nil, nil, nil)
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestScheduler_StaticTriggerRegistration(t *testing.T) {
	coord := &mockCoordinator{answer: "done"}
	notifier := &mockNotifier{}

	sched := New(Config{
		Enabled: true,
		StaticTriggers: []config.SchedulerTriggerConfig{
			{
				Name:     "test-trigger",
				Schedule: "0 9 * * 1",
				Task:     "Test task",
				Channel:  "lark",
				UserID:   "user-1",
				ChatID:   "oc_test",
			},
		},
	}, coord, notifier, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	if sched.TriggerCount() < 1 {
		t.Errorf("expected at least 1 trigger, got %d", sched.TriggerCount())
	}

	names := sched.TriggerNames()
	found := false
	for _, n := range names {
		if n == "test-trigger" {
			found = true
		}
	}
	if !found {
		t.Errorf("trigger 'test-trigger' not found in %v", names)
	}
}

func TestScheduler_InvalidCronExpression(t *testing.T) {
	coord := &mockCoordinator{answer: "done"}

	sched := New(Config{
		Enabled: true,
		StaticTriggers: []config.SchedulerTriggerConfig{
			{
				Name:     "bad-trigger",
				Schedule: "not-a-cron",
				Task:     "Bad task",
			},
		},
	}, coord, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not fail start, just log warning
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// The bad trigger should not be registered
	names := sched.TriggerNames()
	for _, n := range names {
		if n == "bad-trigger" {
			t.Error("bad-trigger should not be registered")
		}
	}
}

func TestScheduler_OKRTriggerSync(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})

	goalContent := `---
id: test-okr
owner: "user-001"
status: active
review_cadence: "0 9 * * 1"
notifications:
  channel: lark
  lark_chat_id: "oc_okr_test"
key_results: {}
---

# Test OKR
`
	if err := store.WriteGoalRaw("test-okr", []byte(goalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	coord := &mockCoordinator{answer: "reviewed"}
	notifier := &mockNotifier{}

	sched := New(Config{
		Enabled:      true,
		OKRGoalsRoot: dir,
	}, coord, notifier, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Should have registered the OKR trigger + the sync job
	names := sched.TriggerNames()
	foundOKR := false
	for _, n := range names {
		if n == "okr:test-okr" {
			foundOKR = true
		}
	}
	if !foundOKR {
		t.Errorf("OKR trigger not found in %v", names)
	}
}

func TestScheduler_OKRTriggerPrune(t *testing.T) {
	dir := t.TempDir()
	store := okr.NewGoalStore(okr.OKRConfig{GoalsRoot: dir})

	goalContent := `---
id: prune-test
owner: "user-001"
status: active
review_cadence: "0 9 * * 1"
notifications:
  channel: lark
  lark_chat_id: "oc_prune"
key_results: {}
---

# Prune test
`
	if err := store.WriteGoalRaw("prune-test", []byte(goalContent)); err != nil {
		t.Fatalf("WriteGoalRaw: %v", err)
	}

	coord := &mockCoordinator{answer: "done"}
	sched := New(Config{
		Enabled:      true,
		OKRGoalsRoot: dir,
	}, coord, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Verify trigger exists
	found := false
	for _, n := range sched.TriggerNames() {
		if n == "okr:prune-test" {
			found = true
		}
	}
	if !found {
		t.Fatal("okr:prune-test should exist")
	}

	// Delete the goal file
	if err := store.DeleteGoal("prune-test"); err != nil {
		t.Fatalf("DeleteGoal: %v", err)
	}

	// Manually trigger sync
	sched.syncOKRTriggers()

	// Verify trigger was pruned
	for _, n := range sched.TriggerNames() {
		if n == "okr:prune-test" {
			t.Error("okr:prune-test should have been pruned")
		}
	}
}

func TestSchedulerExecuteTriggerUsesUniqueSessionID(t *testing.T) {
	coord := &mockCoordinator{answer: "done"}
	sched := New(Config{Enabled: true}, coord, nil, nil)

	trigger := Trigger{
		Name:     "test-trigger",
		Schedule: "* * * * *",
		Task:     "Test task",
	}

	if err := sched.executeTrigger(trigger); err != nil {
		t.Fatalf("executeTrigger: %v", err)
	}
	if err := sched.executeTrigger(trigger); err != nil {
		t.Fatalf("executeTrigger: %v", err)
	}

	if len(coord.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(coord.sessions))
	}
	if coord.sessions[0] == coord.sessions[1] {
		t.Fatalf("expected unique session IDs, got %q", coord.sessions[0])
	}
	for _, sessionID := range coord.sessions {
		if !strings.HasPrefix(sessionID, "scheduler-test-trigger-") {
			t.Fatalf("unexpected session ID %q", sessionID)
		}
	}
}

type timeoutCoordinator struct {
	seenDeadline bool
}

func (t *timeoutCoordinator) ExecuteTask(ctx context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	_, ok := ctx.Deadline()
	t.seenDeadline = ok
	return &agent.TaskResult{Answer: "ok"}, nil
}

func TestSchedulerExecuteTriggerAppliesTimeout(t *testing.T) {
	coord := &timeoutCoordinator{}
	sched := New(Config{
		Enabled:        true,
		TriggerTimeout: 2 * time.Second,
	}, coord, nil, nil)

	if err := sched.executeTrigger(Trigger{Name: "timeout-test", Schedule: "* * * * *", Task: "Task"}); err != nil {
		t.Fatalf("executeTrigger: %v", err)
	}

	if !coord.seenDeadline {
		t.Fatal("expected trigger timeout to set deadline")
	}
}

func TestScheduler_ExecuteTrigger(t *testing.T) {
	coord := &mockCoordinator{answer: "task result"}
	notifier := &mockNotifier{}

	sched := New(Config{Enabled: true}, coord, notifier, nil)

	trigger := Trigger{
		Name:    "exec-test",
		Task:    "Execute this",
		Channel: "lark",
		ChatID:  "oc_exec",
		UserID:  "user-1",
	}

	if err := sched.executeTrigger(trigger); err != nil {
		t.Fatalf("executeTrigger: %v", err)
	}

	if coord.callCount() != 1 {
		t.Errorf("expected 1 coordinator call, got %d", coord.callCount())
	}

	if notifier.messageCount() != 1 {
		t.Fatalf("expected 1 Lark message, got %d", notifier.messageCount())
	}

	notifier.mu.Lock()
	msg := notifier.messages[0]
	notifier.mu.Unlock()

	if msg.ChatID != "oc_exec" {
		t.Errorf("ChatID = %q, want oc_exec", msg.ChatID)
	}
	if msg.Content != "task result" {
		t.Errorf("Content = %q, want 'task result'", msg.Content)
	}
}

func TestScheduler_ExecuteTrigger_NoNotifier(t *testing.T) {
	coord := &mockCoordinator{answer: "done"}

	sched := New(Config{Enabled: true}, coord, nil, nil)

	trigger := Trigger{
		Name:    "no-notifier",
		Task:    "Run this",
		Channel: "lark",
		ChatID:  "oc_test",
	}

	// Should not panic with nil notifier
	if err := sched.executeTrigger(trigger); err != nil {
		t.Fatalf("executeTrigger: %v", err)
	}
	if coord.callCount() != 1 {
		t.Errorf("expected 1 coordinator call, got %d", coord.callCount())
	}
}

func TestFormatResult_Success(t *testing.T) {
	trigger := Trigger{Name: "test"}
	result := &agent.TaskResult{Answer: "all good"}
	got := formatResult(trigger, result, nil)
	if got != "all good" {
		t.Errorf("formatResult = %q, want 'all good'", got)
	}
}

func TestFormatResult_Error(t *testing.T) {
	trigger := Trigger{Name: "test"}
	got := formatResult(trigger, nil, context.DeadlineExceeded)
	if !strings.Contains(got, "failed") {
		t.Errorf("expected 'failed' in output, got %q", got)
	}
}

func TestFormatResult_NilResult(t *testing.T) {
	trigger := Trigger{Name: "test"}
	got := formatResult(trigger, nil, nil)
	if !strings.Contains(got, "no result") {
		t.Errorf("expected 'no result' in output, got %q", got)
	}
}

func TestTrigger_IsOKRTrigger(t *testing.T) {
	okrTrigger := Trigger{GoalID: "q1-2026"}
	if !okrTrigger.IsOKRTrigger() {
		t.Error("expected IsOKRTrigger=true for trigger with GoalID")
	}

	staticTrigger := Trigger{Name: "daily"}
	if staticTrigger.IsOKRTrigger() {
		t.Error("expected IsOKRTrigger=false for trigger without GoalID")
	}
}

func TestScheduler_RapidCronExecution(t *testing.T) {
	coord := &mockCoordinator{answer: "tick"}
	notifier := &mockNotifier{}

	// Use every-minute cron to test actual execution
	sched := New(Config{
		Enabled: true,
		StaticTriggers: []config.SchedulerTriggerConfig{
			{
				Name:     "rapid",
				Schedule: "* * * * *", // every minute
				Task:     "Rapid task",
				Channel:  "lark",
				ChatID:   "oc_rapid",
			},
		},
	}, coord, notifier, nil)

	ctx, cancel := context.WithCancel(context.Background())

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Just verify it registered and can be stopped
	if sched.TriggerCount() < 1 {
		t.Error("expected at least 1 trigger registered")
	}

	cancel()
	// Give a moment for the goroutine to catch the cancel
	time.Sleep(50 * time.Millisecond)
}

func TestScheduler_JobStoreLoadsPersistedJobs(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	payload, err := payloadFromTrigger(Trigger{Channel: "lark", UserID: "user-1", ChatID: "chat-1"})
	if err != nil {
		t.Fatalf("payloadFromTrigger: %v", err)
	}
	job := Job{
		ID:       "persisted",
		Name:     "Persisted",
		CronExpr: "* * * * *",
		Trigger:  "Persisted task",
		Payload:  payload,
		Status:   JobStatusActive,
	}
	if err := store.Save(context.Background(), job); err != nil {
		t.Fatalf("Save: %v", err)
	}

	sched := New(Config{Enabled: true, JobStore: store}, &mockCoordinator{answer: "ok"}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	found := false
	for _, name := range sched.TriggerNames() {
		if name == "persisted" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected persisted job to be registered")
	}
}

func TestScheduler_CooldownSkipsRuns(t *testing.T) {
	coord := &mockCoordinator{answer: "ok"}
	sched := New(Config{
		Enabled:  true,
		Cooldown: 200 * time.Millisecond,
	}, coord, nil, nil)

	sched.mu.Lock()
	if err := sched.registerTriggerLocked(context.Background(), Trigger{Name: "cooldown", Schedule: "* * * * *", Task: "Task"}); err != nil {
		sched.mu.Unlock()
		t.Fatalf("registerTriggerLocked: %v", err)
	}
	sched.mu.Unlock()

	if !sched.runJob("cooldown", jobRunOptions{}) {
		t.Fatal("expected first run to execute")
	}
	if sched.runJob("cooldown", jobRunOptions{}) {
		t.Fatal("expected cooldown to skip execution")
	}
	if coord.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", coord.callCount())
	}
}

func TestScheduler_ConcurrencyLimitSkips(t *testing.T) {
	coord := newBlockingCoordinator()
	sched := New(Config{
		Enabled:       true,
		MaxConcurrent: 1,
	}, coord, nil, nil)

	sched.mu.Lock()
	if err := sched.registerTriggerLocked(context.Background(), Trigger{Name: "concurrent", Schedule: "* * * * *", Task: "Task"}); err != nil {
		sched.mu.Unlock()
		t.Fatalf("registerTriggerLocked: %v", err)
	}
	sched.mu.Unlock()

	go sched.runJob("concurrent", jobRunOptions{})

	waitFor(t, 500*time.Millisecond, func() bool {
		select {
		case <-coord.started:
			return true
		default:
			return false
		}
	})

	if sched.runJob("concurrent", jobRunOptions{}) {
		t.Fatal("expected concurrency limit to skip execution")
	}

	close(coord.release)

	waitFor(t, 500*time.Millisecond, func() bool {
		select {
		case <-coord.done:
			return true
		default:
			return false
		}
	})

	if coord.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", coord.callCount())
	}
}

func TestScheduler_RecoveryRetriesAndPauses(t *testing.T) {
	coord := &mockCoordinator{err: errors.New("boom")}
	sched := New(Config{
		Enabled:            true,
		RecoveryMaxRetries: 1,
		RecoveryBackoff:    10 * time.Millisecond,
	}, coord, nil, nil)
	t.Cleanup(sched.Stop)

	sched.mu.Lock()
	if err := sched.registerTriggerLocked(context.Background(), Trigger{Name: "recover", Schedule: "* * * * *", Task: "Task"}); err != nil {
		sched.mu.Unlock()
		t.Fatalf("registerTriggerLocked: %v", err)
	}
	sched.mu.Unlock()

	if !sched.runJob("recover", jobRunOptions{}) {
		t.Fatal("expected first run to execute")
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return coord.callCount() >= 2
	})

	sched.mu.Lock()
	job := sched.jobs["recover"]
	sched.mu.Unlock()

	if job == nil {
		t.Fatal("expected job to exist")
	}
	if job.FailureCount < 2 {
		t.Fatalf("expected failure count >= 2, got %d", job.FailureCount)
	}
	if job.Status != JobStatusPaused {
		t.Fatalf("expected job to be paused after retries, got %s", job.Status)
	}
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if fn() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for condition")
		case <-ticker.C:
		}
	}
}
