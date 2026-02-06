package reminder

import "context"

// ConfirmationResult captures the user's response to a reminder draft.
type ConfirmationResult struct {
	Approved        bool   // whether the reminder was approved for delivery
	Action          string // which suggested action was chosen (e.g. "Dismiss", "Snooze 15min")
	ModifiedMessage string // non-empty if the user edited the message
}

// ConfirmationGate requests user confirmation before a reminder is sent.
type ConfirmationGate interface {
	RequestConfirmation(ctx context.Context, draft ReminderDraft) (ConfirmationResult, error)
}

// ConfirmSender is the interface for delivering a confirmation request to a channel
// and receiving the user's response. Implementations are channel-specific (Lark, web, etc.).
type ConfirmSender interface {
	SendConfirmation(ctx context.Context, draft ReminderDraft) (ConfirmationResult, error)
}

// AutoApproveGate always approves the reminder without user interaction.
// Useful for testing and CI pipelines.
type AutoApproveGate struct{}

// RequestConfirmation immediately approves the draft.
func (AutoApproveGate) RequestConfirmation(_ context.Context, _ ReminderDraft) (ConfirmationResult, error) {
	return ConfirmationResult{
		Approved: true,
		Action:   "auto_approved",
	}, nil
}

// ChannelConfirmationGate delegates confirmation to a ConfirmSender implementation.
type ChannelConfirmationGate struct {
	Sender ConfirmSender
}

// NewChannelConfirmationGate creates a ChannelConfirmationGate with the given sender.
func NewChannelConfirmationGate(sender ConfirmSender) *ChannelConfirmationGate {
	return &ChannelConfirmationGate{Sender: sender}
}

// RequestConfirmation delegates to the underlying ConfirmSender.
func (g *ChannelConfirmationGate) RequestConfirmation(ctx context.Context, draft ReminderDraft) (ConfirmationResult, error) {
	return g.Sender.SendConfirmation(ctx, draft)
}
