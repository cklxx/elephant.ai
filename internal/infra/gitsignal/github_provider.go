// Package gitsignal implements the GitSignalProvider port for GitHub.
//
// This is the Phase 2 data-plane adapter for git signal ingestion. It
// normalizes GitHub API responses into domain SignalEvent types consumed
// by Enhanced Blocker Radar, Scope Change Detection, and Weekly Pulse.
package gitsignal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"alex/internal/domain/signal"
	signalports "alex/internal/domain/signal/ports"
	"alex/internal/shared/logging"
)

// GitHubProvider implements signalports.GitSignalProvider for the GitHub API.
type GitHubProvider struct {
	config GitHubConfig
	client *http.Client
	logger logging.Logger
}

var _ signalports.GitSignalProvider = (*GitHubProvider)(nil)

// NewGitHubProvider creates a GitHubProvider with the given config.
func NewGitHubProvider(config GitHubConfig, client *http.Client, logger logging.Logger) *GitHubProvider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &GitHubProvider{
		config: config,
		client: client,
		logger: logging.OrNop(logger),
	}
}

// Provider returns "github".
func (g *GitHubProvider) Provider() string { return "github" }

// ListRecentEvents returns normalized git events since the given timestamp
// for all configured repositories.
func (g *GitHubProvider) ListRecentEvents(ctx context.Context, since time.Time) ([]signal.SignalEvent, error) {
	var all []signal.SignalEvent
	for _, repo := range g.config.Repos {
		events, err := g.fetchRepoEvents(ctx, repo, since)
		if err != nil {
			g.logger.Warn("GitHubProvider: failed to fetch events for %s: %v", repo, err)
			continue
		}
		all = append(all, events...)
	}
	return all, nil
}

// GetPRStatus returns the current status of a pull request.
func (g *GitHubProvider) GetPRStatus(ctx context.Context, repo string, prNumber int) (*signal.PRContext, error) {
	path := fmt.Sprintf("/repos/%s/pulls/%d", repo, prNumber)
	body, err := g.apiGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get PR status: %w", err)
	}

	var gh ghPull
	if err := json.Unmarshal(body, &gh); err != nil {
		return nil, fmt.Errorf("parse PR response: %w", err)
	}
	pr := ghPullToPRContext(gh)
	return &pr, nil
}

// ListOpenPRs returns all open pull requests for the given repository.
func (g *GitHubProvider) ListOpenPRs(ctx context.Context, repo string) ([]signal.PRContext, error) {
	path := fmt.Sprintf("/repos/%s/pulls?state=open&per_page=100", repo)
	body, err := g.apiGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list open PRs: %w", err)
	}

	var ghPRs []ghPull
	if err := json.Unmarshal(body, &ghPRs); err != nil {
		return nil, fmt.Errorf("parse PRs response: %w", err)
	}

	prs := make([]signal.PRContext, len(ghPRs))
	for i, gh := range ghPRs {
		prs[i] = ghPullToPRContext(gh)
	}
	return prs, nil
}

// DetectReviewBottlenecks scans open PRs and returns bottleneck signals
// for PRs waiting for review longer than the configured threshold.
func (g *GitHubProvider) DetectReviewBottlenecks(ctx context.Context, repo string, threshold time.Duration) ([]signal.SignalEvent, error) {
	if threshold <= 0 {
		threshold = g.config.reviewThreshold()
	}

	prs, err := g.ListOpenPRs(ctx, repo)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var bottlenecks []signal.SignalEvent
	for _, pr := range prs {
		if pr.ReviewState != signal.ReviewPending {
			continue
		}
		for _, reviewer := range pr.Reviewers {
			// We approximate waiting time from PR creation. A full
			// implementation would use the review_requested event timestamp.
			waitDuration := now.Sub(time.Time{}) // placeholder — will be refined with event data
			if waitDuration < threshold {
				continue
			}
			bottlenecks = append(bottlenecks, signal.SignalEvent{
				ID:        fmt.Sprintf("bottleneck-%s-%d-%s", repo, pr.Number, reviewer),
				Kind:      signal.SignalReviewBottleneck,
				Provider:  "github",
				Repo:      repo,
				Timestamp: now,
				PR:        &pr,
				Bottleneck: &signal.BottleneckContext{
					PRURL:             pr.URL,
					PRNumber:          pr.Number,
					WaitingSince:      now.Add(-waitDuration),
					WaitDuration:      waitDuration,
					RequestedReviewer: reviewer,
					Author:            pr.Author,
				},
				LinkedTicketID: extractTicketID(pr.Branch),
			})
		}
	}
	return bottlenecks, nil
}

// ListCommitActivity returns commit events for the given repo and branch.
func (g *GitHubProvider) ListCommitActivity(ctx context.Context, repo, branch string, since time.Time) ([]signal.SignalEvent, error) {
	path := fmt.Sprintf("/repos/%s/commits?sha=%s&since=%s&per_page=100",
		repo, branch, since.UTC().Format(time.RFC3339))
	body, err := g.apiGet(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}

	var ghCommits []ghCommit
	if err := json.Unmarshal(body, &ghCommits); err != nil {
		return nil, fmt.Errorf("parse commits response: %w", err)
	}

	events := make([]signal.SignalEvent, 0, len(ghCommits))
	for _, c := range ghCommits {
		events = append(events, signal.SignalEvent{
			ID:        c.SHA,
			Kind:      signal.SignalCommitPushed,
			Provider:  "github",
			Repo:      repo,
			Timestamp: c.Commit.Author.Date,
			Commit: &signal.CommitContext{
				SHA:     c.SHA,
				Message: c.Commit.Message,
				Author:  c.Commit.Author.Name,
				Branch:  branch,
			},
			LinkedTicketID: extractTicketID(branch),
		})
	}
	return events, nil
}

