package prepbrief

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/signal"
	"alex/internal/domain/task"
	"alex/internal/domain/workitem"
	workitemports "alex/internal/domain/workitem/ports"
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

func makeTask(id, desc string, status task.Status, userID string) *task.Task {
	return &task.Task{
		TaskID:      id,
		SessionID:   "s1",
		Description: desc,
		Status:      status,
		UserID:      userID,
		Channel:     "test",
	}
}

// ---------- Generate tests ----------

func TestGenerate_Empty(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, nil, DefaultConfig())

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if brief.MemberID != "alice" {
		t.Errorf("MemberID = %q, want alice", brief.MemberID)
	}
	if len(brief.RecentWins) != 0 || len(brief.OpenItems) != 0 || len(brief.Blockers) != 0 {
		t.Error("expected empty brief for empty store")
	}
}

func TestGenerate_RecentWins(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "ship feature X", task.StatusCompleted, "alice")
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	// Another member's task — should not appear.
	other := makeTask("t2", "bob's task", task.StatusCompleted, "bob")
	_ = store.Create(ctx, other)
	_ = store.SetStatus(ctx, "t2", task.StatusCompleted)

	svc := NewService(store, nil, DefaultConfig())
	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.RecentWins) != 1 {
		t.Fatalf("RecentWins = %d, want 1", len(brief.RecentWins))
	}
	if brief.RecentWins[0].TaskID != "t1" {
		t.Errorf("wrong task in RecentWins: %s", brief.RecentWins[0].TaskID)
	}
}

func TestGenerate_OpenItems(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	running := makeTask("t1", "deploying", task.StatusRunning, "alice")
	_ = store.Create(ctx, running)

	svc := NewService(store, nil, DefaultConfig())
	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.OpenItems) != 1 {
		t.Errorf("OpenItems = %d, want 1", len(brief.OpenItems))
	}
}

func TestGenerate_BlockerFromError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "flaky job", task.StatusRunning, "alice")
	_ = store.Create(ctx, tk)
	_ = store.SetError(ctx, "t1", "connection refused")

	svc := NewService(store, nil, DefaultConfig())
	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.Blockers) != 1 {
		t.Fatalf("Blockers = %d, want 1", len(brief.Blockers))
	}
	if !strings.Contains(brief.Blockers[0].Reason, "connection refused") {
		t.Errorf("reason = %q, want connection refused", brief.Blockers[0].Reason)
	}
}

func TestGenerate_BlockerFromStaleProgress(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "stuck deploy", task.StatusRunning, "alice")
	_ = store.Create(ctx, tk)

	svc := NewService(store, nil, DefaultConfig())
	svc.nowFunc = func() time.Time { return time.Now().Add(2 * time.Hour) }

	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.Blockers) != 1 {
		t.Fatalf("Blockers = %d, want 1", len(brief.Blockers))
	}
	if !strings.Contains(brief.Blockers[0].Reason, "no progress") {
		t.Errorf("reason = %q, want 'no progress'", brief.Blockers[0].Reason)
	}
}

func TestGenerate_BlockerFromWaitingInput(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "needs approval", task.StatusPending, "alice")
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusWaitingInput)

	svc := NewService(store, nil, DefaultConfig())
	svc.nowFunc = func() time.Time { return time.Now().Add(20 * time.Minute) }

	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.Blockers) != 1 {
		t.Fatalf("Blockers = %d, want 1", len(brief.Blockers))
	}
	if !strings.Contains(brief.Blockers[0].Reason, "waiting for input") {
		t.Errorf("reason = %q, want 'waiting for input'", brief.Blockers[0].Reason)
	}
}

func TestGenerate_BlockerFromDependency(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	dep := makeTask("dep1", "prerequisite", task.StatusRunning, "alice")
	_ = store.Create(ctx, dep)

	tk := makeTask("t1", "waiting on dep", task.StatusRunning, "alice")
	tk.DependsOn = []string{"dep1"}
	_ = store.Create(ctx, tk)

	cfg := DefaultConfig()
	svc := NewService(store, nil, cfg)

	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	found := false
	for _, bl := range brief.Blockers {
		if bl.Task.TaskID == "t1" && strings.Contains(bl.Reason, "dep1") {
			found = true
		}
	}
	if !found {
		t.Error("expected dependency blocker for t1")
	}
}

