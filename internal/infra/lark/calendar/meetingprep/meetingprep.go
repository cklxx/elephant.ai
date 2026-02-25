// Package meetingprep assembles structured meeting preparation documents from
// pre-fetched calendar context, historical notes, decisions, and tasks. It is
// a pure transformation module — no Lark API calls, no external dependencies —
// so it remains testable and free of import cycles.
package meetingprep

import (
	"fmt"
	"strings"
	"time"
)

// ---------- Input types ----------

// MeetingContext carries all data gathered by the caller (tool/agent) that is
// needed to produce a meeting prep document.
type MeetingContext struct {
	Event            EventInfo
	Attendees        []AttendeeInfo
	RecentNotes      []NoteReference      // past meeting notes for same recurring series/attendees
	RelatedDecisions []DecisionReference   // decisions relevant to meeting topic
	PendingTasks     []TaskReference       // open tasks assigned to attendees
	UserTimezone     string                // IANA timezone name, e.g. "Asia/Shanghai"
}

// EventInfo describes a single calendar event.
type EventInfo struct {
	EventID     string
	Summary     string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	Location    string
	IsRecurring bool
}

// AttendeeInfo describes a meeting participant.
type AttendeeInfo struct {
	UserID string
	Name   string
	Email  string
	Role   string // "organizer", "required", "optional"
}

// NoteReference points to a historical meeting note.
type NoteReference struct {
	Date    time.Time
	Title   string
	Summary string
}

// DecisionReference records a decision relevant to the meeting topic.
type DecisionReference struct {
	Decision  string
	Rationale string
	Date      time.Time
	Resolved  bool
}

// TaskReference represents a tracked task assigned to an attendee.
type TaskReference struct {
	TaskID    string
	Summary   string
	Assignee  string
	DueTime   *time.Time
	Completed bool
}

// ---------- Output type ----------

// PrepDocument is the structured output of meeting preparation.
type PrepDocument struct {
	EventSummary      string
	TimeInfo          string
	AttendeeList      string
	AgendaSuggestions []string
	HistoricalContext string
	OpenDecisions     string
	ActionItems       string
	FullMarkdown      string
}

// ---------- Config ----------

// PrepConfig controls limits and feature flags for prep document generation.
type PrepConfig struct {
	MaxHistoricalNotes int  // max past notes to include (default 3)
	MaxAgendaItems     int  // max agenda suggestions (default 7)
	IncludeCompleted   bool // include completed tasks/decisions (default false)
}

// DefaultPrepConfig returns sensible defaults.
func DefaultPrepConfig() PrepConfig {
	return PrepConfig{
		MaxHistoricalNotes: 3,
		MaxAgendaItems:     7,
		IncludeCompleted:   false,
	}
}

// ---------- Public API ----------

// PrepareMeeting assembles a full PrepDocument from the provided context using
// the default config.
func PrepareMeeting(ctx MeetingContext) (*PrepDocument, error) {
	return PrepareMeetingWithConfig(ctx, DefaultPrepConfig())
}

// PrepareMeetingWithConfig assembles a full PrepDocument with explicit config.
func PrepareMeetingWithConfig(ctx MeetingContext, cfg PrepConfig) (*PrepDocument, error) {
	doc := &PrepDocument{}

	doc.EventSummary = buildEventSummary(ctx.Event)
	doc.TimeInfo = buildTimeInfo(ctx.Event, ctx.UserTimezone)
	doc.AttendeeList = buildAttendeeList(ctx.Attendees)
	doc.AgendaSuggestions = SuggestAgenda(ctx)
	if cfg.MaxAgendaItems > 0 && len(doc.AgendaSuggestions) > cfg.MaxAgendaItems {
		doc.AgendaSuggestions = doc.AgendaSuggestions[:cfg.MaxAgendaItems]
	}
	doc.HistoricalContext = buildHistoricalContext(ctx.RecentNotes, cfg.MaxHistoricalNotes)
	doc.OpenDecisions = buildOpenDecisions(ctx.RelatedDecisions, cfg.IncludeCompleted)
	doc.ActionItems = buildActionItems(ctx.PendingTasks, cfg.IncludeCompleted)
	doc.FullMarkdown = FormatMarkdown(doc)

	return doc, nil
}

// SuggestAgenda produces heuristic agenda suggestions based on the meeting
// context. The rules are:
//   - Include "Review: {decision}" for each unresolved decision.
//   - Include "Follow up: {task}" for each overdue (past due) task.
//   - Include carry-over items derived from recent note summaries.
//   - Include pending (non-overdue, non-completed) tasks.
//   - If the event description is non-empty, include it as the first item.
func SuggestAgenda(ctx MeetingContext) []string {
	var items []string

	// 1. Event description as topic, if present.
	desc := strings.TrimSpace(ctx.Event.Description)
	if desc != "" {
		items = append(items, "Topic: "+firstLine(desc))
	}

	// 2. Unresolved decisions.
	for _, d := range ctx.RelatedDecisions {
		if !d.Resolved {
			items = append(items, "Review: "+d.Decision)
		}
	}

	// 3. Overdue tasks (due before event start, not completed).
	for _, t := range ctx.PendingTasks {
		if t.Completed {
			continue
		}
		if t.DueTime != nil && t.DueTime.Before(ctx.Event.StartTime) {
			items = append(items, "Follow up: "+t.Summary)
		}
	}

	// 4. Carry-over from recent notes (recurring meeting context).
	for _, n := range ctx.RecentNotes {
		if n.Summary != "" {
			items = append(items, "Carry over: "+firstLine(n.Summary))
		}
	}

	// 5. Pending (non-overdue) tasks.
	for _, t := range ctx.PendingTasks {
		if t.Completed {
			continue
		}
		if t.DueTime == nil || !t.DueTime.Before(ctx.Event.StartTime) {
			items = append(items, "Discuss: "+t.Summary)
		}
	}

	return items
}

