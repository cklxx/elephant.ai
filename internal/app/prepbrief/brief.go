// Package prepbrief generates 1:1 prep briefs for team member meetings.
//
// Given a member ID, it queries the task store for recent activity,
// categorises tasks into wins/open/blocked, and produces a Markdown
// brief with suggested discussion points.
package prepbrief

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/task"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
)

// Config controls prep-brief generation.
type Config struct {
	LookbackDuration time.Duration `json:"-" yaml:"-"`
	LookbackSeconds  int           `json:"lookback_seconds" yaml:"lookback_seconds"` // default 604800 (7 days)
	Channel          string        `json:"channel" yaml:"channel"`
	ChatID           string        `json:"chat_id" yaml:"chat_id"`
}

// DefaultConfig returns sensible defaults (7-day lookback).
func DefaultConfig() Config {
	return Config{
		LookbackSeconds:  604800,
		LookbackDuration: 7 * 24 * time.Hour,
		Channel:          "lark",
	}
}

// Brief holds the categorised data for a single 1:1 prep.
type Brief struct {
	MemberID    string
	GeneratedAt time.Time
	Lookback    time.Duration

	RecentWins []*task.Task // completed within lookback
	OpenItems  []*task.Task // running or pending
	Blockers   []Blocker    // tasks with errors, stale progress, or unresolved deps
	Pending    []*task.Task // waiting_input or pending
}

// Blocker describes a single blocked task and the reason.
type Blocker struct {
	Task   *task.Task
	Reason string
}

// Service generates 1:1 prep briefs.
type Service struct {
	store    task.Store
	notifier notification.Notifier
	config   Config
	logger   logging.Logger
	nowFunc  func() time.Time // for testing
}

// NewService creates a prep-brief service.
func NewService(store task.Store, notifier notification.Notifier, cfg Config) *Service {
	if cfg.LookbackSeconds > 0 && cfg.LookbackDuration == 0 {
		cfg.LookbackDuration = time.Duration(cfg.LookbackSeconds) * time.Second
	}
	if cfg.LookbackDuration == 0 {
		cfg.LookbackDuration = 7 * 24 * time.Hour
	}
	return &Service{
		store:    store,
		notifier: notifier,
		config:   cfg,
		logger:   logging.NewComponentLogger("prepbrief"),
		nowFunc:  time.Now,
	}
}

// Generate builds a 1:1 prep brief for the given member.
// It queries completed, active, and failed tasks, filtering by member ID
// (matched against Task.UserID or Task.Metadata["member"]).
func (s *Service) Generate(ctx context.Context, memberID string) (*Brief, error) {
	now := s.nowFunc()
	cutoff := now.Add(-s.config.LookbackDuration)

	brief := &Brief{
		MemberID:    memberID,
		GeneratedAt: now,
		Lookback:    s.config.LookbackDuration,
	}

	// Collect recent completions.
	completed, err := s.store.ListByStatus(ctx, task.StatusCompleted)
	if err != nil {
		return nil, fmt.Errorf("list completed tasks: %w", err)
	}
	for _, t := range completed {
		if !matchesMember(t, memberID) {
			continue
		}
		if t.CompletedAt != nil && t.CompletedAt.After(cutoff) {
			brief.RecentWins = append(brief.RecentWins, t)
		}
	}

	// Collect active (running) items.
	active, err := s.store.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	for _, t := range active {
		if !matchesMember(t, memberID) {
			continue
		}
		switch t.Status {
		case task.StatusRunning:
			brief.OpenItems = append(brief.OpenItems, t)
			s.detectBlocker(ctx, t, now, active, brief)
		case task.StatusPending, task.StatusWaitingInput:
			brief.Pending = append(brief.Pending, t)
			s.detectBlocker(ctx, t, now, active, brief)
		}
	}

	return brief, nil
}

// detectBlocker checks whether a task qualifies as blocked and appends
// to brief.Blockers if so.
func (s *Service) detectBlocker(ctx context.Context, t *task.Task, now time.Time, active []*task.Task, brief *Brief) {
	// Error on active task.
	if t.Error != "" {
		brief.Blockers = append(brief.Blockers, Blocker{
			Task:   t,
			Reason: fmt.Sprintf("has error: %s", truncate(t.Error, 120)),
		})
		return
	}

	// Waiting for user input.
	if t.Status == task.StatusWaitingInput {
		age := now.Sub(t.UpdatedAt)
		brief.Blockers = append(brief.Blockers, Blocker{
			Task:   t,
			Reason: fmt.Sprintf("waiting for input (%s)", formatDuration(age)),
		})
		return
	}

	// Stale running task (no update for >30 min).
	if t.Status == task.StatusRunning {
		age := now.Sub(t.UpdatedAt)
		if age > 30*time.Minute {
			brief.Blockers = append(brief.Blockers, Blocker{
				Task:   t,
				Reason: fmt.Sprintf("no progress for %s", formatDuration(age)),
			})
			return
		}
	}

	// Unresolved dependencies.
	if len(t.DependsOn) > 0 {
		activeIDs := make(map[string]bool, len(active))
		for _, a := range active {
			activeIDs[a.TaskID] = true
		}
		var blocked []string
		for _, depID := range t.DependsOn {
			if activeIDs[depID] {
				blocked = append(blocked, depID)
				continue
			}
			dep, err := s.store.Get(ctx, depID)
			if err != nil || dep.Status != task.StatusCompleted {
				blocked = append(blocked, depID)
			}
		}
		if len(blocked) > 0 {
			brief.Blockers = append(brief.Blockers, Blocker{
				Task:   t,
				Reason: fmt.Sprintf("blocked by: %s", strings.Join(blocked, ", ")),
			})
		}
	}
}

