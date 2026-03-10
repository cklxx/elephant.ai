package summary

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
	"alex/internal/shared/notification"
)

func newTestStore(t *testing.T) task.Store {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "tasks.json")
	s := taskstore.New(taskstore.WithFilePath(fp))
	t.Cleanup(func() { s.Close() })
	return s
}

func makeTask(id, desc string, status task.Status, agentType string) *task.Task {
	now := time.Now()
	return &task.Task{
		TaskID:      id,
		SessionID:   "s1",
		Description: desc,
		Status:      status,
		Channel:     "test",
		AgentType:   agentType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// --- Generation tests ---

func TestGenerate_EmptyStore(t *testing.T) {
	store := newTestStore(t)
	gen := NewGenerator(store)
	s, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.New) != 0 || len(s.Completed) != 0 || len(s.InProgress) != 0 || len(s.Blocked) != 0 {
		t.Error("expected all lists empty for empty store")
	}
	if s.CompletionRate != 0 {
		t.Errorf("CompletionRate = %f, want 0", s.CompletionRate)
	}
}

func TestGenerate_NewTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	tk := makeTask("t1", "setup CI", task.StatusPending, "claude_code")
	_ = store.Create(ctx, tk)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.New) != 1 {
		t.Errorf("New = %d, want 1", len(s.New))
	}
}

func TestGenerate_CompletedTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	now := time.Now()
	started := now.Add(-2 * time.Hour)
	tk := makeTask("t1", "deploy service", task.StatusPending, "internal")
	tk.StartedAt = &started
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.Completed) != 1 {
		t.Errorf("Completed = %d, want 1", len(s.Completed))
	}
}

func TestGenerate_BlockedTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	failed := makeTask("t1", "migrate db", task.StatusPending, "codex")
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "t1", task.StatusFailed)

	waiting := makeTask("t2", "review PR", task.StatusPending, "internal")
	_ = store.Create(ctx, waiting)
	_ = store.SetStatus(ctx, "t2", task.StatusWaitingInput)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.Blocked) != 2 {
		t.Errorf("Blocked = %d, want 2", len(s.Blocked))
	}
}

func TestGenerate_InProgressTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	running := makeTask("t1", "build binary", task.StatusPending, "claude_code")
	_ = store.Create(ctx, running)
	_ = store.SetStatus(ctx, "t1", task.StatusRunning)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.InProgress) != 1 {
		t.Errorf("InProgress = %d, want 1", len(s.InProgress))
	}
}

func TestGenerate_CompletionRate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// 2 completed, 1 failed => 66% completion rate.
	for _, id := range []string{"c1", "c2"} {
		tk := makeTask(id, "completed "+id, task.StatusPending, "internal")
		_ = store.Create(ctx, tk)
		_ = store.SetStatus(ctx, id, task.StatusCompleted)
	}
	failed := makeTask("f1", "failed task", task.StatusPending, "internal")
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "f1", task.StatusFailed)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if s.CompletionRate < 0.66 || s.CompletionRate > 0.67 {
		t.Errorf("CompletionRate = %f, want ~0.666", s.CompletionRate)
	}
}

func TestGenerate_AvgBlockerResolveTime(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now()
	gen := &Generator{store: store, now: func() time.Time { return now }}

	started := now.Add(-1 * time.Hour)
	tk := makeTask("t1", "resolve issue", task.StatusPending, "internal")
	tk.StartedAt = &started
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if s.AvgBlockerResolveTime < 50*time.Minute || s.AvgBlockerResolveTime > 70*time.Minute {
		t.Errorf("AvgBlockerResolveTime = %v, want ~1h", s.AvgBlockerResolveTime)
	}
}

func TestGenerate_TopActiveAgents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// 3 claude_code tasks, 1 codex task.
	for i, id := range []string{"a1", "a2", "a3"} {
		tk := makeTask(id, "task "+id, task.StatusPending, "claude_code")
		_ = i
		_ = store.Create(ctx, tk)
	}
	tk := makeTask("a4", "codex task", task.StatusPending, "codex")
	_ = store.Create(ctx, tk)

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.TopActiveAgents) != 2 {
		t.Fatalf("TopActiveAgents = %d, want 2", len(s.TopActiveAgents))
	}
	if s.TopActiveAgents[0].AgentType != "claude_code" {
		t.Errorf("top agent = %q, want claude_code", s.TopActiveAgents[0].AgentType)
	}
	if s.TopActiveAgents[0].TaskCount != 3 {
		t.Errorf("top agent count = %d, want 3", s.TopActiveAgents[0].TaskCount)
	}
}

