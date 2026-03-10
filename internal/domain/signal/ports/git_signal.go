// Package ports defines the port interface for git signal ingestion.
// Adapters in internal/infra/gitsignal/ implement this interface for
// specific providers (GitHub, GitLab, etc.).
package ports

import (
	"context"
	"time"

	"alex/internal/domain/signal"
)

// GitSignalProvider abstracts read access to a git hosting provider for
// leader agent features. Implementations must be safe for concurrent use.
type GitSignalProvider interface {
	// ListRecentEvents returns normalized git events since the given
	// timestamp for the configured repositories. Results are ordered by
	// timestamp ascending. Returns an empty slice (not an error) when
	// there are no events.
	ListRecentEvents(ctx context.Context, since time.Time) ([]signal.SignalEvent, error)

	// GetPRStatus returns the current status of a pull request by number
	// in the given repository (owner/repo format).
	GetPRStatus(ctx context.Context, repo string, prNumber int) (*signal.PRContext, error)

	// ListOpenPRs returns all open pull requests for the given repository.
	ListOpenPRs(ctx context.Context, repo string) ([]signal.PRContext, error)

	// DetectReviewBottlenecks scans open PRs and returns bottleneck signals
	// for PRs that have been waiting for review longer than the given threshold.
	DetectReviewBottlenecks(ctx context.Context, repo string, threshold time.Duration) ([]signal.SignalEvent, error)

	// ListCommitActivity returns commit events in the given repository
	// and branch since the given timestamp.
	ListCommitActivity(ctx context.Context, repo, branch string, since time.Time) ([]signal.SignalEvent, error)

	// Provider returns the provider name (e.g. "github", "gitlab").
	Provider() string
}
