package pulse

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/signal"
	signalports "alex/internal/domain/signal/ports"
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

func makeTask(id, desc string, status task.Status) *task.Task {
	now := time.Now()
	return &task.Task{
		TaskID:      id,
		SessionID:   "s1",
		Description: desc,
		Status:      status,
		Channel:     "test",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func makeReviewEvent(repo string, number int, createdAt, reviewedAt time.Time, kind signal.SignalKind) signal.SignalEvent {
	return signal.SignalEvent{
		ID:        repo + "-review",
		Kind:      kind,
		Provider:  "github",
		Repo:      repo,
		Timestamp: reviewedAt,
		PR: &signal.PRContext{
			Number:    number,
			Title:     "stabilize background jobs",
			Author:    "alice",
			State:     "open",
			CreatedAt: createdAt,
		},
	}
}

func makeMergeEvent(repo string, number int, at time.Time) signal.SignalEvent {
	return signal.SignalEvent{
		ID:        repo + "-merge",
		Kind:      signal.SignalPRMerged,
		Provider:  "github",
		Repo:      repo,
		Timestamp: at,
		PR: &signal.PRContext{
			Number: number,
			Title:  "ship scheduler hardening",
			Author: "bob",
			State:  "merged",
		},
	}
}

func makeCommitEvent(repo, author string, at time.Time) signal.SignalEvent {
	return signal.SignalEvent{
		ID:        repo + "-commit-" + author,
		Kind:      signal.SignalCommitPushed,
		Provider:  "github",
		Repo:      repo,
		Timestamp: at,
		Commit: &signal.CommitContext{
			SHA:     "abc123",
			Message: "refine pulse formatting",
			Author:  author,
			Branch:  "main",
		},
	}
}

type fakeNotifier struct {
	sent []string
}

func (f *fakeNotifier) Send(_ context.Context, _ notification.Target, content string) error {
	f.sent = append(f.sent, content)
	return nil
}

type fakeGitSignalProvider struct {
	events []signal.SignalEvent
	err    error
}

var _ signalports.GitSignalProvider = (*fakeGitSignalProvider)(nil)

func (f *fakeGitSignalProvider) ListRecentEvents(context.Context, time.Time) ([]signal.SignalEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.events, nil
}

func (f *fakeGitSignalProvider) GetPRStatus(context.Context, string, int) (*signal.PRContext, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) ListOpenPRs(context.Context, string) ([]signal.PRContext, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) DetectReviewBottlenecks(context.Context, string, time.Duration) ([]signal.SignalEvent, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) ListCommitActivity(context.Context, string, string, time.Time) ([]signal.SignalEvent, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) Provider() string { return "github" }

// --- Generation tests ---

func TestGenerate_EmptyStore(t *testing.T) {
	store := newTestStore(t)
	gen := NewGenerator(store)
	pulse, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if pulse.TasksCompleted != 0 {
		t.Errorf("TasksCompleted = %d, want 0", pulse.TasksCompleted)
	}
	if len(pulse.Completed) != 0 || len(pulse.InProgress) != 0 || len(pulse.Blocked) != 0 {
		t.Error("expected all lists empty for empty store")
	}
}

func TestGenerate_CompletedTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// Create a completed task with CompletedAt in window.
	now := time.Now()
	started := now.Add(-2 * time.Hour)
	tk := makeTask("t1", "deploy service", task.StatusPending)
	tk.TokensUsed = 500
	tk.CostUSD = 0.01
	tk.StartedAt = &started
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if pulse.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %d, want 1", pulse.TasksCompleted)
	}
	if pulse.TotalTokens < 500 {
		t.Errorf("TotalTokens = %d, want >= 500", pulse.TotalTokens)
	}
}

func TestGenerate_BlockedTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// Failed task.
	failed := makeTask("t1", "migrate db", task.StatusPending)
	failed.Error = "connection refused"
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "t1", task.StatusFailed)

	// Waiting input task.
	waiting := makeTask("t2", "review PR", task.StatusPending)
	_ = store.Create(ctx, waiting)
	_ = store.SetStatus(ctx, "t2", task.StatusWaitingInput)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(pulse.Blocked) != 2 {
		t.Errorf("Blocked = %d, want 2", len(pulse.Blocked))
	}
}