func TestGenerate_TopActiveAgentsCappedAt5(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	agents := []string{"a", "b", "c", "d", "e", "f", "g"}
	for i, agent := range agents {
		id := fmt.Sprintf("t%d", i)
		tk := makeTask(id, "task "+id, task.StatusPending, agent)
		_ = store.Create(ctx, tk)
	}

	s, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(s.TopActiveAgents) != 5 {
		t.Errorf("TopActiveAgents = %d, want 5 (capped)", len(s.TopActiveAgents))
	}
}

// --- Formatting tests ---

func TestFormatMarkdown_EmptySummary(t *testing.T) {
	s := &DailySummary{
		From: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
	}
	out := FormatMarkdown(s)

	checks := []string{
		"Daily Summary",
		"Highlights",
		"Metrics",
		"Action Items",
		"No action items",
		"Completion rate:** N/A",
		"Avg time-to-resolve:** N/A",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("missing %q in output:\n%s", check, out)
		}
	}
}

func TestFormatMarkdown_WithData(t *testing.T) {
	now := time.Now()
	started := now.Add(-30 * time.Minute)
	s := &DailySummary{
		From: now.Add(-24 * time.Hour),
		To:   now,
		New: []*task.Task{
			{TaskID: "n1", Description: "new task"},
		},
		Completed: []*task.Task{
			{TaskID: "t1", Description: "fix bug", StartedAt: &started, CompletedAt: &now},
		},
		InProgress: []*task.Task{
			{TaskID: "t2", Description: "deploy", Status: task.StatusRunning},
		},
		Blocked: []*task.Task{
			{TaskID: "t3", Description: "migrate", Status: task.StatusFailed, Error: "timeout"},
			{TaskID: "t4", Description: "review", Status: task.StatusWaitingInput},
		},
		CompletionRate:        0.5,
		AvgBlockerResolveTime: 30 * time.Minute,
		TopActiveAgents: []AgentActivity{
			{AgentType: "claude_code", TaskCount: 5},
		},
	}

	out := FormatMarkdown(s)

	checks := []string{
		"Daily Summary",
		"**1** new tasks created",
		"**1** tasks completed",
		"**1** tasks in progress",
		"**2** tasks blocked",
		"Completion rate:** 50%",
		"Avg time-to-resolve:** 30m",
		"claude_code: 5 tasks",
		"migrate",
		"timeout",
		"review",
		"waiting for input",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("missing %q in output:\n%s", check, out)
		}
	}
}

func TestFormatMarkdown_Sections(t *testing.T) {
	s := &DailySummary{
		From: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
	}
	out := FormatMarkdown(s)

	sections := []string{"## Highlights", "## Metrics", "## Action Items"}
	for _, sec := range sections {
		if !strings.Contains(out, sec) {
			t.Errorf("missing section %q in output:\n%s", sec, out)
		}
	}
}

// --- Service tests ---

type mockNotifier struct {
	sent    []string
	sendErr error
}

func (m *mockNotifier) Send(_ context.Context, _ notification.Target, content string) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, content)
	return nil
}

func TestService_GenerateAndSend(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "test task", task.StatusPending, "internal")
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	mock := &mockNotifier{}
	svc := NewService(store, mock, "lark", "chat-123")

	if err := svc.GenerateAndSend(ctx); err != nil {
		t.Fatalf("GenerateAndSend: %v", err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(mock.sent))
	}
	if !strings.Contains(mock.sent[0], "Daily Summary") {
		t.Errorf("sent content missing 'Daily Summary':\n%s", mock.sent[0])
	}
}

func TestService_GenerateAndSend_NilNotifier(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, nil, "", "")

	if err := svc.GenerateAndSend(context.Background()); err != nil {
		t.Fatalf("GenerateAndSend with nil notifier: %v", err)
	}
}

func TestService_GenerateAndSend_SendError(t *testing.T) {
	store := newTestStore(t)
	mock := &mockNotifier{sendErr: fmt.Errorf("network error")}
	svc := NewService(store, mock, "lark", "chat-123")

	err := svc.GenerateAndSend(context.Background())
	if err == nil {
		t.Fatal("expected error from send")
	}
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("error = %q, want to contain 'network error'", err.Error())
	}
}

// --- Helper tests ---

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "he..." {
		t.Errorf("truncate = %q, want he...", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate short = %q, want hi", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{90 * time.Minute, "1h30m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestTaskLabel_FallbackToID(t *testing.T) {
	tk := &task.Task{TaskID: "abc123"}
	if got := taskLabel(tk); got != "abc123" {
		t.Errorf("taskLabel = %q, want abc123", got)
	}
}
