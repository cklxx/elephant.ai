// Package taskfmt provides shared formatting helpers for task display.
//
// These functions are used by blocker, milestone, summary, prepbrief, and
// pulse packages to render task information consistently.
package taskfmt

import (
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/task"
	"alex/internal/shared/utils"
)

// TaskLabel returns a human-readable label for a task, preferring the
// description over the raw ID.
func TaskLabel(t *task.Task) string {
	if t.Description != "" {
		return t.Description
	}
	return t.TaskID
}

// Truncate trims whitespace and cuts s to maxLen runes, appending "..."
// when truncation occurs.
func Truncate(s string, maxLen int) string {
	return utils.TruncateWithEllipsis(strings.TrimSpace(s), maxLen)
}

// FormatDuration renders a duration as a human-friendly string
// (e.g. "2 hours", "1 day", "30 minutes").
func FormatDuration(d time.Duration) string {
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

// FormatDurationCompact renders a duration in short notation
// (e.g. "45s", "5m", "2h30m").
func FormatDurationCompact(d time.Duration) string {
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
