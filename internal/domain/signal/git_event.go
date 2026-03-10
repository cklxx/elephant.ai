// Package signal defines normalized signal events consumed by leader agent
// features such as Enhanced Blocker Radar, Scope Change Detection, and
// Enhanced Weekly Pulse.
package signal

import "time"

// SignalKind classifies the type of git signal.
type SignalKind string

const (
	SignalPROpened          SignalKind = "pr.opened"
	SignalPRMerged          SignalKind = "pr.merged"
	SignalPRClosed          SignalKind = "pr.closed"
	SignalPRReviewRequested SignalKind = "pr.review_requested"
	SignalPRReviewSubmitted SignalKind = "pr.review_submitted"
	SignalPRChangesRequired SignalKind = "pr.changes_requested"
	SignalPRApproved        SignalKind = "pr.approved"
	SignalCommitPushed      SignalKind = "commit.pushed"
	SignalBranchCreated     SignalKind = "branch.created"
	SignalBranchDeleted     SignalKind = "branch.deleted"
	SignalReviewBottleneck  SignalKind = "review.bottleneck"
)

// ReviewState represents the current state of a PR review.
type ReviewState string

const (
	ReviewPending          ReviewState = "pending"
	ReviewApproved         ReviewState = "approved"
	ReviewChangesRequested ReviewState = "changes_requested"
	ReviewCommented        ReviewState = "commented"
	ReviewDismissed        ReviewState = "dismissed"
)

// SignalEvent is a normalized git event that can originate from GitHub,
// GitLab, or any other git hosting provider. It carries enough context
// for leader features to make decisions without provider-specific knowledge.
type SignalEvent struct {
	// Identity
	ID        string     `json:"id"`
	Kind      SignalKind `json:"kind"`
	Provider  string     `json:"provider"` // "github", "gitlab", etc.
	Repo      string     `json:"repo"`     // "owner/repo"
	Timestamp time.Time  `json:"timestamp"`

	// PR context (populated for PR-related signals)
	PR *PRContext `json:"pr,omitempty"`

	// Commit context (populated for commit signals)
	Commit *CommitContext `json:"commit,omitempty"`

	// Review bottleneck context (populated for review.bottleneck signals)
	Bottleneck *BottleneckContext `json:"bottleneck,omitempty"`

	// Linked ticket ID from branch name or PR description (e.g. "PROJ-123")
	LinkedTicketID string `json:"linked_ticket_id,omitempty"`

	// Raw provider-specific metadata for extensibility
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PRContext carries pull request details.
type PRContext struct {
	Number      int         `json:"number"`
	Title       string      `json:"title"`
	Author      string      `json:"author"`
	State       string      `json:"state"` // "open", "closed", "merged"
	Branch      string      `json:"branch"`
	BaseBranch  string      `json:"base_branch"`
	ReviewState ReviewState `json:"review_state"`
	Reviewers   []string    `json:"reviewers,omitempty"`
	Additions   int         `json:"additions"`
	Deletions   int         `json:"deletions"`
	URL         string      `json:"url"`
	CreatedAt   time.Time   `json:"created_at,omitempty"`
	UpdatedAt   time.Time   `json:"updated_at,omitempty"`
}

// CommitContext carries commit details.
type CommitContext struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Branch  string `json:"branch"`
}

// BottleneckContext describes a review bottleneck condition.
type BottleneckContext struct {
	PRURL             string        `json:"pr_url"`
	PRNumber          int           `json:"pr_number"`
	WaitingSince      time.Time     `json:"waiting_since"`
	WaitDuration      time.Duration `json:"wait_duration"`
	RequestedReviewer string        `json:"requested_reviewer"`
	Author            string        `json:"author"`
}
