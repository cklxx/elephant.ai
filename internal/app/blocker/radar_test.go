package blocker

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
	"alex/internal/shared/notification"
)

type fakeNotifier struct {
	sent []sentMsg
}

type sentMsg struct {
	target  notification.Target
	content string
}

func (f *fakeNotifier) Send(_ context.Context, target notification.Target, content string) error {
	f.sent = append(f.sent, sentMsg{target: target, content: content})
	return nil
}

func newTestStore(t *testing.T) task.Store {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "tasks.json")
	s := taskstore.New(taskstore.WithFilePath(fp))
	t.Cleanup(func() { s.Close() })
	return s
}

func makeTask(id, desc string, status task.Status) *task.Task {
	return &task.Task{
		TaskID:      id,
		SessionID:   "s1",
		Description: desc,
		Status:      status,
		Channel:     "test",
	}
}

func TestScan_NoActiveTasks(t *testing.T) {
	store := newTestStore(t)
	r := NewRadar(store, nil, DefaultConfig())

	result, err := r.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(result.Alerts))
	}
	if result.TasksScanned != 0 {
		t.Errorf("expected 0 scanned, got %d", result.TasksScanned)
	}
}

func TestScan_StaleProgress(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "deploy service", task.StatusRunning)
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 10 * time.Minute
	r := NewRadar(store, nil, cfg)
	// Simulate time passing: set nowFunc to 20 min in the future.
	r.nowFunc = func() time.Time { return time.Now().Add(20 * time.Minute) }

	result, err := r.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(result.Alerts))
	}
	if result.Alerts[0].Reason != ReasonStaleProgress {
		t.Errorf("reason = %q, want stale_progress", result.Alerts[0].Reason)
	}
	if result.Alerts[0].Age < 10*time.Minute {
		t.Errorf("age = %v, want >= 10m", result.Alerts[0].Age)
	}
}

func TestScan_StaleProgress_NotTriggeredWhenFresh(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "deploy service", task.StatusRunning)
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Hour
	r := NewRadar(store, nil, cfg)

	result, _ := r.Scan(ctx)
	if len(result.Alerts) != 0 {
		t.Errorf("expected 0 alerts for fresh task, got %d", len(result.Alerts))
	}
}

func TestScan_WaitingInput(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "needs approval", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusWaitingInput)

	cfg := DefaultConfig()
	cfg.InputWaitThreshold = 5 * time.Minute
	r := NewRadar(store, nil, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	result, err := r.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	found := false
	for _, a := range result.Alerts {
		if a.Reason == ReasonWaitingInput {
			found = true
		}
	}
	if !found {
		t.Error("expected waiting_input alert")
	}
}

func TestScan_HasError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "flaky job", task.StatusRunning)
	_ = store.Create(ctx, tk)
	_ = store.SetError(ctx, "t1", "connection timeout")

	cfg := DefaultConfig()
	cfg.StaleThreshold = 999 * time.Hour // prevent stale alert
	r := NewRadar(store, nil, cfg)

	result, err := r.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	found := false
	for _, a := range result.Alerts {
		if a.Reason == ReasonHasError {
			found = true
			if !strings.Contains(a.Detail, "connection timeout") {
				t.Errorf("detail should contain error text, got: %s", a.Detail)
			}
		}
	}
	if !found {
		t.Error("expected has_error alert")
	}
}

func TestScan_HasError_NotTriggeredForTerminal(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "done job", task.StatusRunning)
	_ = store.Create(ctx, tk)
	_ = store.SetError(ctx, "t1", "oops")
	_ = store.SetStatus(ctx, "t1", task.StatusFailed)

	r := NewRadar(store, nil, DefaultConfig())
	result, _ := r.Scan(ctx)
	// Failed tasks are terminal → not in ListActive → no alerts.
	if len(result.Alerts) != 0 {
		t.Errorf("expected 0 alerts for terminal task, got %d", len(result.Alerts))
	}
}

