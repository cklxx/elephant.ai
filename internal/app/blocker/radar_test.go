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
