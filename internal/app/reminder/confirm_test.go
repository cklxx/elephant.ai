package reminder

import (
	"context"
	"testing"
	"time"
)

func TestAutoApproveGate(t *testing.T) {
	gate := AutoApproveGate{}

	draft := ReminderDraft{
		Intent: ReminderIntent{
			Type:        IntentCalendarReminder,
			SourceID:    "evt-001",
			SourceTitle: "Standup",
			TriggerTime: time.Now().Add(15 * time.Minute),
		},
		Message:          "Reminder: Standup starts in 15m.",
		Channel:          "lark",
		SuggestedActions: []string{"Snooze 15min", "Dismiss", "Open event"},
	}

	result, err := gate.RequestConfirmation(context.Background(), draft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Approved {
		t.Error("expected Approved=true from AutoApproveGate")
	}
	if result.Action != "auto_approved" {
		t.Errorf("expected Action='auto_approved', got %q", result.Action)
	}
	if result.ModifiedMessage != "" {
		t.Errorf("expected empty ModifiedMessage, got %q", result.ModifiedMessage)
	}
}

func TestAutoApproveGate_IgnoresContext(t *testing.T) {
	gate := AutoApproveGate{}

	// Even with a cancelled context, AutoApproveGate should work.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := gate.RequestConfirmation(ctx, ReminderDraft{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Approved {
		t.Error("expected Approved=true even with cancelled context")
	}
}