func TestScan_DependencyBlocked(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	dep := makeTask("dep1", "prerequisite", task.StatusRunning)
	_ = store.Create(ctx, dep)

	tk := makeTask("t1", "waiting on dep", task.StatusPending)
	tk.DependsOn = []string{"dep1"}
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 999 * time.Hour
	r := NewRadar(store, nil, cfg)

	result, err := r.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	found := false
	for _, a := range result.Alerts {
		if a.Reason == ReasonDepBlocked && a.Task.TaskID == "t1" {
			found = true
			if !strings.Contains(a.Detail, "dep1") {
				t.Errorf("detail should mention dep1, got: %s", a.Detail)
			}
		}
	}
	if !found {
		t.Error("expected dependency_blocked alert for t1")
	}
}

func TestScan_DependencyResolved(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	dep := makeTask("dep1", "prerequisite", task.StatusRunning)
	_ = store.Create(ctx, dep)
	_ = store.SetStatus(ctx, "dep1", task.StatusCompleted)

	tk := makeTask("t1", "waiting on dep", task.StatusRunning)
	tk.DependsOn = []string{"dep1"}
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 999 * time.Hour
	r := NewRadar(store, nil, cfg)

	result, _ := r.Scan(ctx)
	for _, a := range result.Alerts {
		if a.Reason == ReasonDepBlocked {
			t.Error("should not alert for resolved dependency")
		}
	}
}

func TestScan_DependencyMissing(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "waiting on nonexistent", task.StatusPending)
	tk.DependsOn = []string{"nonexistent"}
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 999 * time.Hour
	r := NewRadar(store, nil, cfg)

	result, _ := r.Scan(ctx)
	found := false
	for _, a := range result.Alerts {
		if a.Reason == ReasonDepBlocked {
			found = true
		}
	}
	if !found {
		t.Error("missing dependency should trigger dep_blocked alert")
	}
}

func TestScan_MultipleAlerts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Stale running task with error.
	tk := makeTask("t1", "stuck job", task.StatusRunning)
	_ = store.Create(ctx, tk)
	_ = store.SetError(ctx, "t1", "timeout")

	// Waiting-input task.
	tk2 := makeTask("t2", "approval needed", task.StatusPending)
	_ = store.Create(ctx, tk2)
	_ = store.SetStatus(ctx, "t2", task.StatusWaitingInput)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 5 * time.Minute
	cfg.InputWaitThreshold = 5 * time.Minute
	r := NewRadar(store, nil, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	result, err := r.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.TasksScanned != 2 {
		t.Errorf("scanned = %d, want 2", result.TasksScanned)
	}
	// t1: stale + error = 2 alerts; t2: waiting_input = 1 alert.
	if len(result.Alerts) < 3 {
		t.Errorf("expected >= 3 alerts, got %d", len(result.Alerts))
	}
}

func TestFormatAlerts_Empty(t *testing.T) {
	result := &ScanResult{ScannedAt: time.Now()}
	out := FormatAlerts(result)
	if out != "" {
		t.Errorf("expected empty string for no alerts, got: %s", out)
	}
}

