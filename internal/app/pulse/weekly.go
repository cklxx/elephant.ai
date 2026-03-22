// Package pulse provides periodic digest reports for the leader agent.
//
// WeeklyPulse aggregates task completion statistics from the task store
// for the past 7 days, identifies blockers and stalled tasks, and
// generates a markdown summary.
package pulse

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"alex/internal/app/digest"
	"alex/internal/app/taskfmt"
	"alex/internal/domain/signal"
	signalports "alex/internal/domain/signal/ports"
	"alex/internal/domain/task"
	"alex/internal/shared/notification"
)

// WeeklyPulse holds the aggregated digest data for a 7-day window.
type WeeklyPulse struct {
	// Window boundaries
	From time.Time
	To   time.Time

	// Classified task lists
	Completed  []*task.Task
	InProgress []*task.Task
	Blocked    []*task.Task // failed + waiting_input

	// Key metrics
	TasksCompleted    int
	AvgCompletionTime time.Duration
	TotalTokens       int
	TotalCostUSD      float64
	SuccessRate       float64 // 0..1

	GitMetrics *GitActivityMetrics
}

// GitActivityMetrics holds the Git activity summary for the same 7-day window.
type GitActivityMetrics struct {
	PRsMerged       int
	ReviewCount     int
	CommitsPushed   int
	AvgReviewTime   time.Duration
	TopContributors []GitContributor
}

// GitContributor captures commit activity by author.
type GitContributor struct {
	Author  string
	Commits int
}

// Generator produces weekly pulse digests from a task store.
type Generator struct {
	store task.Store
	now   func() time.Time // injectable clock for testing
}

// NewGenerator creates a Generator backed by the given task store.
func NewGenerator(store task.Store) *Generator {
	return &Generator{store: store, now: time.Now}
}

// Generate builds a WeeklyPulse covering the past 7 days.
func (g *Generator) Generate(ctx context.Context) (*WeeklyPulse, error) {
	now := g.now()
	from := now.Add(-7 * 24 * time.Hour)

	pulse := &WeeklyPulse{
		From: from,
		To:   now,
	}

	// Fetch completed tasks in the window.
	completed, err := g.store.ListByStatus(ctx, task.StatusCompleted)
	if err != nil {
		return nil, fmt.Errorf("list completed: %w", err)
	}
	for _, t := range completed {
		if t.CompletedAt != nil && t.CompletedAt.After(from) {
			pulse.Completed = append(pulse.Completed, t)
		}
	}

	// Fetch failed tasks in the window (blockers).
	failed, err := g.store.ListByStatus(ctx, task.StatusFailed)
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}
	for _, t := range failed {
		if taskfmt.InWindow(t, from) {
			pulse.Blocked = append(pulse.Blocked, t)
		}
	}

	// Fetch waiting_input tasks (also blockers).
	waiting, err := g.store.ListByStatus(ctx, task.StatusWaitingInput)
	if err != nil {
		return nil, fmt.Errorf("list waiting_input: %w", err)
	}
	for _, t := range waiting {
		if taskfmt.InWindow(t, from) {
			pulse.Blocked = append(pulse.Blocked, t)
		}
	}

	// Fetch active (in-progress) tasks.
	active, err := g.store.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active: %w", err)
	}
	for _, t := range active {
		if t.Status == task.StatusRunning || t.Status == task.StatusPending {
			pulse.InProgress = append(pulse.InProgress, t)
		}
	}

	// Sort blocked by creation time descending (most recent first).
	sort.Slice(pulse.Blocked, func(i, j int) bool {
		return pulse.Blocked[i].CreatedAt.After(pulse.Blocked[j].CreatedAt)
	})

	// Compute metrics.
	pulse.TasksCompleted = len(pulse.Completed)

	var totalDuration time.Duration
	completedWithTimes := 0
	for _, t := range pulse.Completed {
		pulse.TotalTokens += t.TokensUsed
		pulse.TotalCostUSD += t.CostUSD
		if t.StartedAt != nil && t.CompletedAt != nil {
			totalDuration += t.CompletedAt.Sub(*t.StartedAt)
			completedWithTimes++
		}
	}
	for _, t := range pulse.Blocked {
		pulse.TotalTokens += t.TokensUsed
		pulse.TotalCostUSD += t.CostUSD
	}
	for _, t := range pulse.InProgress {
		pulse.TotalTokens += t.TokensUsed
		pulse.TotalCostUSD += t.CostUSD
	}

	if completedWithTimes > 0 {
		pulse.AvgCompletionTime = totalDuration / time.Duration(completedWithTimes)
	}

	totalTerminal := len(pulse.Completed) + taskfmt.CountFailed(pulse.Blocked)
	if totalTerminal > 0 {
		pulse.SuccessRate = float64(len(pulse.Completed)) / float64(totalTerminal)
	}

	return pulse, nil
}