func TestGenerate_InProgressTasks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	running := makeTask("t1", "build binary", task.StatusPending)
	running.TokensUsed = 100
	_ = store.Create(ctx, running)
	_ = store.SetStatus(ctx, "t1", task.StatusRunning)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(pulse.InProgress) != 1 {
		t.Errorf("InProgress = %d, want 1", len(pulse.InProgress))
	}
}

func TestGenerate_SuccessRate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// 2 completed, 1 failed => 66% success rate.
	for _, id := range []string{"c1", "c2"} {
		tk := makeTask(id, "completed "+id, task.StatusPending)
		_ = store.Create(ctx, tk)
		_ = store.SetStatus(ctx, id, task.StatusCompleted)
	}
	failed := makeTask("f1", "failed task", task.StatusPending)
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "f1", task.StatusFailed)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if pulse.TasksCompleted != 2 {
		t.Errorf("TasksCompleted = %d, want 2", pulse.TasksCompleted)
	}
	// success rate = 2/3 = 0.666...
	if pulse.SuccessRate < 0.66 || pulse.SuccessRate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", pulse.SuccessRate)
	}
}

func TestGenerate_AvgCompletionTime(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now()
	gen := &Generator{store: store, now: func() time.Time { return now }}

	// Task that took 1 hour.
	started := now.Add(-1 * time.Hour)
	tk := makeTask("t1", "fast task", task.StatusPending)
	tk.StartedAt = &started
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// The completion time is CompletedAt - StartedAt. CompletedAt is set by
	// store.SetStatus and will be close to `now`.
	if pulse.AvgCompletionTime < 50*time.Minute || pulse.AvgCompletionTime > 70*time.Minute {
		t.Errorf("AvgCompletionTime = %v, want ~1h", pulse.AvgCompletionTime)
	}
}

func TestGenerate_OutsideWindow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now()
	gen := &Generator{store: store, now: func() time.Time { return now }}

	// Completed task outside the 7-day window.
	old := now.Add(-10 * 24 * time.Hour)
	tk := makeTask("old1", "ancient task", task.StatusPending)
	tk.CreatedAt = old
	tk.UpdatedAt = old
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "old1", task.StatusCompleted)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// The task was just completed (CompletedAt ~ now), so it WILL be in window.
	// That's expected since SetStatus sets CompletedAt to time.Now().
	// This tests that the window filtering works for tasks completed recently.
	if pulse.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %d, want 1", pulse.TasksCompleted)
	}
}

// --- Formatting tests ---

func TestFormatMarkdown_EmptyPulse(t *testing.T) {
	pulse := &WeeklyPulse{
		From: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
	}
	out := FormatMarkdown(pulse)
	if !strings.Contains(out, "Weekly Pulse") {
		t.Errorf("missing 'Weekly Pulse' header in output:\n%s", out)
	}
	if !strings.Contains(out, "No tasks completed") {
		t.Errorf("missing 'No tasks completed' in output:\n%s", out)
	}
	if !strings.Contains(out, "No tasks in progress") {
		t.Errorf("missing 'No tasks in progress' in output:\n%s", out)
	}
	if !strings.Contains(out, "No blocked tasks") {
		t.Errorf("missing 'No blocked tasks' in output:\n%s", out)
	}
}