func TestFormatAlerts_WithData(t *testing.T) {
	result := &ScanResult{
		Alerts: []Alert{
			{
				Task:   &task.Task{TaskID: "t1", Description: "deploy", Status: task.StatusRunning},
				Reason: ReasonStaleProgress,
				Detail: "no progress for 30 minutes",
				Age:    30 * time.Minute,
			},
			{
				Task:   &task.Task{TaskID: "t2", Description: "review", Status: task.StatusWaitingInput},
				Reason: ReasonWaitingInput,
				Detail: "waiting for user input for 20 minutes",
				Age:    20 * time.Minute,
			},
		},
		ScannedAt:    time.Now(),
		TasksScanned: 5,
	}

	out := FormatAlerts(result)
	checks := []string{
		"Blocker Radar",
		"2 alert(s)",
		"5 active task(s)",
		"deploy",
		"no progress",
		"review",
		"waiting",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestSendAlerts_NoBlockers(t *testing.T) {
	store := newTestStore(t)
	notif := &fakeNotifier{}
	r := NewRadar(store, notif, DefaultConfig())

	result, err := r.SendAlerts(context.Background())
	if err != nil {
		t.Fatalf("SendAlerts: %v", err)
	}
	if len(result.Alerts) != 0 {
		t.Errorf("expected 0 alerts")
	}
	if len(notif.sent) != 0 {
		t.Error("should not send notification when no blockers")
	}
}

func TestSendAlerts_WithBlockers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "stuck task", task.StatusRunning)
	_ = store.Create(ctx, tk)

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, notif, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	result, err := r.SendAlerts(ctx)
	if err != nil {
		t.Fatalf("SendAlerts: %v", err)
	}
	if len(result.Alerts) == 0 {
		t.Fatal("expected alerts")
	}
	if len(notif.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(notif.sent))
	}
	if notif.sent[0].target.Channel != "lark" {
		t.Errorf("channel = %q, want lark", notif.sent[0].target.Channel)
	}
	if notif.sent[0].target.ChatID != "oc_test" {
		t.Errorf("chatID = %q, want oc_test", notif.sent[0].target.ChatID)
	}
	if !strings.Contains(notif.sent[0].content, "stuck task") {
		t.Error("notification should contain task description")
	}
}

func TestSendAlerts_NoNotifier(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "stuck", task.StatusRunning)
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	r := NewRadar(store, nil, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	result, err := r.SendAlerts(ctx)
	if err != nil {
		t.Fatalf("SendAlerts: %v", err)
	}
	if len(result.Alerts) == 0 {
		t.Error("expected alerts even without notifier")
	}
}

func TestConfigDerivation(t *testing.T) {
	cfg := Config{StaleThresholdSeconds: 600, InputWaitSeconds: 300}
	r := NewRadar(newTestStore(t), nil, cfg)
	if r.config.StaleThreshold != 10*time.Minute {
		t.Errorf("StaleThreshold = %v, want 10m", r.config.StaleThreshold)
	}
	if r.config.InputWaitThreshold != 5*time.Minute {
		t.Errorf("InputWaitThreshold = %v, want 5m", r.config.InputWaitThreshold)
	}
}

func TestHistory_EvictsOldestAtCapacity(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "stuck task", task.StatusRunning)
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	r := NewRadar(store, nil, cfg)

	// Run maxHistoryPerTask+20 scans so history exceeds the cap.
	for i := 0; i < maxHistoryPerTask+20; i++ {
		r.nowFunc = func() time.Time { return time.Now().Add(time.Duration(i+1) * 10 * time.Minute) }
		_, err := r.Scan(ctx)
		if err != nil {
			t.Fatalf("Scan %d: %v", i, err)
		}
	}

	histLen := r.HistoryLen("t1")
	if histLen != maxHistoryPerTask {
		t.Errorf("history len = %d, want %d (cap)", histLen, maxHistoryPerTask)
	}
}

func TestHistory_RecordedOnScan(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "stuck", task.StatusRunning)
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	r := NewRadar(store, nil, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(5 * time.Minute) }

	_, _ = r.Scan(ctx)

	if r.HistoryLen("t1") == 0 {
		t.Error("expected history to be recorded after scan with alerts")
	}
	if r.HistoryTaskCount() != 1 {
		t.Errorf("task count = %d, want 1", r.HistoryTaskCount())
	}
}

func TestHistory_NoRecordWithoutAlerts(t *testing.T) {
	store := newTestStore(t)
	r := NewRadar(store, nil, DefaultConfig())

	_, _ = r.Scan(context.Background())

	if r.HistoryTaskCount() != 0 {
		t.Errorf("expected 0 history entries for scan with no alerts, got %d", r.HistoryTaskCount())
	}
}

