// Package milestone provides periodic progress check-in summaries.
//
// The Service queries the task store for recent activity within a configurable
// lookback window, aggregates progress metrics, and formats a human-readable
// summary suitable for delivery via the notification system.
package milestone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/task"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
)

// Config controls milestone check-in behavior.
type Config struct {
	Enabled          bool          `json:"enabled" yaml:"enabled"`
	IntervalSeconds  int           `json:"interval_seconds" yaml:"interval_seconds"`
	LookbackDuration time.Duration `json:"-" yaml:"-"` // derived from IntervalSeconds
	Channel          string        `json:"channel" yaml:"channel"`     // "lark" or "moltbook"
	ChatID           string        `json:"chat_id" yaml:"chat_id"`     // delivery target
	IncludeActive    bool          `json:"include_active" yaml:"include_active"`       // include in-flight tasks
	IncludeCompleted bool          `json:"include_completed" yaml:"include_completed"` // include recently finished tasks
}

// DefaultConfig returns sensible defaults for milestone check-ins.
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		IntervalSeconds:  3600, // 1 hour
		LookbackDuration: time.Hour,
		Channel:          "lark",
		IncludeActive:    true,
		IncludeCompleted: true,
	}
}

// Service generates periodic progress check-in summaries.
type Service struct {
	store    task.Store
	notifier notification.Notifier
	config   Config
	logger   logging.Logger
}

// NewService creates a milestone check-in service.
func NewService(store task.Store, notifier notification.Notifier, cfg Config) *Service {
	if cfg.IntervalSeconds > 0 && cfg.LookbackDuration == 0 {
		cfg.LookbackDuration = time.Duration(cfg.IntervalSeconds) * time.Second
	}
	if cfg.LookbackDuration == 0 {
		cfg.LookbackDuration = time.Hour
	}
	return &Service{
		store:    store,
		notifier: notifier,
		config:   cfg,
		logger:   logging.NewComponentLogger("milestone"),
	}
}

// Summary holds aggregated progress metrics for a check-in window.
type Summary struct {
	Window        time.Duration
	ActiveTasks   []*task.Task
	CompletedIn   []*task.Task
	FailedIn      []*task.Task
	TotalTokens   int
	TotalCostUSD  float64
	GeneratedAt   time.Time
}

// GenerateSummary queries the task store and builds a progress summary
// covering the configured lookback window. When ChatID is set, results
// are scoped to that chat to prevent task leaks across sessions.
func (s *Service) GenerateSummary(ctx context.Context) (*Summary, error) {
	now := time.Now()
	cutoff := now.Add(-s.config.LookbackDuration)

	sum := &Summary{
		Window:      s.config.LookbackDuration,
		GeneratedAt: now,
	}

	chatID := strings.TrimSpace(s.config.ChatID)

	if s.config.IncludeActive {
		var active []*task.Task
		var err error
		if chatID != "" {
			active, err = s.store.ListByChat(ctx, chatID, true, 0)
		} else {
			active, err = s.store.ListActive(ctx)
		}
		if err != nil {
			return nil, fmt.Errorf("list active tasks: %w", err)
		}
		sum.ActiveTasks = active
	}

	if s.config.IncludeCompleted {
		var completed, failed []*task.Task
		if chatID != "" {
			// ListByChat with activeOnly=false returns all tasks for the chat;
			// filter by status and recency client-side.
			all, err := s.store.ListByChat(ctx, chatID, false, 0)
			if err != nil {
				return nil, fmt.Errorf("list chat tasks: %w", err)
			}
			for _, t := range all {
				if t.CompletedAt == nil || !t.CompletedAt.After(cutoff) {
					continue
				}
				switch t.Status {
				case task.StatusCompleted:
					completed = append(completed, t)
				case task.StatusFailed:
					failed = append(failed, t)
				}
			}
		} else {
			var err error
			completed, err = s.store.ListByStatus(ctx, task.StatusCompleted)
			if err != nil {
				return nil, fmt.Errorf("list completed tasks: %w", err)
			}
			completed = filterRecent(completed, cutoff)

			failed, err = s.store.ListByStatus(ctx, task.StatusFailed)
			if err != nil {
				return nil, fmt.Errorf("list failed tasks: %w", err)
			}
			failed = filterRecent(failed, cutoff)
		}
		sum.CompletedIn = completed
		sum.FailedIn = failed
	}

	// Aggregate token/cost across all included tasks.
	for _, t := range sum.ActiveTasks {
		sum.TotalTokens += t.TokensUsed
		sum.TotalCostUSD += t.CostUSD
	}
	for _, t := range sum.CompletedIn {
		sum.TotalTokens += t.TokensUsed
		sum.TotalCostUSD += t.CostUSD
	}
	for _, t := range sum.FailedIn {
		sum.TotalTokens += t.TokensUsed
		sum.TotalCostUSD += t.CostUSD
	}

	return sum, nil
}

