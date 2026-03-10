// Package summary provides daily activity digest reports for the leader agent.
//
// DailySummary aggregates task activity from the past 24 hours,
// calculates key metrics, and generates a concise markdown digest.
package summary

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"alex/internal/domain/task"
	"alex/internal/shared/notification"
)

// DailySummary holds the aggregated digest data for a 24-hour window.
type DailySummary struct {
	// Window boundaries
	From time.Time
	To   time.Time

	// Classified task lists
	New        []*task.Task
	Completed  []*task.Task
	InProgress []*task.Task
	Blocked    []*task.Task // failed + waiting_input

	// Key metrics
	CompletionRate        float64       // completed / (completed + failed) in 0..1
	AvgBlockerResolveTime time.Duration // avg time from blocked to resolved
	TopActiveAgents       []AgentActivity
}

// AgentActivity tracks task counts per agent type.
type AgentActivity struct {
	AgentType string
	TaskCount int
}

// Generator produces daily summary digests from a task store.
type Generator struct {
	store task.Store
	now   func() time.Time // injectable clock for testing
}

// NewGenerator creates a Generator backed by the given task store.
func NewGenerator(store task.Store) *Generator {
	return &Generator{store: store, now: time.Now}
}

// Generate builds a DailySummary covering the past 24 hours.
func (g *Generator) Generate(ctx context.Context) (*DailySummary, error) {
	now := g.now()
	from := now.Add(-24 * time.Hour)

	s := &DailySummary{
		From: from,
		To:   now,
	}

	// Fetch completed tasks in window.
	completed, err := g.store.ListByStatus(ctx, task.StatusCompleted)
	if err != nil {
		return nil, fmt.Errorf("list completed: %w", err)
	}
	for _, t := range completed {
		if t.CompletedAt != nil && t.CompletedAt.After(from) {
			s.Completed = append(s.Completed, t)
		}
	}

	// Fetch failed tasks in window (blockers).
	failed, err := g.store.ListByStatus(ctx, task.StatusFailed)
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}
	for _, t := range failed {
		if inWindow(t, from) {
			s.Blocked = append(s.Blocked, t)
		}
	}

	// Fetch waiting_input tasks (also blockers).
	waiting, err := g.store.ListByStatus(ctx, task.StatusWaitingInput)
	if err != nil {
		return nil, fmt.Errorf("list waiting_input: %w", err)
	}
	for _, t := range waiting {
		if inWindow(t, from) {
			s.Blocked = append(s.Blocked, t)
		}
	}

	// Fetch active (in-progress) tasks.
	active, err := g.store.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active: %w", err)
	}
	for _, t := range active {
		if t.Status == task.StatusRunning || t.Status == task.StatusPending {
			s.InProgress = append(s.InProgress, t)
		}
	}

	// Identify new tasks: created within the window across all statuses.
	allTasks, _, err := g.store.List(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("list all: %w", err)
	}
	for _, t := range allTasks {
		if t.CreatedAt.After(from) {
			s.New = append(s.New, t)
		}
	}

	// Sort blocked by creation time descending.
	sort.Slice(s.Blocked, func(i, j int) bool {
		return s.Blocked[i].CreatedAt.After(s.Blocked[j].CreatedAt)
	})

	// Compute completion rate.
	totalFailed := countFailed(s.Blocked)
	totalTerminal := len(s.Completed) + totalFailed
	if totalTerminal > 0 {
		s.CompletionRate = float64(len(s.Completed)) / float64(totalTerminal)
	}

	// Compute avg blocker resolve time from completed tasks that were
	// previously blocked (have a StartedAt, indicating they ran and then
	// resolved). We approximate with CompletedAt - StartedAt for tasks that
	// completed after being in a non-trivial state.
	var totalResolve time.Duration
	resolvedCount := 0
	for _, t := range s.Completed {
		if t.StartedAt != nil && t.CompletedAt != nil {
			d := t.CompletedAt.Sub(*t.StartedAt)
			totalResolve += d
			resolvedCount++
		}
	}
	if resolvedCount > 0 {
		s.AvgBlockerResolveTime = totalResolve / time.Duration(resolvedCount)
	}

	// Compute top active agents from all tasks in window.
	agentCounts := make(map[string]int)
	for _, t := range s.New {
		agent := t.AgentType
		if agent == "" {
			agent = "unknown"
		}
		agentCounts[agent]++
	}
	for agent, count := range agentCounts {
		s.TopActiveAgents = append(s.TopActiveAgents, AgentActivity{
			AgentType: agent,
			TaskCount: count,
		})
	}
	sort.Slice(s.TopActiveAgents, func(i, j int) bool {
		return s.TopActiveAgents[i].TaskCount > s.TopActiveAgents[j].TaskCount
	})
	if len(s.TopActiveAgents) > 5 {
		s.TopActiveAgents = s.TopActiveAgents[:5]
	}

	return s, nil
}