func TestGenerate_MemberFilterViaMetadata(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "metadata member", task.StatusRunning, "")
	tk.Metadata = map[string]string{"member": "charlie"}
	_ = store.Create(ctx, tk)

	svc := NewService(store, nil, DefaultConfig())
	brief, err := svc.Generate(ctx, "charlie")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.OpenItems) != 1 {
		t.Errorf("OpenItems = %d, want 1 (matched via metadata)", len(brief.OpenItems))
	}
}

func TestGenerate_EmptyMemberReturnsAll(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "alice task", task.StatusRunning, "alice"))
	_ = store.Create(ctx, makeTask("t2", "bob task", task.StatusRunning, "bob"))

	svc := NewService(store, nil, DefaultConfig())
	brief, err := svc.Generate(ctx, "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.OpenItems) != 2 {
		t.Errorf("OpenItems = %d, want 2 (empty member = all)", len(brief.OpenItems))
	}
}

func TestGenerate_PendingItems(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "queued work", task.StatusPending, "alice")
	_ = store.Create(ctx, tk)

	svc := NewService(store, nil, DefaultConfig())
	brief, err := svc.Generate(ctx, "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.Pending) != 1 {
		t.Errorf("Pending = %d, want 1", len(brief.Pending))
	}
}

// ---------- Format tests ----------

