// Package notification defines the shared notification contract used by the
// scheduler and timer subsystems.
package notification

import "context"

// Target identifies where to send a notification.
type Target struct {
	Channel string // "lark", "moltbook"
	ChatID  string // for lark
}

// Notifier routes messages to external channels.
type Notifier interface {
	Send(ctx context.Context, target Target, content string) error
}
