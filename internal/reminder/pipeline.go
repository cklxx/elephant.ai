package reminder

import (
	"context"
	"fmt"
	"time"
)

// OutcomeStatus describes the result of a reminder pipeline execution.
type OutcomeStatus string

const (
	OutcomeSent      OutcomeStatus = "sent"
	OutcomeSnoozed   OutcomeStatus = "snoozed"
	OutcomeDismissed OutcomeStatus = "dismissed"
	OutcomeModified  OutcomeStatus = "modified"
)

// ReminderOutcome captures the full result of processing a reminder intent.
type ReminderOutcome struct {
	Draft      ReminderDraft
	Result     ConfirmationResult
	Status     OutcomeStatus
	ExecutedAt time.Time
}

// Pipeline orchestrates the reminder flow: intent -> draft -> confirm -> outcome.
type Pipeline struct {
	Builder *DraftBuilder
	Gate    ConfirmationGate
	// Now returns the current time; injectable for testing.
	Now func() time.Time
}

// NewPipeline creates a Pipeline with the given builder and confirmation gate.
func NewPipeline(builder *DraftBuilder, gate ConfirmationGate) *Pipeline {
	return &Pipeline{
		Builder: builder,
		Gate:    gate,
		Now:     time.Now,
	}
}

// Execute runs the full reminder pipeline: build draft, request confirmation, derive outcome.
func (p *Pipeline) Execute(ctx context.Context, intent ReminderIntent) (ReminderOutcome, error) {
	draft := p.Builder.Build(intent)

	result, err := p.Gate.RequestConfirmation(ctx, draft)
	if err != nil {
		return ReminderOutcome{}, fmt.Errorf("reminder confirmation: %w", err)
	}

	status := deriveStatus(result)

	// If user modified the message, update the draft message for the outcome.
	if result.ModifiedMessage != "" {
		draft.Message = result.ModifiedMessage
	}

	return ReminderOutcome{
		Draft:      draft,
		Result:     result,
		Status:     status,
		ExecutedAt: p.Now(),
	}, nil
}

// deriveStatus maps a ConfirmationResult to an OutcomeStatus.
func deriveStatus(r ConfirmationResult) OutcomeStatus {
	if !r.Approved {
		switch r.Action {
		case "Dismiss":
			return OutcomeDismissed
		default:
			return OutcomeSnoozed
		}
	}
	if r.ModifiedMessage != "" {
		return OutcomeModified
	}
	return OutcomeSent
}
