package blocker

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/signal"
	"alex/internal/domain/task"
	"alex/internal/testutil"
)

// ---------- checkDependencies: terminal cache hit for non-terminal dep ----------

func TestCheckDependencies_TerminalCacheHit_NonTerminal(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	// dep1 is running (non-terminal) but NOT in active set—simulating a
	// previously-cached lookup. We exercise the terminalCache path directly.
	dep := testutil.MakeTask("dep1", "running dep", task.StatusRunning)
	_ = store.Create(ctx, dep)

	// t1 depends on dep1.
	tk := testutil.MakeTask("t1", "blocked task", task.StatusPending)
	tk.DependsOn = []string{"dep1"}
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 999 * time.Hour
	r := NewRadar(store, nil, cfg)

	// Pre-populate terminal cache: dep1 is cached as non-terminal (false).
	terminalCache := map[string]bool{"dep1": false}
	// Active set deliberately excludes dep1 to force cache path.
	activeIDs := map[string]*task.Task{}

	var alerts []Alert
	r.checkDependencies(ctx, tk, activeIDs, terminalCache, &alerts)

	if len(alerts) != 1 {
		t.Fatalf("expected 1 dep_blocked alert, got %d", len(alerts))
	}
	if alerts[0].Reason != ReasonDepBlocked {
		t.Errorf("reason = %q, want dependency_blocked", alerts[0].Reason)
	}
	if !strings.Contains(alerts[0].Detail, "dep1") {
		t.Errorf("detail should mention dep1, got: %s", alerts[0].Detail)
	}
}

// ---------- checkDependencies: terminal cache hit for completed dep ----------

func TestCheckDependencies_TerminalCacheHit_Completed(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	tk := testutil.MakeTask("t1", "depends on completed", task.StatusPending)
	tk.DependsOn = []string{"dep-done"}
	_ = store.Create(ctx, tk)

	r := NewRadar(store, nil, DefaultConfig())
	terminalCache := map[string]bool{"dep-done": true}
	activeIDs := map[string]*task.Task{}

	var alerts []Alert
	r.checkDependencies(ctx, tk, activeIDs, terminalCache, &alerts)

	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for completed cached dep, got %d", len(alerts))
	}
}

// ---------- checkError: pending task with error ----------

func TestCheckError_PendingWithError(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	tk := testutil.MakeTask("t1", "pending errored", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetError(ctx, "t1", "init failure")

	cfg := DefaultConfig()
	cfg.StaleThreshold = 999 * time.Hour
	r := NewRadar(store, nil, cfg)

	result, err := r.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	found := false
	for _, a := range result.Alerts {
		if a.Reason == ReasonHasError {
			found = true
			if !strings.Contains(a.Detail, "init failure") {
				t.Errorf("detail should contain error text, got: %s", a.Detail)
			}
		}
	}
	if !found {
		t.Error("expected has_error alert for pending task with error")
	}
}

// ---------- checkWaitingInput: fresh task below threshold ----------

func TestCheckWaitingInput_BelowThreshold(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	tk := testutil.MakeTask("t1", "just started waiting", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusWaitingInput)

	cfg := DefaultConfig()
	cfg.InputWaitThreshold = 1 * time.Hour
	cfg.StaleThreshold = 999 * time.Hour
	r := NewRadar(store, nil, cfg)
	// nowFunc at default (time.Now) — task was just updated, well below 1h.

	result, _ := r.Scan(ctx)
	for _, a := range result.Alerts {
		if a.Reason == ReasonWaitingInput {
			t.Error("should not alert waiting_input for fresh task below threshold")
		}
	}
}

// ---------- alertFromReviewBottleneck: missing PR ----------

func TestAlertFromReviewBottleneck_NilPR(t *testing.T) {
	evt := signal.SignalEvent{
		Kind: signal.SignalReviewBottleneck,
		Bottleneck: &signal.BottleneckContext{
			PRNumber: 1,
		},
		PR: nil, // nil PR
	}
	_, ok := alertFromReviewBottleneck(evt)
	if ok {
		t.Error("expected false for nil PR")
	}
}

// ---------- alertFromReviewBottleneck: missing Bottleneck ----------

func TestAlertFromReviewBottleneck_NilBottleneck(t *testing.T) {
	evt := signal.SignalEvent{
		Kind:       signal.SignalReviewBottleneck,
		Bottleneck: nil,
		PR:         &signal.PRContext{Number: 1},
	}
	_, ok := alertFromReviewBottleneck(evt)
	if ok {
		t.Error("expected false for nil Bottleneck")
	}
}

// ---------- alertFromReviewBottleneck: wrong Kind ----------

func TestAlertFromReviewBottleneck_WrongKind(t *testing.T) {
	evt := signal.SignalEvent{
		Kind:       signal.SignalCommitPushed,
		Bottleneck: &signal.BottleneckContext{PRNumber: 1},
		PR:         &signal.PRContext{Number: 1},
	}
	_, ok := alertFromReviewBottleneck(evt)
	if ok {
		t.Error("expected false for wrong Kind")
	}
}

// ---------- NotifyBlockedTasks: notifier send failure ----------

func TestNotifyBlockedTasks_SendFailure(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, testutil.MakeTask("t1", "stuck", task.StatusRunning))

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, &failingNotifier{err: fmt.Errorf("network down")}, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	nr, err := r.NotifyBlockedTasks(ctx)
	if err != nil {
		t.Fatalf("NotifyBlockedTasks should not error on send failure: %v", err)
	}
	// Alerts detected but none counted as notified (send failed, no recordNotified).
	if nr.Detected == 0 {
		t.Fatal("expected detected alerts")
	}
	if nr.Notified != 0 {
		t.Errorf("notified = %d, want 0 (send failed)", nr.Notified)
	}
}