func TestReapStale_RemovesOldEntries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "old task", task.StatusRunning)
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	r := NewRadar(store, nil, cfg)

	// Scan at t=5min (creates alert history for t1).
	r.nowFunc = func() time.Time { return time.Now().Add(5 * time.Minute) }
	_, _ = r.Scan(ctx)
	if r.HistoryTaskCount() != 1 {
		t.Fatalf("pre-reap task count = %d, want 1", r.HistoryTaskCount())
	}

	// Reap with 31-day age — t1 was just seen, should survive.
	r.nowFunc = func() time.Time { return time.Now().Add(5 * time.Minute) }
	reaped := r.ReapStale(31 * 24 * time.Hour)
	if reaped != 0 {
		t.Errorf("reaped = %d, want 0 (recent entry)", reaped)
	}

	// Move time forward 40 days — now t1 is stale.
	r.nowFunc = func() time.Time { return time.Now().Add(40 * 24 * time.Hour) }
	reaped = r.ReapStale(30 * 24 * time.Hour)
	if reaped != 1 {
		t.Errorf("reaped = %d, want 1 (stale entry)", reaped)
	}
	if r.HistoryTaskCount() != 0 {
		t.Errorf("post-reap task count = %d, want 0", r.HistoryTaskCount())
	}
}

func TestReapStale_PreservesRecentEntries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create two tasks that will trigger alerts.
	tk1 := makeTask("t1", "old task", task.StatusRunning)
	_ = store.Create(ctx, tk1)
	tk2 := makeTask("t2", "new task", task.StatusRunning)
	_ = store.Create(ctx, tk2)

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	r := NewRadar(store, nil, cfg)

	// Scan at t=5min — both tasks get alerts.
	r.nowFunc = func() time.Time { return time.Now().Add(5 * time.Minute) }
	_, _ = r.Scan(ctx)

	// Scan again at t=35days — only t2 gets a new alert (t1 removed from store).
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted) // t1 becomes terminal
	r.nowFunc = func() time.Time { return time.Now().Add(35 * 24 * time.Hour) }
	_, _ = r.Scan(ctx)

	// Reap at t=35days with 30-day threshold — t1 is stale, t2 is recent.
	reaped := r.ReapStale(30 * 24 * time.Hour)
	if reaped != 1 {
		t.Errorf("reaped = %d, want 1 (only t1 stale)", reaped)
	}
	if r.HistoryLen("t1") != 0 {
		t.Errorf("t1 history should be gone after reap")
	}
	if r.HistoryLen("t2") == 0 {
		t.Errorf("t2 history should be preserved (recent)")
	}
}

func TestReapStale_DefaultAge(t *testing.T) {
	store := newTestStore(t)
	r := NewRadar(store, nil, DefaultConfig())

	// Verify default age is used when 0 is passed.
	reaped := r.ReapStale(0)
	if reaped != 0 {
		t.Errorf("reaped = %d on empty history", reaped)
	}
}

