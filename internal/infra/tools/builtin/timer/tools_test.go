package timer

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	tmr "alex/internal/timer"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"
)

func newTestManager(t *testing.T) *tmr.TimerManager {
	t.Helper()
	dir := t.TempDir()
	coord := &noopCoordinator{}
	mgr, err := tmr.NewTimerManager(tmr.Config{
		Enabled:     true,
		StorePath:   dir,
		MaxTimers:   100,
		TaskTimeout: 10 * time.Second,
	}, coord, nil, nil)
	if err != nil {
		t.Fatalf("NewTimerManager: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mgr.Stop)
	return mgr
}

type noopCoordinator struct{}

func (c *noopCoordinator) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{Answer: "ok"}, nil
}

func newCtxWithManager(mgr *tmr.TimerManager) context.Context {
	ctx := context.Background()
	ctx = shared.WithTimerManager(ctx, mgr)
	ctx = id.WithSessionID(ctx, "session-test-123")
	ctx = id.WithUserID(ctx, "user-test")
	return ctx
}

func TestSetTimerWithDelay(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	tool := NewSetTimer()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-1",
		Name: "set_timer",
		Arguments: map[string]any{
			"name":  "test reminder",
			"task":  "check the weather",
			"delay": "10m",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Timer created") {
		t.Errorf("expected 'Timer created' in content, got: %s", result.Content)
	}
	if result.Metadata["timer_id"] == nil {
		t.Error("expected timer_id in metadata")
	}
	if result.Metadata["session_id"] != "session-test-123" {
		t.Errorf("expected session_id in metadata, got: %v", result.Metadata["session_id"])
	}

	// Verify timer was actually created.
	timers := mgr.List("")
	if len(timers) != 1 {
		t.Fatalf("expected 1 timer, got %d", len(timers))
	}
	if timers[0].Task != "check the weather" {
		t.Errorf("task: got %q, want %q", timers[0].Task, "check the weather")
	}
}

func TestSetTimerWithFireAt(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	fireAt := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)

	tool := NewSetTimer()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-2",
		Name: "set_timer",
		Arguments: map[string]any{
			"name":    "absolute timer",
			"task":    "send report",
			"fire_at": fireAt,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
}

func TestSetTimerRecurring(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	tool := NewSetTimer()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-3",
		Name: "set_timer",
		Arguments: map[string]any{
			"name":     "daily check",
			"task":     "review PRs",
			"type":     "recurring",
			"schedule": "0 9 * * *",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
}

func TestSetTimerMissingParams(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	tool := NewSetTimer()

	// Missing name.
	result, _ := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-4",
		Name:      "set_timer",
		Arguments: map[string]any{"task": "do stuff", "delay": "5m"},
	})
	if result.Error == nil {
		t.Error("expected error for missing name")
	}

	// Missing task.
	result, _ = tool.Execute(ctx, ports.ToolCall{
		ID:        "call-5",
		Name:      "set_timer",
		Arguments: map[string]any{"name": "test", "delay": "5m"},
	})
	if result.Error == nil {
		t.Error("expected error for missing task")
	}

	// Missing delay/fire_at for once.
	result, _ = tool.Execute(ctx, ports.ToolCall{
		ID:        "call-6",
		Name:      "set_timer",
		Arguments: map[string]any{"name": "test", "task": "do"},
	})
	if result.Error == nil {
		t.Error("expected error for missing delay/fire_at")
	}
}

func TestSetTimerNoManager(t *testing.T) {
	ctx := context.Background() // no manager
	tool := NewSetTimer()
	result, _ := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-7",
		Name: "set_timer",
		Arguments: map[string]any{
			"name":  "test",
			"task":  "do",
			"delay": "5m",
		},
	})
	if result.Error == nil {
		t.Error("expected error when timer manager not available")
	}
}

func TestListTimers(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	// Add some timers first.
	for i, name := range []string{"alpha", "beta"} {
		tmrObj := &tmr.Timer{
			ID:        tmr.NewTimerID(),
			Name:      name,
			Type:      tmr.TimerTypeOnce,
			FireAt:    time.Now().Add(time.Duration(i+1) * time.Hour),
			Task:      "task-" + name,
			SessionID: "session-test-123",
			UserID:    "user-test",
			CreatedAt: time.Now().UTC(),
			Status:    tmr.StatusActive,
		}
		if err := mgr.Add(tmrObj); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	tool := NewListTimers()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-list",
		Name:      "list_timers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "alpha") || !strings.Contains(result.Content, "beta") {
		t.Errorf("expected timer names in content, got: %s", result.Content)
	}
	if result.Metadata["count"] != 2 {
		t.Errorf("expected count=2, got: %v", result.Metadata["count"])
	}
}

func TestListTimersEmpty(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	tool := NewListTimers()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-empty",
		Name:      "list_timers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "No active timers") {
		t.Errorf("expected 'No active timers' in content, got: %s", result.Content)
	}
}

func TestCancelTimer(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	// Add a timer.
	tmrObj := &tmr.Timer{
		ID:        tmr.NewTimerID(),
		Name:      "cancellable",
		Type:      tmr.TimerTypeOnce,
		FireAt:    time.Now().Add(time.Hour),
		Task:      "should be cancelled",
		SessionID: "session-test-123",
		UserID:    "user-test",
		CreatedAt: time.Now().UTC(),
		Status:    tmr.StatusActive,
	}
	if err := mgr.Add(tmrObj); err != nil {
		t.Fatalf("Add: %v", err)
	}

	tool := NewCancelTimer()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-cancel",
		Name: "cancel_timer",
		Arguments: map[string]any{
			"timer_id": tmrObj.ID,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Timer cancelled") {
		t.Errorf("expected 'Timer cancelled' in content, got: %s", result.Content)
	}

	// Verify.
	got, ok := mgr.Get(tmrObj.ID)
	if !ok {
		t.Fatal("timer not found")
	}
	if got.Status != tmr.StatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, tmr.StatusCancelled)
	}
}

func TestCancelTimerNotFound(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	tool := NewCancelTimer()
	result, _ := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-notfound",
		Name: "cancel_timer",
		Arguments: map[string]any{
			"timer_id": "tmr-nonexistent",
		},
	})
	if result.Error == nil {
		t.Error("expected error for non-existent timer")
	}
}

func TestCancelTimerMissingID(t *testing.T) {
	mgr := newTestManager(t)
	ctx := newCtxWithManager(mgr)

	tool := NewCancelTimer()
	result, _ := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-noid",
		Name:      "cancel_timer",
		Arguments: map[string]any{},
	})
	if result.Error == nil {
		t.Error("expected error for missing timer_id")
	}
}
