package context

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"alex/internal/lark"
)

const calendarSummaryMaxChars = 500

// CalendarSummaryBuilder produces a concise natural-language summary of upcoming
// calendar events. The output is designed for injection into an LLM system
// prompt as proactive context.
type CalendarSummaryBuilder struct{}

// Build renders a grouped summary of the provided events relative to the given
// reference time. Events are bucketed into "Now", "Next", "Today", and
// "Tomorrow" sections. Cancelled events are skipped. If the rendered summary
// exceeds 500 characters it is truncated with a trailing count of omitted
// events. An empty string is returned when there are no relevant events.
func (b *CalendarSummaryBuilder) Build(events []lark.CalendarEvent, now time.Time) string {
	active := filterAndSort(events, now)
	if len(active) == 0 {
		return ""
	}

	todayStart := startOfDay(now)
	tomorrowStart := todayStart.Add(24 * time.Hour)
	tomorrowEnd := tomorrowStart.Add(24 * time.Hour)

	var (
		nowEvents      []lark.CalendarEvent
		nextEvent      *lark.CalendarEvent
		todayEvents    []lark.CalendarEvent
		tomorrowEvents []lark.CalendarEvent
	)

	foundNext := false
	for i := range active {
		ev := active[i]
		switch {
		case isHappeningNow(ev, now):
			nowEvents = append(nowEvents, ev)
		case !foundNext && !ev.StartTime.Before(now) && isSameDay(ev.StartTime, todayStart):
			nextEvent = &active[i]
			foundNext = true
		case !ev.StartTime.Before(now) && isSameDay(ev.StartTime, todayStart):
			todayEvents = append(todayEvents, ev)
		case !ev.StartTime.Before(tomorrowStart) && ev.StartTime.Before(tomorrowEnd):
			tomorrowEvents = append(tomorrowEvents, ev)
		}
	}

	totalEvents := len(nowEvents) + len(todayEvents) + len(tomorrowEvents)
	if nextEvent != nil {
		totalEvents++
	}
	if totalEvents == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Upcoming Calendar\n")
	writeSection(&sb, "Now", nowEvents)
	if nextEvent != nil {
		writeSection(&sb, "Next", []lark.CalendarEvent{*nextEvent})
	}
	writeSection(&sb, "Today", todayEvents)
	writeSection(&sb, "Tomorrow", tomorrowEvents)

	result := sb.String()
	return truncate(result, totalEvents)
}

// filterAndSort removes cancelled events and sorts the rest by start time.
func filterAndSort(events []lark.CalendarEvent, now time.Time) []lark.CalendarEvent {
	tomorrowEnd := startOfDay(now).Add(48 * time.Hour)

	filtered := make([]lark.CalendarEvent, 0, len(events))
	for _, ev := range events {
		if ev.Status == "cancelled" {
			continue
		}
		// Include events that haven't ended yet and start before end of tomorrow.
		if ev.EndTime.After(now) && ev.StartTime.Before(tomorrowEnd) {
			filtered = append(filtered, ev)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.Before(filtered[j].StartTime)
	})
	return filtered
}

// writeSection appends a labelled group of events to the builder.
func writeSection(sb *strings.Builder, label string, events []lark.CalendarEvent) {
	if len(events) == 0 {
		return
	}
	sb.WriteString(label)
	sb.WriteString(":\n")
	for _, ev := range events {
		sb.WriteString("- ")
		sb.WriteString(formatEvent(ev))
		sb.WriteString("\n")
	}
}

// formatEvent renders a single event as "HH:MM-HH:MM Summary [Location]".
func formatEvent(ev lark.CalendarEvent) string {
	timeRange := fmt.Sprintf("%s-%s",
		ev.StartTime.Format("15:04"),
		ev.EndTime.Format("15:04"))
	if ev.Location != "" {
		return fmt.Sprintf("%s %s [%s]", timeRange, ev.Summary, ev.Location)
	}
	return fmt.Sprintf("%s %s", timeRange, ev.Summary)
}

// truncate ensures the output stays within the character budget. When truncation
// is necessary, lines are removed from the end and an overflow indicator is
// appended.
func truncate(s string, totalEvents int) string {
	if len(s) <= calendarSummaryMaxChars {
		return s
	}

	lines := strings.Split(s, "\n")
	// Count rendered event lines (lines starting with "- ").
	rendered := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- ") {
			rendered++
		}
	}

	// Remove trailing event lines until the output fits.
	for len(strings.Join(lines, "\n")) > calendarSummaryMaxChars && len(lines) > 1 {
		// Remove the last non-empty line.
		idx := lastNonEmptyIndex(lines)
		if idx < 0 {
			break
		}
		if strings.HasPrefix(lines[idx], "- ") {
			rendered--
		}
		lines = append(lines[:idx], lines[idx+1:]...)

		// Also remove now-orphaned section headers (a header followed by no
		// event lines before the next header or end).
		lines = removeOrphanedHeaders(lines)
	}

	omitted := totalEvents - rendered
	suffix := fmt.Sprintf("... and %d more events\n", omitted)

	// Make room for the suffix if needed.
	for len(strings.Join(lines, "\n"))+len(suffix) > calendarSummaryMaxChars && len(lines) > 1 {
		idx := lastNonEmptyIndex(lines)
		if idx < 0 {
			break
		}
		if strings.HasPrefix(lines[idx], "- ") {
			rendered--
			omitted = totalEvents - rendered
			suffix = fmt.Sprintf("... and %d more events\n", omitted)
		}
		lines = append(lines[:idx], lines[idx+1:]...)
		lines = removeOrphanedHeaders(lines)
	}

	return strings.Join(lines, "\n") + suffix
}

func lastNonEmptyIndex(lines []string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}

// removeOrphanedHeaders drops section header lines that have no subsequent
// event entries before the next header or end of slice.
func removeOrphanedHeaders(lines []string) []string {
	cleaned := make([]string, 0, len(lines))
	for i, line := range lines {
		if isSectionHeader(line) {
			// Check if any event lines follow before the next header.
			hasEvents := false
			for j := i + 1; j < len(lines); j++ {
				if isSectionHeader(lines[j]) {
					break
				}
				if strings.HasPrefix(lines[j], "- ") {
					hasEvents = true
					break
				}
			}
			if !hasEvents {
				continue
			}
		}
		cleaned = append(cleaned, line)
	}
	return cleaned
}

func isSectionHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	// Section headers are like "Now:", "Next:", "Today:", "Tomorrow:"
	return !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "#") && strings.HasSuffix(trimmed, ":")
}

// isHappeningNow returns true when the reference time falls within the event's
// time span.
func isHappeningNow(ev lark.CalendarEvent, now time.Time) bool {
	return !now.Before(ev.StartTime) && now.Before(ev.EndTime)
}

// isSameDay checks whether t falls on the day starting at dayStart.
func isSameDay(t time.Time, dayStart time.Time) bool {
	return !t.Before(dayStart) && t.Before(dayStart.Add(24*time.Hour))
}

// startOfDay truncates a time to midnight in its location.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