func TestReasonIcon(t *testing.T) {
	tests := []struct {
		reason BlockReason
		want   string
	}{
		{ReasonStaleProgress, "⏱"},
		{ReasonHasError, "⚠"},
		{ReasonWaitingInput, "⏳"},
		{ReasonDepBlocked, "🔗"},
		{BlockReason("unknown"), "!"},
	}
	for _, tt := range tests {
		if got := reasonIcon(tt.reason); got != tt.want {
			t.Errorf("reasonIcon(%q) = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

// ---------- NotifyBlockedTasks tests ----------

func TestNotifyBlockedTasks_NoAlerts(t *testing.T) {
	store := newTestStore(t)
	notif := &fakeNotifier{}
	r := NewRadar(store, notif, DefaultConfig())

	nr, err := r.NotifyBlockedTasks(context.Background())
	if err != nil {
		t.Fatalf("NotifyBlockedTasks: %v", err)
	}
	if nr.Detected != 0 || nr.Notified != 0 || nr.Suppressed != 0 {
		t.Errorf("expected all zeros, got detected=%d notified=%d suppressed=%d",
			nr.Detected, nr.Notified, nr.Suppressed)
	}
	if len(notif.sent) != 0 {
		t.Error("should not send when no alerts")
	}
}

func TestNotifyBlockedTasks_SendsPerTaskNotification(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "stuck deploy", task.StatusRunning))
	_ = store.Create(ctx, makeTask("t2", "flaky build", task.StatusRunning))
	_ = store.SetError(ctx, "t2", "build timeout")

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, notif, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	nr, err := r.NotifyBlockedTasks(ctx)
	if err != nil {
		t.Fatalf("NotifyBlockedTasks: %v", err)
	}
	// t1: stale_progress, t2: stale_progress + has_error = 3 alerts
	if nr.Detected < 3 {
		t.Errorf("detected = %d, want >= 3", nr.Detected)
	}
	if nr.Notified != nr.Detected {
		t.Errorf("notified = %d, want %d (first call, no dedup)", nr.Notified, nr.Detected)
	}
	if nr.Suppressed != 0 {
		t.Errorf("suppressed = %d, want 0", nr.Suppressed)
	}
	if len(notif.sent) != nr.Detected {
		t.Errorf("sent = %d, want %d", len(notif.sent), nr.Detected)
	}
	// Check notification content.
	for _, msg := range notif.sent {
		if msg.target.Channel != "lark" {
			t.Errorf("channel = %q, want lark", msg.target.Channel)
		}
		if !strings.Contains(msg.content, "Blocked Task Alert") {
			t.Error("notification should contain 'Blocked Task Alert'")
		}
		if !strings.Contains(msg.content, "Suggested action") {
			t.Error("notification should contain 'Suggested action'")
		}
	}
}

func TestNotifyBlockedTasks_DeduplicatesWithin24h(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "stuck", task.StatusRunning))

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, notif, cfg)

	baseTime := time.Now().Add(10 * time.Minute)
	r.nowFunc = func() time.Time { return baseTime }

	// First call: should send.
	nr1, _ := r.NotifyBlockedTasks(ctx)
	if nr1.Notified == 0 {
		t.Fatal("first call should send notifications")
	}
	if nr1.Suppressed != 0 {
		t.Errorf("first call suppressed = %d, want 0", nr1.Suppressed)
	}

	sentAfterFirst := len(notif.sent)

	// Second call at same time: should suppress.
	nr2, _ := r.NotifyBlockedTasks(ctx)
	if nr2.Notified != 0 {
		t.Errorf("second call notified = %d, want 0 (dedup)", nr2.Notified)
	}
	if nr2.Suppressed != nr2.Detected {
		t.Errorf("second call suppressed = %d, want %d", nr2.Suppressed, nr2.Detected)
	}
	if len(notif.sent) != sentAfterFirst {
		t.Error("second call should not send additional notifications")
	}
}

func TestNotifyBlockedTasks_ResendAfter24h(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "stuck", task.StatusRunning))

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, notif, cfg)

	t0 := time.Now().Add(10 * time.Minute)
	r.nowFunc = func() time.Time { return t0 }

	// First call.
	nr1, _ := r.NotifyBlockedTasks(ctx)
	if nr1.Notified == 0 {
		t.Fatal("first call should notify")
	}
	sentAfterFirst := len(notif.sent)

	// Advance 25 hours — past cooldown.
	t1 := t0.Add(25 * time.Hour)
	r.nowFunc = func() time.Time { return t1 }

	nr2, _ := r.NotifyBlockedTasks(ctx)
	if nr2.Notified == 0 {
		t.Error("should re-notify after 24h cooldown")
	}
	if len(notif.sent) <= sentAfterFirst {
		t.Error("should have sent additional notifications after cooldown")
	}
}