func TestFormatBrief_Empty(t *testing.T) {
	brief := &Brief{
		MemberID:    "alice",
		GeneratedAt: time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC),
		Lookback:    7 * 24 * time.Hour,
	}
	out := FormatBrief(brief)
	checks := []string{
		"1:1 Prep Brief",
		"alice",
		"No completed tasks",
		"No active tasks",
		"No blockers",
		"No recent activity",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestFormatBrief_WithData(t *testing.T) {
	now := time.Now()
	brief := &Brief{
		MemberID:    "alice",
		GeneratedAt: now,
		Lookback:    7 * 24 * time.Hour,
		RecentWins: []*task.Task{
			{TaskID: "t1", Description: "shipped feature", AnswerPreview: "All tests pass"},
		},
		OpenItems: []*task.Task{
			{TaskID: "t2", Description: "deploy v2", Status: task.StatusRunning, CurrentIteration: 5},
		},
		Blockers: []Blocker{
			{Task: &task.Task{TaskID: "t3", Description: "stuck migration"}, Reason: "no progress for 2 hours"},
		},
		Pending: []*task.Task{
			{TaskID: "t4", Description: "waiting review", Status: task.StatusWaitingInput},
		},
	}

	out := FormatBrief(brief)
	checks := []string{
		"Recent Wins",
		"shipped feature",
		"All tests pass",
		"Open Items",
		"deploy v2",
		"iter 5",
		"Blockers",
		"stuck migration",
		"no progress",
		"Discussion Points",
		"blocker",
		"completion",
		"pending",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestFormatBrief_HighWIP(t *testing.T) {
	brief := &Brief{
		MemberID:    "alice",
		GeneratedAt: time.Now(),
		Lookback:    7 * 24 * time.Hour,
		OpenItems: []*task.Task{
			{TaskID: "t1", Description: "a", Status: task.StatusRunning},
			{TaskID: "t2", Description: "b", Status: task.StatusRunning},
			{TaskID: "t3", Description: "c", Status: task.StatusRunning},
			{TaskID: "t4", Description: "d", Status: task.StatusRunning},
		},
	}
	out := FormatBrief(brief)
	if !strings.Contains(out, "High WIP") {
		t.Errorf("expected high WIP warning in output:\n%s", out)
	}
}

// ---------- Discussion points ----------

func TestSuggestDiscussionPoints_Blockers(t *testing.T) {
	b := &Brief{Blockers: []Blocker{{Task: &task.Task{}, Reason: "err"}}}
	pts := suggestDiscussionPoints(b)
	if len(pts) == 0 || !strings.Contains(pts[0], "blocker") {
		t.Errorf("expected blocker discussion point, got %v", pts)
	}
}

func TestSuggestDiscussionPoints_NoActivity(t *testing.T) {
	b := &Brief{}
	pts := suggestDiscussionPoints(b)
	found := false
	for _, p := range pts {
		if strings.Contains(p, "No recent activity") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no recent activity' point, got %v", pts)
	}
}

// ---------- SendBrief tests ----------

func TestSendBrief_WithNotifier(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeTask("t1", "active task", task.StatusRunning, "alice"))

	notif := &fakeNotifier{}
	cfg := DefaultConfig()
	cfg.Channel = "lark"
	cfg.ChatID = "oc_test"

	svc := NewService(store, notif, cfg)
	brief, err := svc.SendBrief(ctx, "alice")
	if err != nil {
		t.Fatalf("SendBrief: %v", err)
	}
	if len(brief.OpenItems) != 1 {
		t.Errorf("OpenItems = %d, want 1", len(brief.OpenItems))
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
	if !strings.Contains(notif.sent[0].content, "active task") {
		t.Error("notification should contain task description")
	}
}

func TestSendBrief_NoNotifier(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store, nil, DefaultConfig())
	_, err := svc.SendBrief(context.Background(), "alice")
	if err != nil {
		t.Fatalf("SendBrief without notifier: %v", err)
	}
}

// ---------- Config tests ----------

func TestConfigLookbackDerivation(t *testing.T) {
	cfg := Config{LookbackSeconds: 86400}
	svc := NewService(newTestStore(t), nil, cfg)
	if svc.config.LookbackDuration != 24*time.Hour {
		t.Errorf("LookbackDuration = %v, want 24h", svc.config.LookbackDuration)
	}
}

func TestConfigLookbackDefault(t *testing.T) {
	cfg := Config{}
	svc := NewService(newTestStore(t), nil, cfg)
	if svc.config.LookbackDuration != 7*24*time.Hour {
		t.Errorf("LookbackDuration = %v, want 7d", svc.config.LookbackDuration)
	}
}

// ---------- Helpers ----------

func TestMatchesMember(t *testing.T) {
	tests := []struct {
		name     string
		task     *task.Task
		member   string
		expected bool
	}{
		{"empty member matches all", &task.Task{UserID: "alice"}, "", true},
		{"exact UserID match", &task.Task{UserID: "alice"}, "alice", true},
		{"case insensitive", &task.Task{UserID: "Alice"}, "alice", true},
		{"metadata match", &task.Task{Metadata: map[string]string{"member": "bob"}}, "bob", true},
		{"no match", &task.Task{UserID: "charlie"}, "dave", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesMember(tt.task, tt.member); got != tt.expected {
				t.Errorf("matchesMember = %v, want %v", got, tt.expected)
			}
		})
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
		{7 * 24 * time.Hour, "7 days"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// ---------- Mock GitSignalProvider ----------

type fakeGitSignalProvider struct {
	openPRs      map[string][]signal.PRContext          // repo -> PRs
	recentEvents []signal.SignalEvent
	listOpenErr  error
	listEventErr error
}

func (f *fakeGitSignalProvider) ListRecentEvents(_ context.Context, _ time.Time) ([]signal.SignalEvent, error) {
	return f.recentEvents, f.listEventErr
}

func (f *fakeGitSignalProvider) GetPRStatus(_ context.Context, _ string, _ int) (*signal.PRContext, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) ListOpenPRs(_ context.Context, repo string) ([]signal.PRContext, error) {
	if f.listOpenErr != nil {
		return nil, f.listOpenErr
	}
	return f.openPRs[repo], nil
}

func (f *fakeGitSignalProvider) DetectReviewBottlenecks(_ context.Context, _ string, _ time.Duration) ([]signal.SignalEvent, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) ListCommitActivity(_ context.Context, _, _ string, _ time.Time) ([]signal.SignalEvent, error) {
	return nil, nil
}

func (f *fakeGitSignalProvider) Provider() string { return "github" }

// ---------- Mock WorkItemReader ----------

type fakeWorkItemReader struct {
	items   []*workitem.WorkItem
	listErr error
}

func (f *fakeWorkItemReader) Provider() workitem.Provider { return workitem.ProviderJira }

func (f *fakeWorkItemReader) ListWorkItems(_ context.Context, _ workitemports.IssueQuery) (workitemports.ProviderIssuePage, error) {
	if f.listErr != nil {
		return workitemports.ProviderIssuePage{}, f.listErr
	}
	return workitemports.ProviderIssuePage{Items: f.items, Total: len(f.items)}, nil
}

func (f *fakeWorkItemReader) GetWorkItem(_ context.Context, _, _ string) (*workitem.WorkItem, error) {
	return nil, nil
}

func (f *fakeWorkItemReader) ListComments(_ context.Context, _ workitemports.CommentQuery) (workitemports.ProviderCommentPage, error) {
	return workitemports.ProviderCommentPage{}, nil
}

func (f *fakeWorkItemReader) ListStatusChanges(_ context.Context, _ workitemports.StatusChangeQuery) (workitemports.ProviderStatusChangePage, error) {
	return workitemports.ProviderStatusChangePage{}, nil
}

func (f *fakeWorkItemReader) ResolveWorkspaces(_ context.Context) ([]workitemports.WorkspaceRef, error) {
	return nil, nil
}

// ---------- Enrichment tests ----------

func TestGenerate_WithGitSignalOpenPRs(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.GitRepos = []string{"org/repo"}

	svc := NewService(store, nil, cfg)
	svc.GitSignalSource = &fakeGitSignalProvider{
		openPRs: map[string][]signal.PRContext{
			"org/repo": {
				{Number: 42, Title: "Add feature X", Author: "alice", State: "open", ReviewState: signal.ReviewPending, URL: "https://github.com/org/repo/pull/42"},
				{Number: 43, Title: "Fix bug Y", Author: "bob", State: "open", ReviewState: signal.ReviewApproved, URL: "https://github.com/org/repo/pull/43"},
			},
		},
	}

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.OpenPRs) != 2 {
		t.Fatalf("OpenPRs = %d, want 2", len(brief.OpenPRs))
	}
	if brief.OpenPRs[0].Number != 42 {
		t.Errorf("first PR number = %d, want 42", brief.OpenPRs[0].Number)
	}
}

func TestGenerate_WithGitSignalMergedPRs(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.GitRepos = []string{"org/repo"}

	now := time.Now()
	svc := NewService(store, nil, cfg)
	svc.nowFunc = func() time.Time { return now }
	svc.GitSignalSource = &fakeGitSignalProvider{
		openPRs: map[string][]signal.PRContext{},
		recentEvents: []signal.SignalEvent{
			{
				Kind: signal.SignalPRMerged,
				Repo: "org/repo",
				PR:   &signal.PRContext{Number: 40, Title: "Shipped auth", State: "merged", Author: "alice"},
			},
			{
				Kind: signal.SignalPROpened,
				Repo: "org/repo",
				PR:   &signal.PRContext{Number: 41, Title: "WIP", State: "open"},
			},
		},
	}

	brief, err := svc.Generate(context.Background(), "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.RecentlyMergedPRs) != 1 {
		t.Fatalf("RecentlyMergedPRs = %d, want 1", len(brief.RecentlyMergedPRs))
	}
	if brief.RecentlyMergedPRs[0].Number != 40 {
		t.Errorf("merged PR number = %d, want 40", brief.RecentlyMergedPRs[0].Number)
	}
}

func TestGenerate_GitSignalErrorIsNonFatal(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.GitRepos = []string{"org/repo"}

	svc := NewService(store, nil, cfg)
	svc.GitSignalSource = &fakeGitSignalProvider{
		listOpenErr:  fmt.Errorf("network error"),
		listEventErr: fmt.Errorf("network error"),
	}

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate should succeed despite git signal error: %v", err)
	}
	if len(brief.OpenPRs) != 0 {
		t.Errorf("OpenPRs = %d, want 0 on error", len(brief.OpenPRs))
	}
}

func TestGenerate_NoGitSignalSourceSkipsEnrichment(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.GitRepos = []string{"org/repo"}

	svc := NewService(store, nil, cfg)
	// GitSignalSource intentionally nil

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.OpenPRs) != 0 || len(brief.RecentlyMergedPRs) != 0 {
		t.Error("expected no git signal data when source is nil")
	}
}

func TestGenerate_NoGitReposSkipsEnrichment(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	// GitRepos intentionally empty

	svc := NewService(store, nil, cfg)
	svc.GitSignalSource = &fakeGitSignalProvider{
		openPRs: map[string][]signal.PRContext{
			"org/repo": {{Number: 1, Title: "should not appear"}},
		},
	}

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.OpenPRs) != 0 {
		t.Error("expected no PRs when GitRepos is empty")
	}
}

