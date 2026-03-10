package signal

import (
	"testing"
	"time"
)

func TestSignalKindConstants(t *testing.T) {
	kinds := []SignalKind{
		SignalPROpened, SignalPRMerged, SignalPRClosed,
		SignalPRReviewRequested, SignalPRReviewSubmitted,
		SignalPRChangesRequired, SignalPRApproved,
		SignalCommitPushed, SignalBranchCreated, SignalBranchDeleted,
		SignalReviewBottleneck,
	}
	seen := make(map[SignalKind]bool)
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate SignalKind: %s", k)
		}
		seen[k] = true
		if k == "" {
			t.Error("empty SignalKind")
		}
	}
}

func TestReviewStateConstants(t *testing.T) {
	states := []ReviewState{
		ReviewPending, ReviewApproved, ReviewChangesRequested,
		ReviewCommented, ReviewDismissed,
	}
	seen := make(map[ReviewState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("duplicate ReviewState: %s", s)
		}
		seen[s] = true
	}
}

func TestSignalEvent_ZeroValue(t *testing.T) {
	var e SignalEvent
	if e.ID != "" {
		t.Error("expected empty ID")
	}
	if e.Kind != "" {
		t.Error("expected empty Kind")
	}
	if e.PR != nil {
		t.Error("expected nil PR")
	}
	if e.Commit != nil {
		t.Error("expected nil Commit")
	}
	if e.Bottleneck != nil {
		t.Error("expected nil Bottleneck")
	}
}

func TestSignalEvent_WithPRContext(t *testing.T) {
	e := SignalEvent{
		ID:        "evt-1",
		Kind:      SignalPROpened,
		Provider:  "github",
		Repo:      "owner/repo",
		Timestamp: time.Now(),
		PR: &PRContext{
			Number:      42,
			Title:       "Add feature",
			Author:      "user",
			State:       "open",
			Branch:      "feat/x",
			BaseBranch:  "main",
			ReviewState: ReviewPending,
			Reviewers:   []string{"reviewer1"},
			Additions:   100,
			Deletions:   20,
		},
	}
	if e.PR.Number != 42 {
		t.Errorf("expected PR number 42, got %d", e.PR.Number)
	}
	if len(e.PR.Reviewers) != 1 {
		t.Errorf("expected 1 reviewer, got %d", len(e.PR.Reviewers))
	}
}

func TestSignalEvent_WithCommitContext(t *testing.T) {
	e := SignalEvent{
		Kind: SignalCommitPushed,
		Commit: &CommitContext{
			SHA:     "abc123",
			Message: "fix: resolve issue",
			Author:  "dev",
			Branch:  "main",
		},
	}
	if e.Commit.SHA != "abc123" {
		t.Errorf("expected abc123, got %s", e.Commit.SHA)
	}
}

func TestSignalEvent_WithBottleneckContext(t *testing.T) {
	e := SignalEvent{
		Kind: SignalReviewBottleneck,
		Bottleneck: &BottleneckContext{
			PRURL:             "https://github.com/owner/repo/pull/1",
			PRNumber:          1,
			WaitingSince:      time.Now().Add(-48 * time.Hour),
			WaitDuration:      48 * time.Hour,
			RequestedReviewer: "reviewer",
			Author:            "author",
		},
	}
	if e.Bottleneck.WaitDuration != 48*time.Hour {
		t.Errorf("expected 48h, got %v", e.Bottleneck.WaitDuration)
	}
}

func TestSignalEvent_Metadata(t *testing.T) {
	e := SignalEvent{
		Metadata: map[string]string{
			"webhook_id": "wh-123",
		},
	}
	if e.Metadata["webhook_id"] != "wh-123" {
		t.Error("metadata mismatch")
	}
}