func TestNotifyBlockedTasks_DifferentReasonsNotDeduplicated(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "stuck with error", task.StatusRunning)
	_ = store.Create(ctx, tk)
	_ = store.SetError(ctx, "t1", "connection timeout")

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, notif, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	nr, _ := r.NotifyBlockedTasks(ctx)
	// t1 should have stale_progress + has_error = 2 distinct alerts.
	if nr.Detected < 2 {
		t.Fatalf("detected = %d, want >= 2", nr.Detected)
	}
	// Both are first-time, so both should be notified.
	if nr.Notified != nr.Detected {
		t.Errorf("notified = %d, want %d (different reasons)", nr.Notified, nr.Detected)
	}
}

func TestNotifyBlockedTasks_NoNotifier(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "stuck", task.StatusRunning))

	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	r := NewRadar(store, nil, cfg)
	r.nowFunc = func() time.Time { return time.Now().Add(10 * time.Minute) }

	nr, err := r.NotifyBlockedTasks(ctx)
	if err != nil {
		t.Fatalf("NotifyBlockedTasks: %v", err)
	}
	if nr.Notified == 0 {
		t.Error("should count as notified even without notifier")
	}
}

func TestFormatTaskNotification_Content(t *testing.T) {
	a := Alert{
		Task:   &task.Task{TaskID: "t1", Description: "deploy service", Status: task.StatusRunning},
		Reason: ReasonStaleProgress,
		Detail: "no progress for 2 hours",
		Age:    2 * time.Hour,
	}
	out := FormatTaskNotification(a)
	checks := []string{
		"Blocked Task Alert",
		"deploy service",
		"t1",
		"running",
		"no progress for 2 hours",
		"2 hours",
		"Suggested action",
		"restarting",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestFormatTaskNotification_AllReasons(t *testing.T) {
	reasons := []BlockReason{ReasonStaleProgress, ReasonHasError, ReasonWaitingInput, ReasonDepBlocked}
	for _, reason := range reasons {
		a := Alert{
			Task:   &task.Task{TaskID: "t1", Description: "test", Status: task.StatusRunning},
			Reason: reason,
			Detail: "test detail",
		}
		out := FormatTaskNotification(a)
		if !strings.Contains(out, "Suggested action") {
			t.Errorf("reason %s: missing suggested action in output", reason)
		}
	}
}

func TestSuggestAction_AllReasons(t *testing.T) {
	tests := []struct {
		reason BlockReason
		substr string
	}{
		{ReasonStaleProgress, "restarting"},
		{ReasonHasError, "Review the error"},
		{ReasonWaitingInput, "Provide the requested input"},
		{ReasonDepBlocked, "Unblock"},
		{BlockReason("unknown"), "Investigate"},
	}
	for _, tt := range tests {
		got := suggestAction(tt.reason)
		if !strings.Contains(got, tt.substr) {
			t.Errorf("suggestAction(%s) = %q, want substring %q", tt.reason, got, tt.substr)
		}
	}
}

func TestReapStaleNotifications(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "stuck", task.StatusRunning))

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.StaleThreshold = 1 * time.Minute
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"
	r := NewRadar(store, notif, cfg)

	t0 := time.Now().Add(10 * time.Minute)
	r.nowFunc = func() time.Time { return t0 }

	// Trigger notification to populate lastNotified.
	r.NotifyBlockedTasks(ctx)

	// Reap with short age — nothing old yet.
	reaped := r.ReapStaleNotifications(1 * time.Hour)
	if reaped != 0 {
		t.Errorf("reaped = %d, want 0 (recent entries)", reaped)
	}

	// Advance 8 days and reap with 7-day threshold.
	r.nowFunc = func() time.Time { return t0.Add(8 * 24 * time.Hour) }
	reaped = r.ReapStaleNotifications(7 * 24 * time.Hour)
	if reaped == 0 {
		t.Error("expected stale notification entries to be reaped")
	}

	// After reap, same task should be notified again (dedup cleared).
	r.nowFunc = func() time.Time { return t0.Add(8 * 24 * time.Hour) }
	nr, _ := r.NotifyBlockedTasks(ctx)
	if nr.Notified == 0 {
		t.Error("after reap, should be able to re-notify")
	}
}