func TestGenerate_WithBlockedTickets(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.WorkItemWorkspaceID = "ws-1"

	svc := NewService(store, nil, cfg)
	svc.WorkItemSource = &fakeWorkItemReader{
		items: []*workitem.WorkItem{
			{ID: "1", Key: "PROJ-10", Title: "Blocked migration", StatusClass: workitem.StatusBlocked, IsBlocked: true, BlockedReason: "waiting on DBA"},
			{ID: "2", Key: "PROJ-11", Title: "In progress", StatusClass: workitem.StatusInProgress},
			{ID: "3", Key: "PROJ-12", Title: "Also blocked", IsBlocked: true},
		},
	}

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.BlockedTickets) != 2 {
		t.Fatalf("BlockedTickets = %d, want 2", len(brief.BlockedTickets))
	}
	if brief.BlockedTickets[0].Key != "PROJ-10" {
		t.Errorf("first blocked ticket key = %q, want PROJ-10", brief.BlockedTickets[0].Key)
	}
}

func TestGenerate_WorkItemErrorIsNonFatal(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.WorkItemWorkspaceID = "ws-1"

	svc := NewService(store, nil, cfg)
	svc.WorkItemSource = &fakeWorkItemReader{
		listErr: fmt.Errorf("jira down"),
	}

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate should succeed despite work item error: %v", err)
	}
	if len(brief.BlockedTickets) != 0 {
		t.Errorf("BlockedTickets = %d, want 0 on error", len(brief.BlockedTickets))
	}
}

