package agent

import "context"

// TmuxSender sends input to tmux panes.
type TmuxSender interface {
	SendKeys(ctx context.Context, pane string, data string) error
}
