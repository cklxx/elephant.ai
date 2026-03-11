package pulse

import (
	"context"
	"testing"
	"time"

	"alex/internal/domain/signal"
	"alex/internal/domain/task"
)

// ---------- Generate: multiple completed tasks for avg duration ----------

func TestGenerate_MultipleCompletedAvgDuration(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	gen := &Generator{store: store, now: func() time.Time { return now }}

	// Task 1: took 1 hour.
	s1 := now.Add(-1 * time.Hour)
	tk1 := makeTask("t1", "fast", task.StatusPending)
	tk1.StartedAt = &s1
	tk1.TokensUsed = 100
	tk1.CostUSD = 0.01
	_ = store.Create(ctx, tk1)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	// Task 2: took 3 hours.
	s2 := now.Add(-3 * time.Hour)
	tk2 := makeTask("t2", "slow", task.StatusPending)
	tk2.StartedAt = &s2
	tk2.TokensUsed = 300
	tk2.CostUSD = 0.03
	_ = store.Create(ctx, tk2)
	_ = store.SetStatus(ctx, "t2", task.StatusCompleted)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if pulse.TasksCompleted != 2 {
		t.Errorf("TasksCompleted = %d, want 2", pulse.TasksCompleted)
	}
	// avg = (1h + 3h) / 2 = 2h
	if pulse.AvgCompletionTime < 90*time.Minute || pulse.AvgCompletionTime > 150*time.Minute {
		t.Errorf("AvgCompletionTime = %v, want ~2h", pulse.AvgCompletionTime)
	}
	if pulse.TotalTokens < 400 {
		t.Errorf("TotalTokens = %d, want >= 400", pulse.TotalTokens)
	}
	if pulse.TotalCostUSD < 0.04 {
		t.Errorf("TotalCostUSD = %f, want >= 0.04", pulse.TotalCostUSD)
	}
}

// ---------- Generate: tokens/cost aggregation across blocked + in-progress ----------

func TestGenerate_TokensCostAcrossAllCategories(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// Completed task.
	completed := makeTask("c1", "done", task.StatusPending)
	completed.TokensUsed = 100
	completed.CostUSD = 0.01
	_ = store.Create(ctx, completed)
	_ = store.SetStatus(ctx, "c1", task.StatusCompleted)

	// Failed task (blocked).
	failed := makeTask("f1", "fail", task.StatusPending)
	failed.TokensUsed = 200
	failed.CostUSD = 0.02
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "f1", task.StatusFailed)

	// Running task (in-progress).
	running := makeTask("r1", "running", task.StatusPending)
	running.TokensUsed = 300
	running.CostUSD = 0.03
	_ = store.Create(ctx, running)
	_ = store.SetStatus(ctx, "r1", task.StatusRunning)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Tokens: 100 + 200 + 300 = 600
	if pulse.TotalTokens != 600 {
		t.Errorf("TotalTokens = %d, want 600", pulse.TotalTokens)
	}
	if pulse.TotalCostUSD < 0.059 || pulse.TotalCostUSD > 0.061 {
		t.Errorf("TotalCostUSD = %f, want ~0.06", pulse.TotalCostUSD)
	}
}

// ---------- Generate: completed task with CompletedAt but no StartedAt ----------

func TestGenerate_CompletedWithoutStartedAt(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	gen := NewGenerator(store)

	// Task without StartedAt — should not contribute to AvgCompletionTime.
	tk := makeTask("t1", "no start", task.StatusPending)
	tk.TokensUsed = 50
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	pulse, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if pulse.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %d, want 1", pulse.TasksCompleted)
	}
	if pulse.AvgCompletionTime != 0 {
		t.Errorf("AvgCompletionTime = %v, want 0 (no StartedAt)", pulse.AvgCompletionTime)
	}
}

// ---------- summarizeGitActivity: nil Commit event ----------

