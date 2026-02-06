package timer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
)

// mockCoordinator records calls to ExecuteTask.
type mockCoordinator struct {
	mu        sync.Mutex
	calls     []executedTask
	result    *agent.TaskResult
	err       error
	callCount int
	done      chan struct{} // closed after first call
}

type executedTask struct {
	Task      string
	SessionID string
}

func newMockCoordinator(result *agent.TaskResult, err error) *mockCoordinator {
	return &mockCoordinator{
		result: result,
		err:    err,
		done:   make(chan struct{}),
	}
}

func (m *mockCoordinator) ExecuteTask(_ context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	m.mu.Lock()
	m.calls = append(m.calls, executedTask{Task: task, SessionID: sessionID})
	m.callCount++
	count := m.callCount
	m.mu.Unlock()

	if count == 1 {
		close(m.done)
	}
	return m.result, m.err
}

func (m *mockCoordinator) waitForCall(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-m.done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for ExecuteTask call")
	}
}

func (m *mockCoordinator) getCalls() []executedTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]executedTask, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// mockNotifier records notifications.
type mockNotifier struct {
	mu       sync.Mutex
	lark     []larkMsg
	moltbook []string
}

type larkMsg struct {
	ChatID  string
	Content string
}

func (n *mockNotifier) SendLark(_ context.Context, chatID, content string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lark = append(n.lark, larkMsg{ChatID: chatID, Content: content})
	return nil
}

func (n *mockNotifier) SendMoltbook(_ context.Context, content string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.moltbook = append(n.moltbook, content)
	return nil
}

func TestManagerOneShotFires(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(&agent.TaskResult{Answer: "done"}, nil)
	notifier := &mockNotifier{}

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 30 * time.Second,
	}, coord, notifier, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "quick timer",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(50 * time.Millisecond),
		Task:      "check weather",
		SessionID: "session-abc",
		UserID:    "user-1",
		Channel:   "lark",
		ChatID:    "oc_123",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}

	if err := mgr.Add(tmr); err != nil {
		t.Fatalf("Add: %v", err)
	}

	coord.waitForCall(t, 5*time.Second)

	calls := coord.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one ExecuteTask call")
	}
	if calls[0].Task != "check weather" {
		t.Errorf("task: got %q, want %q", calls[0].Task, "check weather")
	}
	if calls[0].SessionID != "session-abc" {
		t.Errorf("sessionID: got %q, want %q", calls[0].SessionID, "session-abc")
	}

	// Wait a bit for status update.
	time.Sleep(100 * time.Millisecond)

	got, ok := mgr.Get(tmr.ID)
	if !ok {
		t.Fatal("timer not found after firing")
	}
	if got.Status != StatusFired {
		t.Errorf("status: got %q, want %q", got.Status, StatusFired)
	}
}

func TestManagerRecurring(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(&agent.TaskResult{Answer: "ok"}, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "recurring check",
		Type:      TimerTypeRecurring,
		Schedule:  "* * * * *", // every minute
		Task:      "daily check",
		SessionID: "session-rec",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}

	if err := mgr.Add(tmr); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify it's registered (cron entry exists).
	mgr.mu.Lock()
	_, hasCron := mgr.cronIDs[tmr.ID]
	mgr.mu.Unlock()
	if !hasCron {
		t.Error("expected cron entry for recurring timer")
	}

	// Timer should remain active (recurring doesn't auto-fire to StatusFired).
	got, ok := mgr.Get(tmr.ID)
	if !ok {
		t.Fatal("timer not found")
	}
	if got.Status != StatusActive {
		t.Errorf("status: got %q, want %q", got.Status, StatusActive)
	}
}

func TestManagerCancel(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(&agent.TaskResult{Answer: "ok"}, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "cancellable",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(10 * time.Minute), // far future
		Task:      "should not fire",
		SessionID: "session-cancel",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}

	if err := mgr.Add(tmr); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := mgr.Cancel(tmr.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, ok := mgr.Get(tmr.ID)
	if !ok {
		t.Fatal("timer not found after cancel")
	}
	if got.Status != StatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, StatusCancelled)
	}

	// Verify go timer was removed.
	mgr.mu.Lock()
	_, hasGoTimer := mgr.goTimers[tmr.ID]
	mgr.mu.Unlock()
	if hasGoTimer {
		t.Error("expected go timer to be removed after cancel")
	}

	// Verify coordinator was never called.
	time.Sleep(100 * time.Millisecond)
	if len(coord.getCalls()) > 0 {
		t.Error("cancelled timer should not fire")
	}
}

func TestManagerMaxTimerLimit(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(nil, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   2,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	// Add 2 timers (at the limit).
	for i := 0; i < 2; i++ {
		tmr := &Timer{
			ID:        NewTimerID(),
			Name:      fmt.Sprintf("timer-%d", i),
			Type:      TimerTypeOnce,
			FireAt:    time.Now().Add(10 * time.Minute),
			Task:      "task",
			SessionID: "session-limit",
			CreatedAt: time.Now().UTC(),
			Status:    StatusActive,
		}
		if err := mgr.Add(tmr); err != nil {
			t.Fatalf("Add timer %d: %v", i, err)
		}
	}

	// Third should fail.
	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "over limit",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(10 * time.Minute),
		Task:      "task",
		SessionID: "session-limit",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}
	err = mgr.Add(tmr)
	if err == nil {
		t.Fatal("expected error when exceeding max timer limit")
	}
}

