// Package notification defines the shared notification contract used by the
// scheduler and timer subsystems.
package notification

import "context"

// Channel constants identify supported notification channels.
const (
	ChannelLark     = "lark"
	ChannelMoltbook = "moltbook"
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