// ---------- ReapStaleNotifications: default age ----------

func TestReapStaleNotifications_DefaultAge(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	r := NewRadar(store, nil, DefaultConfig())
	// 0 triggers default (7 days).
	reaped := r.ReapStaleNotifications(0)
	if reaped != 0 {
		t.Errorf("reaped = %d on empty map", reaped)
	}
}

// ---------- lastSeen: empty history ----------

func TestLastSeen_EmptyHistory(t *testing.T) {
	h := &taskHistory{}
	got := h.lastSeen()
	if !got.IsZero() {
		t.Errorf("lastSeen on empty history = %v, want zero", got)
	}
}

// ---------- checkDependencies: dep not in active, not in cache, found in store as non-completed ----------

func TestCheckDependencies_StoreLookup_NonCompleted(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	// dep1 exists in store as failed (non-terminal for dep purposes—not completed).
	dep := testutil.MakeTask("dep1", "failed dep", task.StatusPending)
	_ = store.Create(ctx, dep)
	_ = store.SetStatus(ctx, "dep1", task.StatusFailed)

	tk := testutil.MakeTask("t1", "blocked", task.StatusPending)
	tk.DependsOn = []string{"dep1"}
	_ = store.Create(ctx, tk)

	r := NewRadar(store, nil, DefaultConfig())
	terminalCache := map[string]bool{}
	activeIDs := map[string]*task.Task{}

	var alerts []Alert
	r.checkDependencies(ctx, tk, activeIDs, terminalCache, &alerts)

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	// Verify cache was populated.
	if done, ok := terminalCache["dep1"]; !ok || done {
		t.Errorf("terminalCache[dep1] = %v/%v, want false/true", done, ok)
	}
}

// ---------- checkDependencies: dep found in store as completed ----------

func TestCheckDependencies_StoreLookup_Completed(t *testing.T) {
	store := testutil.NewTestTaskStore(t)
	ctx := context.Background()

	dep := testutil.MakeTask("dep1", "done dep", task.StatusPending)
	_ = store.Create(ctx, dep)
	_ = store.SetStatus(ctx, "dep1", task.StatusCompleted)

	tk := testutil.MakeTask("t1", "ready", task.StatusPending)
	tk.DependsOn = []string{"dep1"}
	_ = store.Create(ctx, tk)

	r := NewRadar(store, nil, DefaultConfig())
	terminalCache := map[string]bool{}
	activeIDs := map[string]*task.Task{}

	var alerts []Alert
	r.checkDependencies(ctx, tk, activeIDs, terminalCache, &alerts)

	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for completed dep, got %d", len(alerts))
	}
	if done, ok := terminalCache["dep1"]; !ok || !done {
		t.Errorf("terminalCache[dep1] = %v/%v, want true/true", done, ok)
	}
}