func TestManagerRestartRecovery(t *testing.T) {
	dir := t.TempDir()

	// Phase 1: create a timer and persist it.
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	pastTimer := Timer{
		ID:        "tmr-past",
		Name:      "past due",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(-5 * time.Minute), // past
		Task:      "overdue task",
		SessionID: "session-past",
		CreatedAt: time.Now().Add(-10 * time.Minute).UTC(),
		Status:    StatusActive,
	}
	futureTimer := Timer{
		ID:        "tmr-future",
		Name:      "future",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(50 * time.Millisecond),
		Task:      "future task",
		SessionID: "session-future",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}
	firedTimer := Timer{
		ID:        "tmr-fired",
		Name:      "already fired",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(-1 * time.Hour),
		Task:      "old task",
		CreatedAt: time.Now().Add(-2 * time.Hour).UTC(),
		Status:    StatusFired,
	}

	for _, tmr := range []Timer{pastTimer, futureTimer, firedTimer} {
		if err := store.Save(tmr); err != nil {
			t.Fatalf("Save %s: %v", tmr.ID, err)
		}
	}

	// Phase 2: start a new manager — should recover.
	coord := newMockCoordinator(&agent.TaskResult{Answer: "recovered"}, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   100,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	// Wait for both past-due and future timer to fire.
	coord.waitForCall(t, 5*time.Second)

	// Give a bit more time for the future timer.
	time.Sleep(300 * time.Millisecond)

	calls := coord.getCalls()
	if len(calls) < 1 {
		t.Fatalf("expected at least 1 call, got %d", len(calls))
	}

	// Verify the past-due timer was fired.
	found := false
	for _, c := range calls {
		if c.SessionID == "session-past" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected past-due timer to fire immediately on restart")
	}

	// The already-fired timer should NOT be recovered.
	_, ok := mgr.Get("tmr-fired")
	if ok {
		t.Error("already-fired timer should not be loaded as active")
	}
}

func TestManagerStopCleansUp(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(nil, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Add a far-future timer.
	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "stop test",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(10 * time.Hour),
		Task:      "should not fire",
		SessionID: "session-stop",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}
	if err := mgr.Add(tmr); err != nil {
		t.Fatalf("Add: %v", err)
	}

	mgr.Stop()

	// Done channel should be closed.
	select {
	case <-mgr.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done channel not closed after Stop")
	}

	// No calls should have been made.
	if len(coord.getCalls()) > 0 {
		t.Error("timer should not fire after Stop")
	}
}

func TestManagerSessionIDPassedThrough(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(&agent.TaskResult{Answer: "ok"}, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "session resume",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(50 * time.Millisecond),
		Task:      "resume task",
		SessionID: "session-resume-xyz",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}

	if err := mgr.Add(tmr); err != nil {
		t.Fatalf("Add: %v", err)
	}

	coord.waitForCall(t, 5*time.Second)

	// Wait for fireTimer to complete store update.
	time.Sleep(100 * time.Millisecond)

	calls := coord.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected ExecuteTask call")
	}
	if calls[0].SessionID != "session-resume-xyz" {
		t.Errorf("sessionID: got %q, want %q", calls[0].SessionID, "session-resume-xyz")
	}
}

func TestManagerNotification(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(&agent.TaskResult{Answer: "timer result"}, nil)
	notifier := &mockNotifier{}

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 10 * time.Second,
	}, coord, notifier, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	tmr := &Timer{
		ID:        NewTimerID(),
		Name:      "notify test",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(50 * time.Millisecond),
		Task:      "notify task",
		SessionID: "session-notify",
		Channel:   "lark",
		ChatID:    "oc_chat456",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}

	if err := mgr.Add(tmr); err != nil {
		t.Fatalf("Add: %v", err)
	}

	coord.waitForCall(t, 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.lark) == 0 {
		t.Fatal("expected Lark notification")
	}
	if notifier.lark[0].ChatID != "oc_chat456" {
		t.Errorf("chatID: got %q, want %q", notifier.lark[0].ChatID, "oc_chat456")
	}
	if notifier.lark[0].Content != "timer result" {
		t.Errorf("content: got %q, want %q", notifier.lark[0].Content, "timer result")
	}
}

func TestManagerList(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(nil, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   10,
		TaskTimeout: 10 * time.Second,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	// Add timers for different users.
	for _, u := range []string{"user-a", "user-b", "user-a"} {
		tmr := &Timer{
			ID:        NewTimerID(),
			Name:      "timer-" + u,
			Type:      TimerTypeOnce,
			FireAt:    time.Now().Add(10 * time.Minute),
			Task:      "task",
			SessionID: "sess",
			UserID:    u,
			CreatedAt: time.Now().UTC(),
			Status:    StatusActive,
		}
		if err := mgr.Add(tmr); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	all := mgr.List("")
	if len(all) != 3 {
		t.Errorf("List all: got %d, want 3", len(all))
	}

	userA := mgr.List("user-a")
	if len(userA) != 2 {
		t.Errorf("List user-a: got %d, want 2", len(userA))
	}

	userB := mgr.List("user-b")
	if len(userB) != 1 {
		t.Errorf("List user-b: got %d, want 1", len(userB))
	}
}

func TestManagerDisabled(t *testing.T) {
	dir := t.TempDir()
	coord := newMockCoordinator(nil, nil)

	mgr, err := NewTimerManager(Config{
		Enabled:   false,
		StorePath: dir,
	}, coord, &mockNotifier{}, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start disabled: %v", err)
	}
	mgr.Stop()
}
