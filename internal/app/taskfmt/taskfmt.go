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
)

// TaskLabel returns a human-readable label for a task, preferring the
// description over the raw ID.
func TaskLabel(t *task.Task) string {
	if t.Description != "" {
		return t.Description
	}
	return t.TaskID
}

// Truncate trims whitespace and cuts s to maxLen characters, appending "..."
// when truncation occurs.
func Truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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
