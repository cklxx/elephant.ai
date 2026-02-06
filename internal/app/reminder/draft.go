package reminder

import (
	"bytes"
	"fmt"
	"text/template"
	"time"
)

// ReminderDraft is the generated reminder message awaiting user confirmation.
type ReminderDraft struct {
	Intent           ReminderIntent
	Message          string   // human-readable reminder text
	Channel          string   // delivery channel (lark, web, wechat, etc.)
	SuggestedActions []string // e.g. "Snooze 15min", "Dismiss", "Open event"
}

var templateFuncs = template.FuncMap{
	"duration": formatDuration,
}

var calendarTmpl = template.Must(
	template.New("calendar").Funcs(templateFuncs).Parse(
		`Reminder: {{.Title}} starts in {{duration .Until}}.{{if .Location}} Location: {{.Location}}{{end}}`,
	),
)

var taskDeadlineTmpl = template.Must(
	template.New("task_deadline").Funcs(templateFuncs).Parse(
		`Task '{{.Title}}' is due in {{duration .Until}}.`,
	),
)

var followUpTmpl = template.Must(
	template.New("follow_up").Parse(
		`Follow up on: {{.Title}}`,
	),
)

// templateData is the common data passed to all templates.
type templateData struct {
	Title    string
	Until    time.Duration
	Location string
}

// formatDuration returns a human-friendly duration string.
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "now"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return "now"
	}
}

// DraftBuilder generates reminder drafts from intents.
type DraftBuilder struct {
	// Now returns the current time; injectable for testing.
	Now func() time.Time
}

// NewDraftBuilder creates a DraftBuilder using the real clock.
func NewDraftBuilder() *DraftBuilder {
	return &DraftBuilder{Now: time.Now}
}

// Build generates a ReminderDraft from the given intent.
func (b *DraftBuilder) Build(intent ReminderIntent) ReminderDraft {
	now := b.Now()
	until := intent.TriggerTime.Sub(now)
	if until < 0 {
		until = 0
	}

	data := templateData{
		Title:    intent.SourceTitle,
		Until:    until,
		Location: intent.Context["location"],
	}

	var message string
	var actions []string

	switch intent.Type {
	case IntentCalendarReminder:
		message = renderTemplate(calendarTmpl, data)
		actions = []string{"Snooze 15min", "Dismiss", "Open event"}
	case IntentTaskDeadline:
		message = renderTemplate(taskDeadlineTmpl, data)
		actions = []string{"Snooze 15min", "Dismiss", "Mark complete"}
	case IntentFollowUp:
		message = renderTemplate(followUpTmpl, data)
		actions = []string{"Snooze 15min", "Dismiss", "Open thread"}
	default:
		message = fmt.Sprintf("Reminder: %s", intent.SourceTitle)
		actions = []string{"Dismiss"}
	}

	channel := intent.Context["channel"]
	if channel == "" {
		channel = "lark"
	}

	return ReminderDraft{
		Intent:           intent,
		Message:          message,
		Channel:          channel,
		SuggestedActions: actions,
	}
}

// renderTemplate executes a template with the given data, returning the result string.
func renderTemplate(tmpl *template.Template, data templateData) string {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Reminder: %s", data.Title)
	}
	return buf.String()
}