// FormatMarkdown renders a PrepDocument as a Markdown string.
func FormatMarkdown(doc *PrepDocument) string {
	var sb strings.Builder

	sb.WriteString("# Meeting Prep\n\n")

	// Event summary.
	sb.WriteString("## ")
	sb.WriteString(doc.EventSummary)
	sb.WriteString("\n\n")

	// Time.
	if doc.TimeInfo != "" {
		sb.WriteString("**When:** ")
		sb.WriteString(doc.TimeInfo)
		sb.WriteString("\n\n")
	}

	// Attendees.
	if doc.AttendeeList != "" {
		sb.WriteString("## Attendees\n\n")
		sb.WriteString(doc.AttendeeList)
		sb.WriteString("\n\n")
	}

	// Agenda suggestions.
	if len(doc.AgendaSuggestions) > 0 {
		sb.WriteString("## Suggested Agenda\n\n")
		for i, item := range doc.AgendaSuggestions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
		}
		sb.WriteString("\n")
	}

	// Historical context.
	if doc.HistoricalContext != "" {
		sb.WriteString("## Historical Context\n\n")
		sb.WriteString(doc.HistoricalContext)
		sb.WriteString("\n\n")
	}

	// Open decisions.
	if doc.OpenDecisions != "" {
		sb.WriteString("## Open Decisions\n\n")
		sb.WriteString(doc.OpenDecisions)
		sb.WriteString("\n\n")
	}

	// Action items.
	if doc.ActionItems != "" {
		sb.WriteString("## Action Items\n\n")
		sb.WriteString(doc.ActionItems)
		sb.WriteString("\n")
	}

	return sb.String()
}

// ---------- Internal helpers ----------

// buildEventSummary produces a one-line event summary.
func buildEventSummary(ev EventInfo) string {
	summary := ev.Summary
	if summary == "" {
		summary = "(No title)"
	}
	if ev.Location != "" {
		summary += " @ " + ev.Location
	}
	if ev.IsRecurring {
		summary += " [recurring]"
	}
	return summary
}

// buildTimeInfo formats the event start/end using the specified timezone.
func buildTimeInfo(ev EventInfo, tz string) string {
	loc := time.UTC
	if tz != "" {
		if parsed, err := time.LoadLocation(tz); err == nil {
			loc = parsed
		}
	}

	start := ev.StartTime.In(loc)
	end := ev.EndTime.In(loc)

	dur := ev.EndTime.Sub(ev.StartTime)
	durStr := formatDuration(dur)

	return fmt.Sprintf("%s - %s (%s, %s)",
		start.Format("2006-01-02 15:04"),
		end.Format("15:04"),
		durStr,
		loc.String(),
	)
}

// buildAttendeeList formats attendees with their roles.
func buildAttendeeList(attendees []AttendeeInfo) string {
	if len(attendees) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, a := range attendees {
		name := a.Name
		if name == "" {
			name = a.Email
		}
		if name == "" {
			name = a.UserID
		}
		role := a.Role
		if role == "" {
			role = "attendee"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", name, role))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// buildHistoricalContext summarizes past meeting notes.
func buildHistoricalContext(notes []NoteReference, maxNotes int) string {
	if len(notes) == 0 {
		return ""
	}
	limit := len(notes)
	if maxNotes > 0 && limit > maxNotes {
		limit = maxNotes
	}

	var sb strings.Builder
	for _, n := range notes[:limit] {
		dateStr := n.Date.Format("2006-01-02")
		sb.WriteString(fmt.Sprintf("- **%s** (%s)", n.Title, dateStr))
		if n.Summary != "" {
			sb.WriteString(": " + n.Summary)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// buildOpenDecisions lists unresolved (and optionally resolved) decisions.
func buildOpenDecisions(decisions []DecisionReference, includeResolved bool) string {
	if len(decisions) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, d := range decisions {
		if d.Resolved && !includeResolved {
			continue
		}
		status := "OPEN"
		if d.Resolved {
			status = "RESOLVED"
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s", status, d.Decision))
		if d.Rationale != "" {
			sb.WriteString(fmt.Sprintf(" — Rationale: %s", d.Rationale))
		}
		sb.WriteString(fmt.Sprintf(" (%s)\n", d.Date.Format("2006-01-02")))
	}
	result := strings.TrimRight(sb.String(), "\n")
	return result
}

// buildActionItems lists pending (and optionally completed) tasks.
func buildActionItems(tasks []TaskReference, includeCompleted bool) string {
	if len(tasks) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, t := range tasks {
		if t.Completed && !includeCompleted {
			continue
		}
		marker := "[ ]"
		if t.Completed {
			marker = "[x]"
		}
		sb.WriteString(fmt.Sprintf("- %s %s", marker, t.Summary))
		if t.Assignee != "" {
			sb.WriteString(fmt.Sprintf(" (@%s)", t.Assignee))
		}
		if t.DueTime != nil {
			sb.WriteString(fmt.Sprintf(" due %s", t.DueTime.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}
	result := strings.TrimRight(sb.String(), "\n")
	return result
}

// formatDuration renders a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "< 1 min"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	hours := minutes / 60
	rem := minutes % 60
	if rem == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, rem)
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return s
}