// matchesMember returns true if the task belongs to the given member.
func matchesMember(t *task.Task, memberID string) bool {
	if memberID == "" {
		return true
	}
	if strings.EqualFold(t.UserID, memberID) {
		return true
	}
	if t.Metadata != nil {
		if strings.EqualFold(t.Metadata["member"], memberID) {
			return true
		}
	}
	return false
}

// FormatBrief renders a Brief as a Markdown string.
func FormatBrief(b *Brief) string {
	var out strings.Builder

	out.WriteString(fmt.Sprintf("## 1:1 Prep Brief — %s\n\n", b.MemberID))
	out.WriteString(fmt.Sprintf("_Generated %s · %s lookback_\n\n", b.GeneratedAt.Format("2006-01-02 15:04"), formatDuration(b.Lookback)))

	// Recent Wins.
	out.WriteString("### Recent Wins\n\n")
	if len(b.RecentWins) == 0 {
		out.WriteString("No completed tasks in this period.\n\n")
	} else {
		for _, t := range b.RecentWins {
			desc := truncate(taskLabel(t), 80)
			line := fmt.Sprintf("- %s", desc)
			if t.AnswerPreview != "" {
				line += fmt.Sprintf(" — %s", truncate(t.AnswerPreview, 100))
			}
			out.WriteString(line + "\n")
		}
		out.WriteString("\n")
	}

	// Open Items.
	out.WriteString("### Open Items\n\n")
	if len(b.OpenItems) == 0 {
		out.WriteString("No active tasks.\n\n")
	} else {
		for _, t := range b.OpenItems {
			desc := truncate(taskLabel(t), 80)
			out.WriteString(fmt.Sprintf("- [%s] %s", t.Status, desc))
			if t.CurrentIteration > 0 {
				out.WriteString(fmt.Sprintf(" (iter %d)", t.CurrentIteration))
			}
			out.WriteString("\n")
		}
		out.WriteString("\n")
	}

	// Blockers.
	out.WriteString("### Blockers\n\n")
	if len(b.Blockers) == 0 {
		out.WriteString("No blockers detected.\n\n")
	} else {
		for _, bl := range b.Blockers {
			desc := truncate(taskLabel(bl.Task), 80)
			out.WriteString(fmt.Sprintf("- **%s** — %s\n", desc, bl.Reason))
		}
		out.WriteString("\n")
	}

	// Suggested Discussion Points.
	out.WriteString("### Suggested Discussion Points\n\n")
	points := suggestDiscussionPoints(b)
	if len(points) == 0 {
		out.WriteString("No specific topics flagged.\n")
	} else {
		for _, p := range points {
			out.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	return out.String()
}

// suggestDiscussionPoints generates talking points based on brief data.
func suggestDiscussionPoints(b *Brief) []string {
	var points []string

	if len(b.Blockers) > 0 {
		points = append(points, fmt.Sprintf("Resolve %d blocker(s) — prioritise unblocking before new work", len(b.Blockers)))
	}
	if len(b.RecentWins) > 0 {
		points = append(points, fmt.Sprintf("Acknowledge %d recent completion(s)", len(b.RecentWins)))
	}
	if len(b.Pending) > 0 {
		points = append(points, fmt.Sprintf("Review %d pending/waiting item(s) — may need input or re-prioritisation", len(b.Pending)))
	}
	if len(b.OpenItems) > 3 {
		points = append(points, fmt.Sprintf("High WIP (%d open items) — consider reducing work-in-progress", len(b.OpenItems)))
	}
	if len(b.RecentWins) == 0 && len(b.OpenItems) == 0 && len(b.Blockers) == 0 {
		points = append(points, "No recent activity — check in on priorities and capacity")
	}

	return points
}

// SendBrief generates a brief and delivers it via the configured notifier.
func (s *Service) SendBrief(ctx context.Context, memberID string) (*Brief, error) {
	brief, err := s.Generate(ctx, memberID)
	if err != nil {
		return nil, fmt.Errorf("generate brief: %w", err)
	}

	content := FormatBrief(brief)

	if s.notifier == nil {
		s.logger.Info("Prep brief for %s (no notifier):\n%s", memberID, content)
		return brief, nil
	}

	target := notification.Target{
		Channel: s.config.Channel,
		ChatID:  s.config.ChatID,
	}
	if err := s.notifier.Send(ctx, target, content); err != nil {
		return brief, fmt.Errorf("send brief: %w", err)
	}

	s.logger.Info("Prep brief for %s sent to %s/%s", memberID, s.config.Channel, s.config.ChatID)
	return brief, nil
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
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d >= time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	mins := int(d.Minutes())
	if mins <= 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", mins)
}
