package reminder

import "time"

// IntentType classifies the kind of reminder.
type IntentType string

const (
	// IntentCalendarReminder is a reminder for an upcoming calendar event.
	IntentCalendarReminder IntentType = "calendar_reminder"
	// IntentTaskDeadline is a reminder for an approaching task deadline.
	IntentTaskDeadline IntentType = "task_deadline"
	// IntentFollowUp is a reminder to follow up on a previous item.
	IntentFollowUp IntentType = "follow_up"
)

// ReminderIntent captures the extracted intent from a calendar event or task.
type ReminderIntent struct {
	Type        IntentType
	SourceID    string            // event or task ID
	SourceTitle string            // human-readable title
	TriggerTime time.Time        // when the reminder should fire
	Context     map[string]string // extra context (attendees, location, etc.)
}
