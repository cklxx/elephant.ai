package gitsignal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alex/internal/domain/signal"
	signalports "alex/internal/domain/signal/ports"
)

// Compile-time interface compliance assertion.
var _ signalports.GitSignalProvider = (*GitHubProvider)(nil)

func TestNewGitHubProvider_NilClient(t *testing.T) {
	p := NewGitHubProvider(DefaultGitHubConfig(), nil, nil)
	if p.client == nil {
		t.Fatal("expected default http client when nil is passed")
	}
}

func TestProvider(t *testing.T) {
	p := NewGitHubProvider(DefaultGitHubConfig(), nil, nil)
	if got := p.Provider(); got != "github" {
		t.Fatalf("expected provider=github, got %q", got)
	}
}

func TestListRecentEvents(t *testing.T) {
	now := time.Now().UTC()
	events := []ghEvent{
		{
			ID:        "1",
			Type:      "PullRequestEvent",
			Actor:     ghActor{Login: "alice"},
			CreatedAt: now.Add(-1 * time.Minute),
			Payload:   mustJSON(ghPREventPayload{Action: "opened", Number: 42, PR: ghPull{Number: 42, Title: "feat: something", User: ghActor{Login: "alice"}, Head: ghRef{Ref: "feat/PROJ-123-something"}, HTMLURL: "https://github.com/org/repo/pull/42"}}),
		},
		{
			ID:        "2",
			Type:      "PushEvent",
			Actor:     ghActor{Login: "bob"},
			CreatedAt: now.Add(-30 * time.Second),
			Payload:   mustJSON(ghPushPayload{Ref: "refs/heads/feat/PROJ-456-fix", Commits: []ghCommit{{SHA: "abc123", Commit: ghCommitData{Message: "fix bug", Author: ghCommitAuthor{Name: "bob", Date: now}}}}}),
		},
		{
			ID:        "old",
			Type:      "PushEvent",
			Actor:     ghActor{Login: "charlie"},
			CreatedAt: now.Add(-2 * time.Hour),
			Payload:   mustJSON(ghPushPayload{Ref: "refs/heads/main"}),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(events)
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{
		BaseURL: srv.URL,
		Repos:   []string{"org/repo"},
	}, srv.Client(), nil)

	got, err := p.ListRecentEvents(context.Background(), now.Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events (excluding old), got %d", len(got))
	}
	if got[0].Kind != signal.SignalPROpened {
		t.Errorf("expected first event kind=pr.opened, got %q", got[0].Kind)
	}
	if got[0].LinkedTicketID != "PROJ-123" {
		t.Errorf("expected ticket ID=PROJ-123, got %q", got[0].LinkedTicketID)
	}
	if got[1].Kind != signal.SignalCommitPushed {
		t.Errorf("expected second event kind=commit.pushed, got %q", got[1].Kind)
	}
	if got[1].LinkedTicketID != "PROJ-456" {
		t.Errorf("expected ticket ID=PROJ-456, got %q", got[1].LinkedTicketID)
	}
}

func TestGetPRStatus(t *testing.T) {
	pr := ghPull{
		Number:  10,
		Title:   "feat: new feature",
		State:   "open",
		HTMLURL: "https://github.com/org/repo/pull/10",
		User:    ghActor{Login: "alice"},
		Head:    ghRef{Ref: "feat/new-feature"},
		Base:    ghRef{Ref: "main"},
		Reviewers: []ghActor{{Login: "bob"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(pr)
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{BaseURL: srv.URL}, srv.Client(), nil)
	got, err := p.GetPRStatus(context.Background(), "org/repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Number != 10 {
		t.Errorf("expected PR number=10, got %d", got.Number)
	}
	if got.Author != "alice" {
		t.Errorf("expected author=alice, got %q", got.Author)
	}
	if len(got.Reviewers) != 1 || got.Reviewers[0] != "bob" {
		t.Errorf("expected reviewers=[bob], got %v", got.Reviewers)
	}
}

func TestListOpenPRs(t *testing.T) {
	prs := []ghPull{
		{Number: 1, Title: "PR 1", State: "open", User: ghActor{Login: "alice"}},
		{Number: 2, Title: "PR 2", State: "open", User: ghActor{Login: "bob"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(prs)
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{BaseURL: srv.URL}, srv.Client(), nil)
	got, err := p.ListOpenPRs(context.Background(), "org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(got))
	}
}

func TestListCommitActivity(t *testing.T) {
	commits := []ghCommit{
		{SHA: "abc123", Commit: ghCommitData{Message: "fix: typo", Author: ghCommitAuthor{Name: "alice", Date: time.Now()}}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(commits)
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{BaseURL: srv.URL}, srv.Client(), nil)
	got, err := p.ListCommitActivity(context.Background(), "org/repo", "main", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 commit event, got %d", len(got))
	}
	if got[0].Commit.SHA != "abc123" {
		t.Errorf("expected SHA=abc123, got %q", got[0].Commit.SHA)
	}
}

func TestDetectReviewBottlenecks(t *testing.T) {
	prs := []ghPull{
		{
			Number:    5,
			Title:     "waiting PR",
			State:     "open",
			HTMLURL:   "https://github.com/org/repo/pull/5",
			User:      ghActor{Login: "alice"},
			Reviewers: []ghActor{{Login: "bob"}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(prs)
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{
		BaseURL:                   srv.URL,
		ReviewBottleneckThreshold: 1 * time.Nanosecond, // very small threshold to trigger
	}, srv.Client(), nil)

	got, err := p.DetectReviewBottlenecks(context.Background(), "org/repo", 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 bottleneck, got %d", len(got))
	}
	if got[0].Kind != signal.SignalReviewBottleneck {
		t.Errorf("expected kind=review.bottleneck, got %q", got[0].Kind)
	}
	if got[0].Bottleneck.RequestedReviewer != "bob" {
		t.Errorf("expected reviewer=bob, got %q", got[0].Bottleneck.RequestedReviewer)
	}
}

func TestNormalizeGHEvent_PRMerged(t *testing.T) {
	payload := ghPREventPayload{
		Action: "closed",
		PR:     ghPull{Number: 7, Merged: true, Head: ghRef{Ref: "feat/ABC-99"}},
	}
	e := ghEvent{
		ID:        "123",
		Type:      "PullRequestEvent",
		CreatedAt: time.Now(),
		Payload:   mustJSON(payload),
	}
	evt, ok := normalizeGHEvent(e, "org/repo")
	if !ok {
		t.Fatal("expected normalization to succeed")
	}
	if evt.Kind != signal.SignalPRMerged {
		t.Errorf("expected kind=pr.merged, got %q", evt.Kind)
	}
	if evt.LinkedTicketID != "ABC-99" {
		t.Errorf("expected ticket=ABC-99, got %q", evt.LinkedTicketID)
	}
}

func TestNormalizeGHEvent_ReviewApproved(t *testing.T) {
	payload := ghPRReviewPayload{
		Action: "submitted",
		Review: ghReview{State: "approved", User: ghActor{Login: "reviewer"}},
		PR:     ghPull{Number: 8},
	}
	e := ghEvent{
		ID:        "456",
		Type:      "PullRequestReviewEvent",
		CreatedAt: time.Now(),
		Payload:   mustJSON(payload),
	}
	evt, ok := normalizeGHEvent(e, "org/repo")
	if !ok {
		t.Fatal("expected normalization to succeed")
	}
	if evt.Kind != signal.SignalPRApproved {
		t.Errorf("expected kind=pr.approved, got %q", evt.Kind)
	}
}

func TestNormalizeGHEvent_UnknownType(t *testing.T) {
	e := ghEvent{ID: "789", Type: "ForkEvent", CreatedAt: time.Now()}
	_, ok := normalizeGHEvent(e, "org/repo")
	if ok {
		t.Fatal("expected unknown event type to be skipped")
	}
}

func TestExtractTicketID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feat/PROJ-123-something", "PROJ-123"},
		{"fix/abc-99-bug", "ABC-99"},
		{"main", ""},
		{"JIRA-1", "JIRA-1"},
		{"no-ticket-here", ""},
	}
	for _, tt := range tests {
		got := extractTicketID(tt.input)
		if got != tt.want {
			t.Errorf("extractTicketID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAPIGet_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{BaseURL: srv.URL}, srv.Client(), nil)
	_, err := p.apiGet(context.Background(), "/repos/org/repo/pulls")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestGhPullToPRContext_MergedState(t *testing.T) {
	pr := ghPull{
		Number:  1,
		State:   "closed",
		Merged:  true,
		User:    ghActor{Login: "alice"},
		Head:    ghRef{Ref: "feat/x"},
		Base:    ghRef{Ref: "main"},
	}
	ctx := ghPullToPRContext(pr)
	if ctx.State != "merged" {
		t.Errorf("expected state=merged, got %q", ctx.State)
	}
}

func TestDefaultGitHubConfig(t *testing.T) {
	cfg := DefaultGitHubConfig()
	if cfg.BaseURL != "https://api.github.com" {
		t.Errorf("expected default base URL, got %q", cfg.BaseURL)
	}
	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("expected 5m poll interval, got %v", cfg.PollInterval)
	}
	if cfg.ReviewBottleneckThreshold != 24*time.Hour {
		t.Errorf("expected 24h bottleneck threshold, got %v", cfg.ReviewBottleneckThreshold)
	}
}

func mustJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(b)
}