// FormatMarkdown renders a WeeklyPulse as a markdown report.
func FormatMarkdown(p *WeeklyPulse) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Weekly Pulse (%s - %s)\n\n",
		p.From.Format("Jan 2"), p.To.Format("Jan 2")))

	// Key Metrics
	b.WriteString("## Key Metrics\n\n")
	b.WriteString(fmt.Sprintf("- **Tasks completed:** %d\n", p.TasksCompleted))
	if p.AvgCompletionTime > 0 {
		b.WriteString(fmt.Sprintf("- **Avg completion time:** %s\n", taskfmt.FormatDurationCompact(p.AvgCompletionTime)))
	}
	b.WriteString(fmt.Sprintf("- **Tokens used:** %d\n", p.TotalTokens))
	b.WriteString(fmt.Sprintf("- **Cost:** $%.4f\n", p.TotalCostUSD))
	if p.TasksCompleted > 0 || taskfmt.CountFailed(p.Blocked) > 0 {
		b.WriteString(fmt.Sprintf("- **Success rate:** %.0f%%\n", p.SuccessRate*100))
	}

	if p.GitMetrics != nil {
		b.WriteString("\n## Git Activity\n\n")
		b.WriteString(fmt.Sprintf("- **PRs merged:** %d\n", p.GitMetrics.PRsMerged))
		b.WriteString(fmt.Sprintf("- **Reviews submitted:** %d\n", p.GitMetrics.ReviewCount))
		b.WriteString(fmt.Sprintf("- **Commits pushed:** %d\n", p.GitMetrics.CommitsPushed))
		if p.GitMetrics.AvgReviewTime > 0 {
			b.WriteString(fmt.Sprintf("- **Avg review time:** %s\n", taskfmt.FormatDurationCompact(p.GitMetrics.AvgReviewTime)))
		} else {
			b.WriteString("- **Avg review time:** n/a\n")
		}
		if len(p.GitMetrics.TopContributors) == 0 {
			b.WriteString("- **Top contributors:** none\n")
		} else {
			var contributors []string
			for _, contributor := range p.GitMetrics.TopContributors {
				contributors = append(contributors, fmt.Sprintf("%s (%s)", contributor.Author, formatCommitCount(contributor.Commits)))
			}
			b.WriteString(fmt.Sprintf("- **Top contributors:** %s\n", strings.Join(contributors, ", ")))
		}
	}

	// Completed
	b.WriteString("\n## Completed\n\n")
	if len(p.Completed) == 0 {
		b.WriteString("No tasks completed this week.\n")
	} else {
		for _, t := range p.Completed {
			desc := taskfmt.TaskLabel(t)
			dur := ""
			if t.StartedAt != nil && t.CompletedAt != nil {
				dur = fmt.Sprintf(" (%s)", taskfmt.FormatDurationCompact(t.CompletedAt.Sub(*t.StartedAt)))
			}
			b.WriteString(fmt.Sprintf("- %s%s\n", taskfmt.Truncate(desc, 100), dur))
		}
	}

	// In Progress
	b.WriteString("\n## In Progress\n\n")
	if len(p.InProgress) == 0 {
		b.WriteString("No tasks in progress.\n")
	} else {
		for _, t := range p.InProgress {
			desc := taskfmt.TaskLabel(t)
			b.WriteString(fmt.Sprintf("- [%s] %s (%d tokens)\n", t.Status, taskfmt.Truncate(desc, 80), t.TokensUsed))
		}
	}

	// Blocked
	b.WriteString("\n## Blocked\n\n")
	if len(p.Blocked) == 0 {
		b.WriteString("No blocked tasks.\n")
	} else {
		for _, t := range p.Blocked {
			desc := taskfmt.TaskLabel(t)
			reason := ""
			if t.Status == task.StatusFailed && t.Error != "" {
				reason = fmt.Sprintf(" - error: %s", taskfmt.Truncate(t.Error, 100))
			} else if t.Status == task.StatusWaitingInput {
				reason = " - waiting for input"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s%s\n", t.Status, taskfmt.Truncate(desc, 80), reason))
		}
	}

	return b.String()
}

func formatCommitCount(count int) string {
	if count == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", count)
}

// WeeklyPulseSpec implements digest.DigestSpec for the weekly pulse.
// It wraps the Generator and optional git signal enrichment.
type WeeklyPulseSpec struct {
	gen    *Generator
	gitSrc signalports.GitSignalProvider
	pulse  *WeeklyPulse // stashed by Generate for Format
}