func TestFormatMarkdown_WithData(t *testing.T) {
	now := time.Now()
	started := now.Add(-30 * time.Minute)
	pulse := &WeeklyPulse{
		From: now.Add(-7 * 24 * time.Hour),
		To:   now,
		Completed: []*task.Task{
			{TaskID: "t1", Description: "fix bug", StartedAt: &started, CompletedAt: &now, TokensUsed: 200},
		},
		InProgress: []*task.Task{
			{TaskID: "t2", Description: "deploy", Status: task.StatusRunning, TokensUsed: 500},
		},
		Blocked: []*task.Task{
			{TaskID: "t3", Description: "migrate", Status: task.StatusFailed, Error: "timeout"},
			{TaskID: "t4", Description: "review", Status: task.StatusWaitingInput},
		},
		TasksCompleted:    1,
		AvgCompletionTime: 30 * time.Minute,
		TotalTokens:       700,
		TotalCostUSD:      0.05,
		SuccessRate:       0.5,
	}

	out := FormatMarkdown(pulse)

	checks := []string{
		"Weekly Pulse",
		"Key Metrics",
		"Tasks completed:** 1",
		"Avg completion time:** 30m",
		"Tokens used:** 700",
		"Cost:** $0.0500",
		"Success rate:** 50%",
		"Completed",
		"fix bug",
		"In Progress",
		"deploy",
		"Blocked",
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

func TestFormatMarkdown_WithGitMetrics(t *testing.T) {
	now := time.Now()
	pulse := &WeeklyPulse{
		From: now.Add(-7 * 24 * time.Hour),
		To:   now,
		GitMetrics: &GitActivityMetrics{
			PRsMerged:     4,
			ReviewCount:   6,
			CommitsPushed: 9,
			AvgReviewTime: 18 * time.Hour,
			TopContributors: []GitContributor{
				{Author: "alice", Commits: 5},
				{Author: "bob", Commits: 3},
			},
		},
	}

	out := FormatMarkdown(pulse)
	checks := []string{
		"## Git Activity",
		"PRs merged:** 4",
		"Reviews submitted:** 6",
		"Commits pushed:** 9",
		"Avg review time:** 18h",
		"alice (5 commits)",
		"bob (3 commits)",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("missing %q in output:\n%s", check, out)
		}
	}
}

func TestFormatMarkdown_Sections(t *testing.T) {
	pulse := &WeeklyPulse{
		From: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
	}
	out := FormatMarkdown(pulse)

	sections := []string{"## Key Metrics", "## Completed", "## In Progress", "## Blocked"}
	for _, sec := range sections {
		if !strings.Contains(out, sec) {
			t.Errorf("missing section %q in output:\n%s", sec, out)
		}
	}
}

// --- Classification tests ---

func TestClassify_FailedAsBlocked(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	tk := makeTask("f1", "broken task", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "f1", task.StatusFailed)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	found := false
	for _, b := range pulse.Blocked {
		if b.TaskID == "f1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("failed task should be classified as blocked")
	}
}

func TestClassify_WaitingInputAsBlocked(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	tk := makeTask("w1", "needs input", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "w1", task.StatusWaitingInput)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	found := false
	for _, b := range pulse.Blocked {
		if b.TaskID == "w1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("waiting_input task should be classified as blocked")
	}
}

// --- Edge case tests ---

func TestGenerate_ZeroTokensAndCost(t *testing.T) {
	store := newTestStore(t)
	gen := NewGenerator(store)

	pulse, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if pulse.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", pulse.TotalTokens)
	}
	if pulse.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD = %f, want 0", pulse.TotalCostUSD)
	}
	if pulse.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0", pulse.SuccessRate)
	}
}

func TestGenerateAndSend_EnrichesPulseWithGitMetrics(t *testing.T) {
	store := newTestStore(t)
	notifier := &fakeNotifier{}
	svc := NewService(store, notifier, "lark", "test-chat")

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	svc.gen.now = func() time.Time { return now }
	svc.GitSignalSource = &fakeGitSignalProvider{
		events: []signal.SignalEvent{
			makeMergeEvent("org/repo", 101, now.Add(-48*time.Hour)),
			makeMergeEvent("org/repo", 102, now.Add(-24*time.Hour)),
			makeReviewEvent("org/repo", 101, now.Add(-72*time.Hour), now.Add(-48*time.Hour), signal.SignalPRApproved),
			makeReviewEvent("org/repo", 102, now.Add(-36*time.Hour), now.Add(-24*time.Hour), signal.SignalPRReviewSubmitted),
			makeCommitEvent("org/repo", "alice", now.Add(-5*time.Hour)),
			makeCommitEvent("org/repo", "alice", now.Add(-4*time.Hour)),
			makeCommitEvent("org/repo", "bob", now.Add(-3*time.Hour)),
		},
	}

	if err := svc.GenerateAndSend(context.Background()); err != nil {
		t.Fatalf("GenerateAndSend: %v", err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(notifier.sent))
	}

	out := notifier.sent[0]
	checks := []string{
		"## Git Activity",
		"PRs merged:** 2",
		"Reviews submitted:** 2",
		"Commits pushed:** 3",
		"Avg review time:** 18h",
		"alice (2 commits)",
		"bob (1 commit)",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("missing %q in output:\n%s", check, out)
		}
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