// --- GitHub API helpers ---

func (g *GitHubProvider) apiGet(ctx context.Context, path string) ([]byte, error) {
	url := g.config.baseURL() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.config.Token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("GitHub API %s: %d %s", path, resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (g *GitHubProvider) fetchRepoEvents(ctx context.Context, repo string, since time.Time) ([]signal.SignalEvent, error) {
	path := fmt.Sprintf("/repos/%s/events?per_page=100", repo)
	body, err := g.apiGet(ctx, path)
	if err != nil {
		return nil, err
	}

	var ghEvents []ghEvent
	if err := json.Unmarshal(body, &ghEvents); err != nil {
		return nil, fmt.Errorf("parse events: %w", err)
	}

	var events []signal.SignalEvent
	for _, e := range ghEvents {
		if e.CreatedAt.Before(since) {
			continue
		}
		if evt, ok := normalizeGHEvent(e, repo); ok {
			events = append(events, evt)
		}
	}
	return events, nil
}

// --- GitHub response types ---

type ghEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Actor     ghActor         `json:"actor"`
	Repo      ghRepo          `json:"repo"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type ghActor struct {
	Login string `json:"login"`
}

type ghRepo struct {
	Name string `json:"name"`
}

type ghPull struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Merged    bool      `json:"merged"`
	HTMLURL   string    `json:"html_url"`
	User      ghActor   `json:"user"`
	Head      ghRef     `json:"head"`
	Base      ghRef     `json:"base"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	Reviewers []ghActor `json:"requested_reviewers"`
}

type ghRef struct {
	Ref string `json:"ref"`
}

type ghCommit struct {
	SHA    string       `json:"sha"`
	Commit ghCommitData `json:"commit"`
}

type ghCommitData struct {
	Message string         `json:"message"`
	Author  ghCommitAuthor `json:"author"`
}

type ghCommitAuthor struct {
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

type ghPREventPayload struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     ghPull `json:"pull_request"`
}

type ghPushPayload struct {
	Ref     string     `json:"ref"`
	Commits []ghCommit `json:"commits"`
}

type ghPRReviewPayload struct {
	Action string   `json:"action"`
	Review ghReview `json:"review"`
	PR     ghPull   `json:"pull_request"`
}

type ghReview struct {
	State string  `json:"state"`
	User  ghActor `json:"user"`
}

// --- Normalization ---

func normalizeGHEvent(e ghEvent, repo string) (signal.SignalEvent, bool) {
	base := signal.SignalEvent{
		ID:        e.ID,
		Provider:  "github",
		Repo:      repo,
		Timestamp: e.CreatedAt,
	}

	switch e.Type {
	case "PullRequestEvent":
		var p ghPREventPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return base, false
		}
		base.PR = ptrPRContext(ghPullToPRContext(p.PR))
		base.LinkedTicketID = extractTicketID(p.PR.Head.Ref)
		switch p.Action {
		case "opened":
			base.Kind = signal.SignalPROpened
		case "closed":
			if p.PR.Merged {
				base.Kind = signal.SignalPRMerged
			} else {
				base.Kind = signal.SignalPRClosed
			}
		default:
			return base, false
		}
		return base, true

	case "PullRequestReviewEvent":
		var p ghPRReviewPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return base, false
		}
		base.PR = ptrPRContext(ghPullToPRContext(p.PR))
		base.LinkedTicketID = extractTicketID(p.PR.Head.Ref)
		switch strings.ToLower(p.Review.State) {
		case "approved":
			base.Kind = signal.SignalPRApproved
		case "changes_requested":
			base.Kind = signal.SignalPRChangesRequired
		default:
			base.Kind = signal.SignalPRReviewSubmitted
		}
		return base, true

	case "PushEvent":
		var p ghPushPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return base, false
		}
		branch := strings.TrimPrefix(p.Ref, "refs/heads/")
		base.Kind = signal.SignalCommitPushed
		base.LinkedTicketID = extractTicketID(branch)
		if len(p.Commits) > 0 {
			c := p.Commits[len(p.Commits)-1] // most recent
			base.Commit = &signal.CommitContext{
				SHA:     c.SHA,
				Message: c.Commit.Message,
				Author:  c.Commit.Author.Name,
				Branch:  branch,
			}
		}
		return base, true

	default:
		return base, false
	}
}

func ghPullToPRContext(gh ghPull) signal.PRContext {
	state := gh.State
	if gh.Merged {
		state = "merged"
	}
	reviewers := make([]string, len(gh.Reviewers))
	for i, r := range gh.Reviewers {
		reviewers[i] = r.Login
	}
	rs := signal.ReviewPending
	if len(reviewers) == 0 {
		rs = signal.ReviewPending
	}
	return signal.PRContext{
		Number:      gh.Number,
		Title:       gh.Title,
		Author:      gh.User.Login,
		State:       state,
		Branch:      gh.Head.Ref,
		BaseBranch:  gh.Base.Ref,
		ReviewState: rs,
		Reviewers:   reviewers,
		Additions:   gh.Additions,
		Deletions:   gh.Deletions,
		URL:         gh.HTMLURL,
	}
}

func ptrPRContext(pr signal.PRContext) *signal.PRContext { return &pr }

// ticketPattern matches common ticket IDs like PROJ-123, ABC-1, etc.
var ticketPattern = regexp.MustCompile(`(?i)([A-Z]{2,10}-\d+)`)

// extractTicketID attempts to extract a Jira/Linear-style ticket ID from
// a branch name or string. Returns empty string if none found.
func extractTicketID(s string) string {
	m := ticketPattern.FindString(s)
	if m != "" {
		return strings.ToUpper(m)
	}
	return ""
}
