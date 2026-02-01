package reminder

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPipeline_FullFlow_AutoApprove(t *testing.T) {
	now := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	gate := AutoApproveGate{}
	pipeline := &Pipeline{
		Builder: builder,
		Gate:    gate,
		Now:     func() time.Time { return now },
	}

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-pipeline",
		SourceTitle: "Architecture Review",
		TriggerTime: now.Add(20 * time.Minute),
		Context: map[string]string{
			"location": "Building A",
			"channel":  "lark",
		},
	}

	outcome, err := pipeline.Execute(context.Background(), intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.Status != OutcomeSent {
		t.Errorf("expected status 'sent', got %q", outcome.Status)
	}
	if !outcome.Result.Approved {
		t.Error("expected approved=true")
	}
	if outcome.ExecutedAt != now {
		t.Errorf("expected ExecutedAt=%v, got %v", now, outcome.ExecutedAt)
	}
	if !strings.Contains(outcome.Draft.Message, "Architecture Review") {
		t.Errorf("expected draft message to contain title, got %q", outcome.Draft.Message)
	}
	if outcome.Draft.Channel != "lark" {
		t.Errorf("expected channel 'lark', got %q", outcome.Draft.Channel)
	}
}

func TestPipeline_TaskDeadline(t *testing.T) {
	now := time.Date(2026, 2, 1, 14, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	pipeline := NewPipeline(builder, AutoApproveGate{})
	pipeline.Now = func() time.Time { return now }

	intent := ReminderIntent{
		Type:        IntentTaskDeadline,
		SourceID:    "task-pipeline",
		SourceTitle: "Deploy v2.0",
		TriggerTime: now.Add(1 * time.Hour),
		Context:     map[string]string{},
	}

	outcome, err := pipeline.Execute(context.Background(), intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.Status != OutcomeSent {
		t.Errorf("expected status 'sent', got %q", outcome.Status)
	}
	if !strings.Contains(outcome.Draft.Message, "Deploy v2.0") {
		t.Errorf("expected title in message, got %q", outcome.Draft.Message)
	}
	if !strings.Contains(outcome.Draft.Message, "due in") {
		t.Errorf("expected 'due in' in message, got %q", outcome.Draft.Message)
	}
}

func TestPipeline_FollowUp(t *testing.T) {
	now := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	pipeline := NewPipeline(builder, AutoApproveGate{})
	pipeline.Now = func() time.Time { return now }

	intent := ReminderIntent{
		Type:        IntentFollowUp,
		SourceID:    "thread-pipeline",
		SourceTitle: "Hiring decision",
		TriggerTime: now.Add(5 * time.Minute),
		Context:     map[string]string{},
	}

	outcome, err := pipeline.Execute(context.Background(), intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.Status != OutcomeSent {
		t.Errorf("expected status 'sent', got %q", outcome.Status)
	}
	if !strings.Contains(outcome.Draft.Message, "Follow up on:") {
		t.Errorf("expected 'Follow up on:' in message, got %q", outcome.Draft.Message)
	}
}

// dismissGate is a mock that always dismisses.
type dismissGate struct{}

func (dismissGate) RequestConfirmation(_ context.Context, _ ReminderDraft) (ConfirmationResult, error) {
	return ConfirmationResult{
		Approved: false,
		Action:   "Dismiss",
	}, nil
}

func TestPipeline_Dismissed(t *testing.T) {
	now := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	pipeline := NewPipeline(builder, dismissGate{})
	pipeline.Now = func() time.Time { return now }

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-dismiss",
		SourceTitle: "Skipped meeting",
		TriggerTime: now.Add(10 * time.Minute),
		Context:     map[string]string{},
	}

	outcome, err := pipeline.Execute(context.Background(), intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.Status != OutcomeDismissed {
		t.Errorf("expected status 'dismissed', got %q", outcome.Status)
	}
	if outcome.Result.Approved {
		t.Error("expected approved=false for dismissed")
	}
}

// snoozeGate is a mock that always snoozes.
type snoozeGate struct{}

func (snoozeGate) RequestConfirmation(_ context.Context, _ ReminderDraft) (ConfirmationResult, error) {
	return ConfirmationResult{
		Approved: false,
		Action:   "Snooze 15min",
	}, nil
}

func TestPipeline_Snoozed(t *testing.T) {
	now := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	pipeline := NewPipeline(builder, snoozeGate{})
	pipeline.Now = func() time.Time { return now }

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-snooze",
		SourceTitle: "Snoozed meeting",
		TriggerTime: now.Add(10 * time.Minute),
		Context:     map[string]string{},
	}

	outcome, err := pipeline.Execute(context.Background(), intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.Status != OutcomeSnoozed {
		t.Errorf("expected status 'snoozed', got %q", outcome.Status)
	}
	if outcome.Result.Action != "Snooze 15min" {
		t.Errorf("expected action 'Snooze 15min', got %q", outcome.Result.Action)
	}
}

// modifyGate approves but modifies the message.
type modifyGate struct{}

func (modifyGate) RequestConfirmation(_ context.Context, _ ReminderDraft) (ConfirmationResult, error) {
	return ConfirmationResult{
		Approved:        true,
		Action:          "Send",
		ModifiedMessage: "Custom edited reminder",
	}, nil
}

func TestPipeline_Modified(t *testing.T) {
	now := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	pipeline := NewPipeline(builder, modifyGate{})
	pipeline.Now = func() time.Time { return now }

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-modify",
		SourceTitle: "Modified meeting",
		TriggerTime: now.Add(10 * time.Minute),
		Context:     map[string]string{},
	}

	outcome, err := pipeline.Execute(context.Background(), intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outcome.Status != OutcomeModified {
		t.Errorf("expected status 'modified', got %q", outcome.Status)
	}
	if outcome.Draft.Message != "Custom edited reminder" {
		t.Errorf("expected modified message in draft, got %q", outcome.Draft.Message)
	}
}

// errorGate always returns an error.
type errorGate struct{}

func (errorGate) RequestConfirmation(_ context.Context, _ ReminderDraft) (ConfirmationResult, error) {
	return ConfirmationResult{}, fmt.Errorf("confirmation service unavailable")
}

func TestPipeline_ConfirmationError(t *testing.T) {
	now := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	builder := &DraftBuilder{Now: func() time.Time { return now }}
	pipeline := NewPipeline(builder, errorGate{})

	intent := ReminderIntent{
		Type:        IntentCalendarReminder,
		SourceID:    "evt-err",
		SourceTitle: "Error meeting",
		TriggerTime: now.Add(10 * time.Minute),
		Context:     map[string]string{},
	}

	_, err := pipeline.Execute(context.Background(), intent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "confirmation service unavailable") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "reminder confirmation") {
		t.Errorf("expected wrapped error with 'reminder confirmation', got: %v", err)
	}
}

func TestNewPipeline(t *testing.T) {
	builder := NewDraftBuilder()
	gate := AutoApproveGate{}
	pipeline := NewPipeline(builder, gate)

	if pipeline.Builder == nil {
		t.Error("expected non-nil builder")
	}
	if pipeline.Gate == nil {
		t.Error("expected non-nil gate")
	}
	if pipeline.Now == nil {
		t.Error("expected non-nil Now func")
	}
}