func TestSummarizeGitActivity_NilCommit(t *testing.T) {
	events := []signal.SignalEvent{
		{
			Kind:      signal.SignalCommitPushed,
			Repo:      "org/repo",
			Timestamp: time.Now(),
			Commit:    nil, // nil Commit
		},
	}
	metrics := summarizeGitActivity(events)
	if metrics.CommitsPushed != 1 {
		t.Errorf("CommitsPushed = %d, want 1", metrics.CommitsPushed)
	}
	if len(metrics.TopContributors) != 0 {
		t.Errorf("TopContributors len = %d, want 0 (nil Commit skipped)", len(metrics.TopContributors))
	}
}

// ---------- summarizeGitActivity: nil PR in review event ----------

func TestSummarizeGitActivity_NilPRInReview(t *testing.T) {
	events := []signal.SignalEvent{
		{
			Kind:      signal.SignalPRReviewSubmitted,
			Repo:      "org/repo",
			Timestamp: time.Now(),
			PR:        nil, // nil PR
		},
	}
	metrics := summarizeGitActivity(events)
	if metrics.ReviewCount != 1 {
		t.Errorf("ReviewCount = %d, want 1", metrics.ReviewCount)
	}
	if metrics.AvgReviewTime != 0 {
		t.Errorf("AvgReviewTime = %v, want 0 (nil PR skipped)", metrics.AvgReviewTime)
	}
}

// ---------- summarizeGitActivity: negative review latency ----------

func TestSummarizeGitActivity_NegativeReviewLatency(t *testing.T) {
	createdAt := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	events := []signal.SignalEvent{
		// Review timestamp BEFORE PR creation — negative latency.
		makeReviewEvent("org/repo", 1, createdAt, createdAt.Add(-2*time.Hour), signal.SignalPRApproved),
	}
	metrics := summarizeGitActivity(events)
	if metrics.ReviewCount != 1 {
		t.Errorf("ReviewCount = %d, want 1", metrics.ReviewCount)
	}
	// Negative latency should be skipped.
	if metrics.AvgReviewTime != 0 {
		t.Errorf("AvgReviewTime = %v, want 0 (negative latency skipped)", metrics.AvgReviewTime)
	}
}

// ---------- summarizeGitActivity: empty author filtering ----------

func TestSummarizeGitActivity_EmptyAuthorFiltered(t *testing.T) {
	events := []signal.SignalEvent{
		{
			Kind:      signal.SignalCommitPushed,
			Repo:      "org/repo",
			Timestamp: time.Now(),
			Commit:    &signal.CommitContext{Author: "", SHA: "abc"},
		},
		{
			Kind:      signal.SignalCommitPushed,
			Repo:      "org/repo",
			Timestamp: time.Now(),
			Commit:    &signal.CommitContext{Author: "alice", SHA: "def"},
		},
	}
	metrics := summarizeGitActivity(events)
	if metrics.CommitsPushed != 2 {
		t.Errorf("CommitsPushed = %d, want 2", metrics.CommitsPushed)
	}
	if len(metrics.TopContributors) != 1 {
		t.Fatalf("TopContributors len = %d, want 1 (empty author filtered)", len(metrics.TopContributors))
	}
	if metrics.TopContributors[0].Author != "alice" {
		t.Errorf("top contributor = %s, want alice", metrics.TopContributors[0].Author)
	}
}

// ---------- summarizeGitActivity: empty events ----------

func TestSummarizeGitActivity_EmptyEvents(t *testing.T) {
	metrics := summarizeGitActivity(nil)
	if metrics.PRsMerged != 0 || metrics.ReviewCount != 0 || metrics.CommitsPushed != 0 {
		t.Errorf("expected all zeros for empty events, got %+v", metrics)
	}
}

// ---------- summarizeGitActivity: PRChangesRequired kind ----------

func TestSummarizeGitActivity_ChangesRequested(t *testing.T) {
	createdAt := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	events := []signal.SignalEvent{
		makeReviewEvent("org/repo", 1, createdAt, createdAt.Add(6*time.Hour), signal.SignalPRChangesRequired),
	}
	metrics := summarizeGitActivity(events)
	if metrics.ReviewCount != 1 {
		t.Errorf("ReviewCount = %d, want 1 (changes_required counts as review)", metrics.ReviewCount)
	}
	if metrics.AvgReviewTime != 6*time.Hour {
		t.Errorf("AvgReviewTime = %v, want 6h", metrics.AvgReviewTime)
	}
}