// FormatMarkdown renders a DailySummary as a markdown digest.
func FormatMarkdown(s *DailySummary) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Daily Summary (%s)\n\n",
		s.From.Format("Jan 2, 2006")))

	// Highlights
	b.WriteString("## Highlights\n\n")
	b.WriteString(fmt.Sprintf("- **%d** new tasks created\n", len(s.New)))
	b.WriteString(fmt.Sprintf("- **%d** tasks completed\n", len(s.Completed)))
	b.WriteString(fmt.Sprintf("- **%d** tasks in progress\n", len(s.InProgress)))
	b.WriteString(fmt.Sprintf("- **%d** tasks blocked\n", len(s.Blocked)))

	// Metrics
	b.WriteString("\n## Metrics\n\n")
	if len(s.Completed) > 0 || countFailed(s.Blocked) > 0 {
		b.WriteString(fmt.Sprintf("- **Completion rate:** %.0f%%\n", s.CompletionRate*100))
	} else {
		b.WriteString("- **Completion rate:** N/A\n")
	}
	if s.AvgBlockerResolveTime > 0 {
		b.WriteString(fmt.Sprintf("- **Avg time-to-resolve:** %s\n", formatDuration(s.AvgBlockerResolveTime)))
	} else {
		b.WriteString("- **Avg time-to-resolve:** N/A\n")
	}
	if len(s.TopActiveAgents) > 0 {
		b.WriteString("- **Top active agents:**\n")
		for _, a := range s.TopActiveAgents {
			b.WriteString(fmt.Sprintf("  - %s: %d tasks\n", a.AgentType, a.TaskCount))
		}
	}

	// Action Items
	b.WriteString("\n## Action Items\n\n")
	if len(s.Blocked) == 0 {
		b.WriteString("No action items — all clear.\n")
	} else {
		for _, t := range s.Blocked {
			desc := taskLabel(t)
			reason := ""
			if t.Status == task.StatusFailed && t.Error != "" {
				reason = fmt.Sprintf(" — error: %s", truncate(t.Error, 100))
			} else if t.Status == task.StatusWaitingInput {
				reason = " — waiting for input"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s%s\n", t.Status, truncate(desc, 80), reason))
		}
	}

	return b.String()
}

// Service wraps a Generator with notification delivery for scheduler integration.
type Service struct {
	gen      *Generator
	notifier notification.Notifier
	channel  string
	chatID   string
}

// NewService creates a daily summary Service that generates and sends digests.
func NewService(store task.Store, notifier notification.Notifier, channel, chatID string) *Service {
	return &Service{
		gen:      NewGenerator(store),
		notifier: notifier,
		channel:  channel,
		chatID:   chatID,
	}
}

// GenerateAndSend produces a daily summary digest and delivers it via the notifier.
// If no notifier is configured, it returns without sending.
func (svc *Service) GenerateAndSend(ctx context.Context) error {
	summary, err := svc.gen.Generate(ctx)
	if err != nil {
		return fmt.Errorf("generate daily summary: %w", err)
	}

	content := FormatMarkdown(summary)

	if svc.notifier == nil {
		return nil
	}

	target := notification.Target{
		Channel: svc.channel,
		ChatID:  svc.chatID,
	}
	if err := svc.notifier.Send(ctx, target, content); err != nil {
		return fmt.Errorf("send daily summary: %w", err)
	}
	return nil
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
