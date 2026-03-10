// Package pulse provides periodic digest reports for the leader agent.
//
// WeeklyPulse aggregates task completion statistics from the task store
// for the past 7 days, identifies blockers and stalled tasks, and
// generates a markdown summary.
package pulse

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
		if inWindow(t, from) {
			pulse.Blocked = append(pulse.Blocked, t)
		}
	}

	// Fetch waiting_input tasks (also blockers).
	waiting, err := g.store.ListByStatus(ctx, task.StatusWaitingInput)
	if err != nil {
		return nil, fmt.Errorf("list waiting_input: %w", err)
	}
	for _, t := range waiting {
		if inWindow(t, from) {
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

	totalTerminal := len(pulse.Completed) + countFailed(pulse.Blocked)
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
		b.WriteString(fmt.Sprintf("- **Avg completion time:** %s\n", formatDuration(p.AvgCompletionTime)))
	}
	b.WriteString(fmt.Sprintf("- **Tokens used:** %d\n", p.TotalTokens))
	b.WriteString(fmt.Sprintf("- **Cost:** $%.4f\n", p.TotalCostUSD))
	if p.TasksCompleted > 0 || countFailed(p.Blocked) > 0 {
		b.WriteString(fmt.Sprintf("- **Success rate:** %.0f%%\n", p.SuccessRate*100))
	}

	if p.GitMetrics != nil {
		b.WriteString("\n## Git Activity\n\n")
		b.WriteString(fmt.Sprintf("- **PRs merged:** %d\n", p.GitMetrics.PRsMerged))
		b.WriteString(fmt.Sprintf("- **Reviews submitted:** %d\n", p.GitMetrics.ReviewCount))
		b.WriteString(fmt.Sprintf("- **Commits pushed:** %d\n", p.GitMetrics.CommitsPushed))
		if p.GitMetrics.AvgReviewTime > 0 {
			b.WriteString(fmt.Sprintf("- **Avg review time:** %s\n", formatDuration(p.GitMetrics.AvgReviewTime)))
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
			desc := taskLabel(t)
			dur := ""
			if t.StartedAt != nil && t.CompletedAt != nil {
				dur = fmt.Sprintf(" (%s)", formatDuration(t.CompletedAt.Sub(*t.StartedAt)))
			}
			b.WriteString(fmt.Sprintf("- %s%s\n", truncate(desc, 100), dur))
		}
	}

	// In Progress
	b.WriteString("\n## In Progress\n\n")
	if len(p.InProgress) == 0 {
		b.WriteString("No tasks in progress.\n")
	} else {
		for _, t := range p.InProgress {
			desc := taskLabel(t)
			b.WriteString(fmt.Sprintf("- [%s] %s (%d tokens)\n", t.Status, truncate(desc, 80), t.TokensUsed))
		}
	}

	// Blocked
	b.WriteString("\n## Blocked\n\n")
	if len(p.Blocked) == 0 {
		b.WriteString("No blocked tasks.\n")
	} else {
		for _, t := range p.Blocked {
			desc := taskLabel(t)
			reason := ""
			if t.Status == task.StatusFailed && t.Error != "" {
				reason = fmt.Sprintf(" - error: %s", truncate(t.Error, 100))
			} else if t.Status == task.StatusWaitingInput {
				reason = " - waiting for input"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s%s\n", t.Status, truncate(desc, 80), reason))
		}
	}

	return b.String()
}

// inWindow returns true if the task was updated or completed within the window.
func inWindow(t *task.Task, from time.Time) bool {
	if t.CompletedAt != nil && t.CompletedAt.After(from) {
		return true
	}
	return t.UpdatedAt.After(from)
}

// countFailed returns the number of failed tasks in a slice.
func countFailed(tasks []*task.Task) int {
	n := 0
	for _, t := range tasks {
		if t.Status == task.StatusFailed {
			n++
		}
	}
	return n
}

func taskLabel(t *task.Task) string {
	if t.Description != "" {
		return t.Description
	}
	return t.TaskID
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}

func formatCommitCount(count int) string {
	if count == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", count)
}

// Service wraps a Generator with notification delivery for scheduler integration.
type Service struct {
	gen      *Generator
	notifier notification.Notifier
	channel  string
	chatID   string

	GitSignalSource signalports.GitSignalProvider
}

// NewService creates a pulse Service that generates and optionally sends digests.
func NewService(store task.Store, notifier notification.Notifier, channel, chatID string) *Service {
	return &Service{
		gen:      NewGenerator(store),
		notifier: notifier,
		channel:  channel,
		chatID:   chatID,
	}
}

// GenerateAndSend produces a weekly pulse digest and delivers it via the notifier.
// If no notifier is configured, it returns the formatted markdown without sending.
func (s *Service) GenerateAndSend(ctx context.Context) error {
	pulse, err := s.gen.Generate(ctx)
	if err != nil {
		return fmt.Errorf("generate pulse: %w", err)
	}
	if s.GitSignalSource != nil {
		gitMetrics, err := s.generateGitMetrics(ctx, pulse.From)
		if err != nil {
			return fmt.Errorf("generate git metrics: %w", err)
		}
		pulse.GitMetrics = gitMetrics
	}

	content := FormatMarkdown(pulse)

	if s.notifier == nil {
		return nil
	}

	target := notification.Target{
		Channel: s.channel,
		ChatID:  s.chatID,
	}
	if err := s.notifier.Send(ctx, target, content); err != nil {
		return fmt.Errorf("send pulse: %w", err)
	}
	return nil
}

func (s *Service) generateGitMetrics(ctx context.Context, from time.Time) (*GitActivityMetrics, error) {
	events, err := s.GitSignalSource.ListRecentEvents(ctx, from)
	if err != nil {
		return nil, err
	}
	return summarizeGitActivity(events), nil
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