// FormatSummary renders a Summary as a human-readable Markdown string.
func FormatSummary(sum *Summary) string {
	var b strings.Builder

	windowLabel := formatDuration(sum.Window)
	b.WriteString(fmt.Sprintf("## Milestone Check-in (%s window)\n\n", windowLabel))

	totalCompleted := len(sum.CompletedIn)
	totalFailed := len(sum.FailedIn)
	totalActive := len(sum.ActiveTasks)
	totalFinished := totalCompleted + totalFailed

	b.WriteString(fmt.Sprintf("**Active:** %d | **Completed:** %d | **Failed:** %d\n", totalActive, totalCompleted, totalFailed))

	if sum.TotalTokens > 0 || sum.TotalCostUSD > 0 {
		b.WriteString(fmt.Sprintf("**Tokens used:** %d | **Cost:** $%.4f\n", sum.TotalTokens, sum.TotalCostUSD))
	}

	if totalFinished > 0 {
		rate := float64(totalCompleted) / float64(totalFinished) * 100
		b.WriteString(fmt.Sprintf("**Success rate:** %.0f%%\n", rate))
	}

	if totalActive > 0 {
		b.WriteString("\n### In Progress\n")
		for _, t := range sum.ActiveTasks {
			desc := truncate(taskLabel(t), 80)
			b.WriteString(fmt.Sprintf("- [%s] %s (iter %d, %d tokens)\n", t.Status, desc, t.CurrentIteration, t.TokensUsed))
		}
	}

	if totalCompleted > 0 {
		b.WriteString("\n### Completed\n")
		for _, t := range sum.CompletedIn {
			desc := truncate(taskLabel(t), 80)
			preview := truncate(t.AnswerPreview, 120)
			line := fmt.Sprintf("- %s", desc)
			if preview != "" {
				line += fmt.Sprintf(" — %s", preview)
			}
			b.WriteString(line + "\n")
		}
	}

	if totalFailed > 0 {
		b.WriteString("\n### Failed\n")
		for _, t := range sum.FailedIn {
			desc := truncate(taskLabel(t), 80)
			errMsg := truncate(t.Error, 120)
			line := fmt.Sprintf("- %s", desc)
			if errMsg != "" {
				line += fmt.Sprintf(" — error: %s", errMsg)
			}
			b.WriteString(line + "\n")
		}
	}

	if totalActive == 0 && totalFinished == 0 {
		b.WriteString("\nNo task activity in this window.\n")
	}

	return b.String()
}

// SendCheckin generates a summary and delivers it via the configured notifier.
func (s *Service) SendCheckin(ctx context.Context) error {
	sum, err := s.GenerateSummary(ctx)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	content := FormatSummary(sum)

	if s.notifier == nil {
		s.logger.Info("Milestone check-in (no notifier):\n%s", content)
		return nil
	}

	target := notification.Target{
		Channel: s.config.Channel,
		ChatID:  s.config.ChatID,
	}
	if err := s.notifier.Send(ctx, target, content); err != nil {
		return fmt.Errorf("send check-in: %w", err)
	}

	s.logger.Info("Milestone check-in sent to %s/%s", s.config.Channel, s.config.ChatID)
	return nil
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

func filterRecent(tasks []*task.Task, cutoff time.Time) []*task.Task {
	var out []*task.Task
	for _, t := range tasks {
		if t.CompletedAt != nil && t.CompletedAt.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
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
	if mins == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", mins)
}