func (s *WeeklyPulseSpec) Name() string { return "Weekly Pulse" }

func (s *WeeklyPulseSpec) Generate(ctx context.Context) (*digest.Content, error) {
	pulse, err := s.gen.Generate(ctx)
	if err != nil {
		return nil, err
	}
	if s.gitSrc != nil {
		events, gErr := s.gitSrc.ListRecentEvents(ctx, pulse.From)
		if gErr != nil {
			// Best-effort: log and continue with empty git metrics.
			log.Printf("pulse: git metrics fetch failed (best-effort): %v", gErr)
			pulse.GitMetrics = &GitActivityMetrics{}
		} else {
			pulse.GitMetrics = summarizeGitActivity(events)
		}
	}
	s.pulse = pulse
	return &digest.Content{Title: "Weekly Pulse"}, nil
}

func (s *WeeklyPulseSpec) Format(_ *digest.Content) string {
	return FormatMarkdown(s.pulse)
}

// Service wraps a DigestSpec with digest.Service for scheduler integration.
type Service struct {
	digestSvc *digest.Service
	spec      *WeeklyPulseSpec

	// GitSignalSource enriches the pulse with git activity.
	// Set after construction by bootstrap.
	GitSignalSource signalports.GitSignalProvider
}

// NewService creates a pulse Service that generates and optionally sends digests.
func NewService(store task.Store, notifier notification.Notifier, channel, chatID string) *Service {
	var dsvc *digest.Service
	if notifier != nil {
		dsvc = digest.NewService(notifier, notification.Target{Channel: channel, ChatID: chatID}, nil, nil)
	}
	return &Service{
		digestSvc: dsvc,
		spec:      &WeeklyPulseSpec{gen: NewGenerator(store)},
	}
}

// GenerateAndSend produces a weekly pulse digest and delivers it via the notifier.
// If no notifier was configured, it generates but does not send.
func (s *Service) GenerateAndSend(ctx context.Context) error {
	s.spec.gitSrc = s.GitSignalSource
	if s.digestSvc == nil {
		// Generate-only mode: validate data without sending.
		_, err := s.spec.Generate(ctx)
		return err
	}
	return s.digestSvc.Run(ctx, s.spec)
}

func summarizeGitActivity(events []signal.SignalEvent) *GitActivityMetrics {
	metrics := &GitActivityMetrics{}
	if len(events) == 0 {
		return metrics
	}

	type contributorCount struct {
		author  string
		commits int
	}

	firstReviewLatencyByPR := make(map[string]time.Duration)
	commitCounts := make(map[string]int)

	for _, evt := range events {
		switch evt.Kind {
		case signal.SignalPRMerged:
			metrics.PRsMerged++
		case signal.SignalPRReviewSubmitted, signal.SignalPRApproved, signal.SignalPRChangesRequired:
			metrics.ReviewCount++
			if evt.PR == nil || evt.PR.CreatedAt.IsZero() {
				continue
			}
			latency := evt.Timestamp.Sub(evt.PR.CreatedAt)
			if latency < 0 {
				continue
			}
			key := gitPRKey(evt.Repo, evt.PR.Number)
			existing, ok := firstReviewLatencyByPR[key]
			if !ok || latency < existing {
				firstReviewLatencyByPR[key] = latency
			}
		case signal.SignalCommitPushed:
			metrics.CommitsPushed++
			if evt.Commit == nil {
				continue
			}
			author := strings.TrimSpace(evt.Commit.Author)
			if author == "" {
				continue
			}
			commitCounts[author]++
		}
	}

	var totalLatency time.Duration
	for _, latency := range firstReviewLatencyByPR {
		totalLatency += latency
	}
	if len(firstReviewLatencyByPR) > 0 {
		metrics.AvgReviewTime = totalLatency / time.Duration(len(firstReviewLatencyByPR))
	}

	contributors := make([]contributorCount, 0, len(commitCounts))
	for author, commits := range commitCounts {
		contributors = append(contributors, contributorCount{author: author, commits: commits})
	}
	sort.Slice(contributors, func(i, j int) bool {
		if contributors[i].commits == contributors[j].commits {
			return contributors[i].author < contributors[j].author
		}
		return contributors[i].commits > contributors[j].commits
	})
	for i, contributor := range contributors {
		if i >= 3 {
			break
		}
		metrics.TopContributors = append(metrics.TopContributors, GitContributor{
			Author:  contributor.author,
			Commits: contributor.commits,
		})
	}

	return metrics
}

func gitPRKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}
