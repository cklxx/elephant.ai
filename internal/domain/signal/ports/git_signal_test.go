package ports

import (
	"context"
	"testing"
	"time"

	"alex/internal/domain/signal"
)

type mockGitSignalProvider struct {
	provider string
	events   []signal.SignalEvent
	err      error
}

func (m *mockGitSignalProvider) ListRecentEvents(_ context.Context, _ time.Time) ([]signal.SignalEvent, error) {
	return m.events, m.err
}

func (m *mockGitSignalProvider) GetPRStatus(_ context.Context, _ string, _ int) (*signal.PRContext, error) {
	return nil, m.err
}

func (m *mockGitSignalProvider) ListOpenPRs(_ context.Context, _ string) ([]signal.PRContext, error) {
	return nil, m.err
}

func (m *mockGitSignalProvider) DetectReviewBottlenecks(_ context.Context, _ string, _ time.Duration) ([]signal.SignalEvent, error) {
	return nil, m.err
}

func (m *mockGitSignalProvider) ListCommitActivity(_ context.Context, _, _ string, _ time.Time) ([]signal.SignalEvent, error) {
	return nil, m.err
}

func (m *mockGitSignalProvider) Provider() string {
	return m.provider
}

func TestGitSignalProvider_Interface(t *testing.T) {
	var provider GitSignalProvider = &mockGitSignalProvider{
		provider: "github",
		events: []signal.SignalEvent{
			{
				ID:   "evt-1",
				Kind: signal.SignalPROpened,
			},
		},
	}

	if provider.Provider() != "github" {
		t.Errorf("Provider() = %q, want %q", provider.Provider(), "github")
	}

	events, err := provider.ListRecentEvents(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != signal.SignalPROpened {
		t.Errorf("Kind = %q, want %q", events[0].Kind, signal.SignalPROpened)
	}
}

func TestGitSignalProvider_EmptyResults(t *testing.T) {
	provider := &mockGitSignalProvider{
		provider: "gitlab",
	}

	events, err := provider.ListRecentEvents(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
}

func TestGitSignalProvider_AllMethods(t *testing.T) {
	provider := &mockGitSignalProvider{provider: "github"}

	ctx := context.Background()

	if _, err := provider.GetPRStatus(ctx, "org/repo", 1); err != nil {
		t.Errorf("GetPRStatus: %v", err)
	}

	if _, err := provider.ListOpenPRs(ctx, "org/repo"); err != nil {
		t.Errorf("ListOpenPRs: %v", err)
	}

	if _, err := provider.DetectReviewBottlenecks(ctx, "org/repo", time.Hour); err != nil {
		t.Errorf("DetectReviewBottlenecks: %v", err)
	}

	if _, err := provider.ListCommitActivity(ctx, "org/repo", "main", time.Now()); err != nil {
		t.Errorf("ListCommitActivity: %v", err)
	}
}
