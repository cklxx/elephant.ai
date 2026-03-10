// Package notification defines the shared notification contract used by the
// scheduler and timer subsystems.
package notification

import "context"

// Channel constants identify supported notification channels.
const (
	ChannelLark     = "lark"
	ChannelMoltbook = "moltbook"
)

// AlertOutcome describes the lifecycle state of a leader notification.
type AlertOutcome string

const (
	OutcomeSent      AlertOutcome = "sent"
	OutcomeDelivered AlertOutcome = "delivered"
	OutcomeFailed    AlertOutcome = "failed"
	OutcomeOpened    AlertOutcome = "opened"
	OutcomeDismissed AlertOutcome = "dismissed"
	OutcomeActedOn   AlertOutcome = "acted_on"
)

// Target identifies where to send a notification.
type Target struct {
	Channel string // ChannelLark, ChannelMoltbook
	ChatID  string // for lark
}

// Notifier routes messages to external channels.
type Notifier interface {
	Send(ctx context.Context, target Target, content string) error
}

// OutcomeRecorder records alert lifecycle outcomes for telemetry.
type OutcomeRecorder interface {
	RecordAlertOutcome(ctx context.Context, feature, channel string, outcome AlertOutcome)
}
