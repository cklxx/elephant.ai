// Package cards provides Lark interactive card templates for leader agent
// notifications. Cards follow the Lark card JSON schema and are returned as
// serialized JSON strings ready for sending via "interactive" message type.
package cards

import (
	"fmt"
	"strings"
	"time"

	jsonx "alex/internal/shared/json"
)

// BlockerAlertCard builds a Lark interactive card for a blocked task alert.
// It shows the task description, block reason, duration, and a suggested action.
func BlockerAlertCard(taskID, description, reason, detail string, duration time.Duration, suggestedAction string) string {
	if description == "" {
		description = taskID
	}

	durationText := ""
	if duration > 0 {
		durationText = fmt.Sprintf("\n**Duration:** %s", formatDuration(duration))
	}

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "Blocked Task Alert",
			},
			"template": "red",
		},
		"elements": []any{
			map[string]any{
				"tag":     "markdown",
				"content": fmt.Sprintf("**Task:** %s\n**ID:** `%s`\n**Reason:** %s%s", truncate(description, 100), taskID, detail, durationText),
			},
			map[string]any{
				"tag": "hr",
			},
			map[string]any{
				"tag":     "markdown",
				"content": fmt.Sprintf("**Suggested action:** %s", suggestedAction),
			},
		},
	}

	data, _ := jsonx.Marshal(card)
	return string(data)
}

// PulseMetric holds a single metric row for the weekly pulse card.
type PulseMetric struct {
	Label string
	Value string
	Trend string // "up", "down", "flat", or ""
}

// BlockerEntry holds a blocker summary for the weekly pulse card.
type BlockerEntry struct {
	Description string
	Status      string
	Reason      string
}

// WeeklyPulseCard builds a Lark interactive card for the weekly pulse digest.
// It shows a stats table with trend arrows, top blockers, and task breakdown.
func WeeklyPulseCard(fromDate, toDate string, metrics []PulseMetric, blockers []BlockerEntry) string {
	// Build metrics table.
	var metricsLines []string
	for _, m := range metrics {
		arrow := trendArrow(m.Trend)
		if arrow != "" {
			metricsLines = append(metricsLines, fmt.Sprintf("- **%s:** %s %s", m.Label, m.Value, arrow))
		} else {
			metricsLines = append(metricsLines, fmt.Sprintf("- **%s:** %s", m.Label, m.Value))
		}
	}
	metricsText := strings.Join(metricsLines, "\n")

	elements := []any{
		map[string]any{
			"tag":     "markdown",
			"content": metricsText,
		},
	}

	// Add blockers section if any.
	if len(blockers) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})

		var blockerLines []string
		blockerLines = append(blockerLines, "**Top Blockers:**")
		for _, b := range blockers {
			blockerLines = append(blockerLines, fmt.Sprintf("- [%s] %s — %s", b.Status, truncate(b.Description, 60), b.Reason))
		}
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": strings.Join(blockerLines, "\n"),
		})
	}

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": fmt.Sprintf("Weekly Pulse (%s – %s)", fromDate, toDate),
			},
			"template": "blue",
		},
		"elements": elements,
	}

	data, _ := jsonx.Marshal(card)
	return string(data)
}

// DailySummaryCard builds a compact Lark interactive card for the daily digest.
// It shows highlights counts and action items.
func DailySummaryCard(date string, newTasks, completed, inProgress, blocked int, completionRate float64, actionItems []string) string {
	highlightsText := fmt.Sprintf(
		"**New:** %d  |  **Completed:** %d  |  **In Progress:** %d  |  **Blocked:** %d\n**Completion Rate:** %.0f%%",
		newTasks, completed, inProgress, blocked, completionRate*100,
	)

	elements := []any{
		map[string]any{
			"tag":     "markdown",
			"content": highlightsText,
		},
	}

	if len(actionItems) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		var lines []string
		lines = append(lines, "**Action Items:**")
		for _, item := range actionItems {
			lines = append(lines, fmt.Sprintf("- %s", item))
		}
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": strings.Join(lines, "\n"),
		})
	}

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": fmt.Sprintf("Daily Summary — %s", date),
			},
			"template": "turquoise",
		},
		"elements": elements,
	}

	data, _ := jsonx.Marshal(card)
	return string(data)
}

// MilestoneCard builds a Lark interactive card for a milestone progress check-in.
// It shows a text-based progress bar with completion percentage and task counts.
func MilestoneCard(title string, completedCount, totalCount int, activeTasks []string, recentCompletions []string) string {
	pct := 0.0
	if totalCount > 0 {
		pct = float64(completedCount) / float64(totalCount) * 100
	}

	progressBar := buildProgressBar(pct)
	statsText := fmt.Sprintf(
		"%s  **%.0f%%**\n**Completed:** %d / %d",
		progressBar, pct, completedCount, totalCount,
	)

	elements := []any{
		map[string]any{
			"tag":     "markdown",
			"content": statsText,
		},
	}

	if len(recentCompletions) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		var lines []string
		lines = append(lines, "**Recently Completed:**")
		for _, c := range recentCompletions {
			lines = append(lines, fmt.Sprintf("- %s", truncate(c, 80)))
		}
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": strings.Join(lines, "\n"),
		})
	}

	if len(activeTasks) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		var lines []string
		lines = append(lines, "**Active:**")
		for _, a := range activeTasks {
			lines = append(lines, fmt.Sprintf("- %s", truncate(a, 80)))
		}
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": strings.Join(lines, "\n"),
		})
	}

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": "green",
		},
		"elements": elements,
	}

	data, _ := jsonx.Marshal(card)
	return string(data)
}

// buildProgressBar creates a text-based progress bar using block characters.
func buildProgressBar(pct float64) string {
	const totalBlocks = 20
	filled := int(pct / 100.0 * totalBlocks)
	if filled > totalBlocks {
		filled = totalBlocks
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", totalBlocks-filled)
}

func trendArrow(trend string) string {
	switch trend {
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "flat":
		return "→"
	default:
		return ""
	}
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
