package reminder

import (
	"context"
	"fmt"
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

// mockConfirmSender implements ConfirmSender for testing ChannelConfirmationGate.
type mockConfirmSender struct {
	result ConfirmationResult
	err    error
	called bool
}

func (m *mockConfirmSender) SendConfirmation(_ context.Context, _ ReminderDraft) (ConfirmationResult, error) {
	m.called = true
	return m.result, m.err
}

func TestChannelConfirmationGate_DelegatesToSender(t *testing.T) {
	sender := &mockConfirmSender{
		result: ConfirmationResult{
			Approved:        true,
			Action:          "Open event",
			ModifiedMessage: "",
		},
	}

	gate := NewChannelConfirmationGate(sender)

	draft := ReminderDraft{
		Message: "Test reminder",
		Channel: "lark",
	}

	result, err := gate.RequestConfirmation(context.Background(), draft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sender.called {
		t.Error("expected sender to be called")
	}
	if !result.Approved {
		t.Error("expected Approved=true")
	}
	if result.Action != "Open event" {
		t.Errorf("expected Action='Open event', got %q", result.Action)
	}
}

func TestChannelConfirmationGate_PropagatesError(t *testing.T) {
	sender := &mockConfirmSender{
		err: fmt.Errorf("channel unavailable"),
	}

	gate := NewChannelConfirmationGate(sender)

	_, err := gate.RequestConfirmation(context.Background(), ReminderDraft{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "channel unavailable" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestChannelConfirmationGate_DismissResult(t *testing.T) {
	sender := &mockConfirmSender{
		result: ConfirmationResult{
			Approved: false,
			Action:   "Dismiss",
		},
	}

	gate := NewChannelConfirmationGate(sender)
	result, err := gate.RequestConfirmation(context.Background(), ReminderDraft{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Approved {
		t.Error("expected Approved=false for dismiss")
	}
	if result.Action != "Dismiss" {
		t.Errorf("expected Action='Dismiss', got %q", result.Action)
	}
}

func TestChannelConfirmationGate_ModifiedMessage(t *testing.T) {
	sender := &mockConfirmSender{
		result: ConfirmationResult{
			Approved:        true,
			Action:          "Send",
			ModifiedMessage: "Updated reminder text",
		},
	}

	gate := NewChannelConfirmationGate(sender)
	result, err := gate.RequestConfirmation(context.Background(), ReminderDraft{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModifiedMessage != "Updated reminder text" {
		t.Errorf("expected modified message, got %q", result.ModifiedMessage)
	}
}