func TestGenerate_NoWorkItemSourceSkipsEnrichment(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	cfg.WorkItemWorkspaceID = "ws-1"

	svc := NewService(store, nil, cfg)
	// WorkItemSource intentionally nil

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.BlockedTickets) != 0 {
		t.Error("expected no blocked tickets when source is nil")
	}
}

func TestGenerate_NoWorkspaceIDSkipsEnrichment(t *testing.T) {
	store := newTestStore(t)
	cfg := DefaultConfig()
	// WorkItemWorkspaceID intentionally empty

	svc := NewService(store, nil, cfg)
	svc.WorkItemSource = &fakeWorkItemReader{
		items: []*workitem.WorkItem{
			{ID: "1", Key: "PROJ-1", StatusClass: workitem.StatusBlocked},
		},
	}

	brief, err := svc.Generate(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(brief.BlockedTickets) != 0 {
		t.Error("expected no blocked tickets when workspace ID is empty")
	}
}

// ---------- Format enrichment tests ----------

func TestFormatBrief_WithOpenPRs(t *testing.T) {
	brief := &Brief{
		MemberID:    "alice",
		GeneratedAt: time.Now(),
		Lookback:    7 * 24 * time.Hour,
		OpenPRs: []signal.PRContext{
			{Number: 42, Title: "Add feature X", ReviewState: signal.ReviewPending, Additions: 150, Deletions: 30, URL: "https://example.com/pr/42"},
		},
	}
	out := FormatBrief(brief)
	checks := []string{"Open PRs", "#42", "Add feature X", "pending", "+150/-30"}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestFormatBrief_WithMergedPRs(t *testing.T) {
	brief := &Brief{
		MemberID:    "alice",
		GeneratedAt: time.Now(),
		Lookback:    7 * 24 * time.Hour,
		RecentlyMergedPRs: []signal.PRContext{
			{Number: 40, Title: "Shipped auth", Additions: 200, Deletions: 50, URL: "https://example.com/pr/40"},
		},
	}
	out := FormatBrief(brief)
	checks := []string{"Recently Merged PRs", "#40", "Shipped auth", "+200/-50"}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestFormatBrief_WithBlockedTickets(t *testing.T) {
	brief := &Brief{
		MemberID:    "alice",
		GeneratedAt: time.Now(),
		Lookback:    7 * 24 * time.Hour,
		BlockedTickets: []*workitem.WorkItem{
			{Key: "PROJ-10", Title: "Blocked migration", BlockedReason: "waiting on DBA", URL: "https://jira.example.com/PROJ-10"},
		},
	}
	out := FormatBrief(brief)
	checks := []string{"Blocked Tickets", "PROJ-10", "Blocked migration", "waiting on DBA"}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}
}

func TestSuggestDiscussionPoints_OpenPRs(t *testing.T) {
	b := &Brief{
		OpenPRs: []signal.PRContext{{Number: 1}},
	}
	pts := suggestDiscussionPoints(b)
	found := false
	for _, p := range pts {
		if strings.Contains(p, "open PR") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected open PR discussion point, got %v", pts)
	}
}

func TestSuggestDiscussionPoints_BlockedTickets(t *testing.T) {
	b := &Brief{
		BlockedTickets: []*workitem.WorkItem{{Key: "X-1"}},
	}
	pts := suggestDiscussionPoints(b)
	found := false
	for _, p := range pts {
		if strings.Contains(p, "blocked ticket") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected blocked ticket discussion point, got %v", pts)
	}
}
