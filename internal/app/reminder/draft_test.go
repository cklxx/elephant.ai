package reminder

import (
	"strings"
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
}

func TestDraftBuilder_CalendarReminder(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-001",
		SourceTitle: "Team Standup",
		TriggerTime: fixedNow().Add(30 * time.Minute),
		Context: map[string]string{
			"location": "Room 42",
			"channel":  "lark",
		},
	}

	draft := builder.Build(intent)

	if !strings.Contains(draft.Message, "Team Standup") {
		t.Errorf("expected message to contain title, got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "30m") {
		t.Errorf("expected message to contain '30m', got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "Room 42") {
		t.Errorf("expected message to contain location, got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "Reminder:") {
		t.Errorf("expected message to start with 'Reminder:', got %q", draft.Message)
	}
	if draft.Channel != "lark" {
		t.Errorf("expected channel 'lark', got %q", draft.Channel)
	}

	expectedActions := []string{"Snooze 15min", "Dismiss", "Open event"}
	if len(draft.SuggestedActions) != len(expectedActions) {
		t.Fatalf("expected %d actions, got %d", len(expectedActions), len(draft.SuggestedActions))
	}
	for i, action := range expectedActions {
		if draft.SuggestedActions[i] != action {
			t.Errorf("action[%d] = %q, want %q", i, draft.SuggestedActions[i], action)
		}
	}
}

func TestDraftBuilder_CalendarReminder_NoLocation(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-002",
		SourceTitle: "1:1 with Manager",
		TriggerTime: fixedNow().Add(1*time.Hour + 15*time.Minute),
		Context:     map[string]string{},
	}

	draft := builder.Build(intent)

	if strings.Contains(draft.Message, "Location:") {
		t.Errorf("expected no location in message, got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "1h15m") {
		t.Errorf("expected '1h15m' duration, got %q", draft.Message)
	}
}

func TestDraftBuilder_TaskDeadline(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentTaskDeadline,
		SourceID:    "task-001",
		SourceTitle: "Submit quarterly report",
		TriggerTime: fixedNow().Add(2 * time.Hour),
		Context: map[string]string{
			"channel": "web",
		},
	}

	draft := builder.Build(intent)

	if !strings.Contains(draft.Message, "Submit quarterly report") {
		t.Errorf("expected message to contain title, got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "due in") {
		t.Errorf("expected message to contain 'due in', got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "2h") {
		t.Errorf("expected '2h' duration, got %q", draft.Message)
	}
	if draft.Channel != "web" {
		t.Errorf("expected channel 'web', got %q", draft.Channel)
	}

	expectedActions := []string{"Snooze 15min", "Dismiss", "Mark complete"}
	if len(draft.SuggestedActions) != len(expectedActions) {
		t.Fatalf("expected %d actions, got %d", len(expectedActions), len(draft.SuggestedActions))
	}
	for i, action := range expectedActions {
		if draft.SuggestedActions[i] != action {
			t.Errorf("action[%d] = %q, want %q", i, draft.SuggestedActions[i], action)
		}
	}
}

func TestDraftBuilder_FollowUp(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentFollowUp,
		SourceID:    "thread-001",
		SourceTitle: "Design review feedback",
		TriggerTime: fixedNow().Add(45 * time.Minute),
		Context:     map[string]string{},
	}

	draft := builder.Build(intent)

	if !strings.Contains(draft.Message, "Follow up on:") {
		t.Errorf("expected 'Follow up on:' in message, got %q", draft.Message)
	}
	if !strings.Contains(draft.Message, "Design review feedback") {
		t.Errorf("expected title in message, got %q", draft.Message)
	}
	// Default channel
	if draft.Channel != "lark" {
		t.Errorf("expected default channel 'lark', got %q", draft.Channel)
	}

	expectedActions := []string{"Snooze 15min", "Dismiss", "Open thread"}
	if len(draft.SuggestedActions) != len(expectedActions) {
		t.Fatalf("expected %d actions, got %d", len(expectedActions), len(draft.SuggestedActions))
	}
	for i, action := range expectedActions {
		if draft.SuggestedActions[i] != action {
			t.Errorf("action[%d] = %q, want %q", i, draft.SuggestedActions[i], action)
		}
	}
}

func TestDraftBuilder_PastTriggerTime(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-past",
		SourceTitle: "Already started meeting",
		TriggerTime: fixedNow().Add(-10 * time.Minute), // in the past
		Context:     map[string]string{},
	}

	draft := builder.Build(intent)

	if !strings.Contains(draft.Message, "now") {
		t.Errorf("expected 'now' for past trigger time, got %q", draft.Message)
	}
}

func TestDraftBuilder_UnknownIntentType(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentType("unknown_type"),
		SourceID:    "x-001",
		SourceTitle: "Mystery item",
		TriggerTime: fixedNow().Add(10 * time.Minute),
		Context:     map[string]string{},
	}

	draft := builder.Build(intent)

	if !strings.Contains(draft.Message, "Mystery item") {
		t.Errorf("expected title in fallback message, got %q", draft.Message)
	}
	if len(draft.SuggestedActions) != 1 || draft.SuggestedActions[0] != "Dismiss" {
		t.Errorf("expected single 'Dismiss' action, got %v", draft.SuggestedActions)
	}
}

func TestDraftBuilder_IntentPreserved(t *testing.T) {
	builder := &DraftBuilder{Now: fixedNow}

	intent := ReminderIntent{
		Type:        IntentTaskDeadline,
		SourceID:    "task-preserve",
		SourceTitle: "Preserved",
		TriggerTime: fixedNow().Add(5 * time.Minute),
		Context:     map[string]string{"key": "value"},
	}

	draft := builder.Build(intent)

	if draft.Intent.SourceID != "task-preserve" {
		t.Errorf("expected intent SourceID to be preserved, got %q", draft.Intent.SourceID)
	}
	if draft.Intent.Context["key"] != "value" {
		t.Errorf("expected intent context to be preserved")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "now"},
		{"negative", -5 * time.Minute, "now"},
		{"minutes_only", 15 * time.Minute, "15m"},
		{"hours_only", 2 * time.Hour, "2h"},
		{"hours_and_minutes", 1*time.Hour + 30*time.Minute, "1h30m"},
		{"just_seconds", 30 * time.Second, "now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
