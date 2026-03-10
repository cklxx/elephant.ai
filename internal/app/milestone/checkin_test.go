package milestone

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

// fakeNotifier records sent messages.
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

func TestGenerateSummary_Empty(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, nil, DefaultConfig())

	sum, err := svc.GenerateSummary(context.Background())
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if len(sum.ActiveTasks) != 0 || len(sum.CompletedIn) != 0 || len(sum.FailedIn) != 0 {
		t.Error("expected empty summary for empty store")
	}
}

func TestGenerateSummary_WithTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create active tasks.
	active1 := makeTask("t1", "deploy service", task.StatusRunning)
	active1.TokensUsed = 500
	active1.CostUSD = 0.01
	_ = store.Create(ctx, active1)

	// Create completed task with CompletedAt in window.
	done := makeTask("t2", "fix bug", task.StatusCompleted)
	done.TokensUsed = 200
	done.AnswerPreview = "Fixed the null pointer"
	_ = store.Create(ctx, done)
	now := time.Now()
	done.CompletedAt = &now
	_ = store.SetStatus(ctx, "t2", task.StatusCompleted)

	// Create failed task with CompletedAt in window.
	failed := makeTask("t3", "migrate database", task.StatusFailed)
	failed.Error = "connection refused"
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "t3", task.StatusFailed)

	cfg := DefaultConfig()
	cfg.IncludeActive = true
	cfg.IncludeCompleted = true
	cfg.LookbackDuration = time.Hour

	svc := NewService(store, nil, cfg)
	sum, err := svc.GenerateSummary(ctx)
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}

	if len(sum.ActiveTasks) != 1 {
		t.Errorf("active = %d, want 1", len(sum.ActiveTasks))
	}
	if len(sum.CompletedIn) != 1 {
		t.Errorf("completed = %d, want 1", len(sum.CompletedIn))
	}
	if len(sum.FailedIn) != 1 {
		t.Errorf("failed = %d, want 1", len(sum.FailedIn))
	}
	if sum.TotalTokens < 500 {
		t.Errorf("TotalTokens = %d, want >= 500", sum.TotalTokens)
	}
}

func TestFormatSummary_Empty(t *testing.T) {
	sum := &Summary{
		Window:      time.Hour,
		GeneratedAt: time.Now(),
	}
	out := FormatSummary(sum)
	if !strings.Contains(out, "No task activity") {
		t.Errorf("expected 'No task activity' in empty summary, got:\n%s", out)
	}
}

func TestFormatSummary_WithData(t *testing.T) {
	now := time.Now()
	sum := &Summary{
		Window: time.Hour,
		ActiveTasks: []*task.Task{
			{TaskID: "t1", Description: "deploy", Status: task.StatusRunning, CurrentIteration: 3, TokensUsed: 500},
		},
		CompletedIn: []*task.Task{
			{TaskID: "t2", Description: "fix bug", AnswerPreview: "Fixed it", CompletedAt: &now},
		},
		FailedIn: []*task.Task{
			{TaskID: "t3", Description: "migrate", Error: "timeout", CompletedAt: &now},
		},
		TotalTokens:  700,
		TotalCostUSD: 0.05,
		GeneratedAt:  now,
	}

	out := FormatSummary(sum)

	checks := []string{
		"Milestone Check-in",
		"Active:** 1",
		"Completed:** 1",
		"Failed:** 1",
		"Tokens used:** 700",
		"Success rate:** 50%",
		"In Progress",
		"deploy",
		"Completed",
		"fix bug",
		"Fixed it",
		"Failed",
		"migrate",
		"timeout",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("missing %q in output:\n%s", check, out)
		}
	}
}

func TestFormatSummary_LargeWindow(t *testing.T) {
	sum := &Summary{Window: 48 * time.Hour, GeneratedAt: time.Now()}
	out := FormatSummary(sum)
	if !strings.Contains(out, "2 days") {
		t.Errorf("expected '2 days' window label, got:\n%s", out)
	}
}

func TestSendCheckin(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "test task", task.StatusRunning)
	tk.ChatID = "oc_test123"
	_ = store.Create(ctx, tk)

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test123"

	svc := NewService(store, notif, cfg)
	err := svc.SendCheckin(ctx)
	if err != nil {
		t.Fatalf("SendCheckin: %v", err)
	}

	if len(notif.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(notif.sent))
	}
	if notif.sent[0].target.Channel != "lark" {
		t.Errorf("channel = %q, want lark", notif.sent[0].target.Channel)
	}
	if notif.sent[0].target.ChatID != "oc_test123" {
		t.Errorf("chatID = %q, want oc_test123", notif.sent[0].target.ChatID)
	}
	if !strings.Contains(notif.sent[0].content, "test task") {
		t.Errorf("content should contain task description")
	}
}

func TestSendCheckin_NoNotifier(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, nil, DefaultConfig())
	err := svc.SendCheckin(context.Background())
	if err != nil {
		t.Fatalf("SendCheckin without notifier: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30 minutes"},
		{1 * time.Minute, "1 minute"},
		{1 * time.Hour, "1 hour"},
		{6 * time.Hour, "6 hours"},
		{24 * time.Hour, "1 day"},
		{72 * time.Hour, "3 days"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "he..." {
		t.Errorf("truncate = %q, want he...", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate short = %q, want hi", got)
	}
}

func TestConfigLookbackDerivation(t *testing.T) {
	cfg := Config{IntervalSeconds: 7200}
	svc := NewService(newTestStore(t), nil, cfg)
	if svc.config.LookbackDuration != 2*time.Hour {
		t.Errorf("LookbackDuration = %v, want 2h", svc.config.LookbackDuration)
	}
}

func TestGenerateSummary_ScopedByChatID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create tasks in two different chats.
	t1 := makeTask("t1", "chat-A task", task.StatusRunning)
	t1.ChatID = "chat-A"
	_ = store.Create(ctx, t1)

	t2 := makeTask("t2", "chat-B task", task.StatusRunning)
	t2.ChatID = "chat-B"
	_ = store.Create(ctx, t2)

	// Scoped to chat-A: should only see t1.
	cfg := DefaultConfig()
	cfg.ChatID = "chat-A"
	svc := NewService(store, nil, cfg)
	sum, err := svc.GenerateSummary(ctx)
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if len(sum.ActiveTasks) != 1 {
		t.Errorf("scoped active = %d, want 1", len(sum.ActiveTasks))
	}
	if len(sum.ActiveTasks) == 1 && sum.ActiveTasks[0].TaskID != "t1" {
		t.Errorf("scoped task = %q, want t1", sum.ActiveTasks[0].TaskID)
	}
}

func TestGenerateSummary_GlobalWhenNoChatID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	t1 := makeTask("t1", "task A", task.StatusRunning)
	t1.ChatID = "chat-A"
	_ = store.Create(ctx, t1)

	t2 := makeTask("t2", "task B", task.StatusRunning)
	t2.ChatID = "chat-B"
	_ = store.Create(ctx, t2)

	// No ChatID: should see both.
	cfg := DefaultConfig()
	cfg.ChatID = ""
	svc := NewService(store, nil, cfg)
	sum, err := svc.GenerateSummary(ctx)
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if len(sum.ActiveTasks) != 2 {
		t.Errorf("global active = %d, want 2", len(sum.ActiveTasks))
	}
}
