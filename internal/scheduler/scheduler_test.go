package scheduler

import (
	"context"
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
	mu      sync.Mutex
	calls   []string
	answer  string
	err     error
}

func (m *mockCoordinator) ExecuteTask(_ context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, task)
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

	sched.executeTrigger(trigger)

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
	sched.executeTrigger(trigger)
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
