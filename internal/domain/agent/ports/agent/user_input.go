package agent

import "context"

// UserInput represents a message injected by a user into a running ReAct loop.
type UserInput struct {
	Content   string
	SenderID  string
	MessageID string
}

type userInputChKey struct{}

// WithUserInputCh attaches a read-only user input channel to the context.
func WithUserInputCh(ctx context.Context, ch <-chan UserInput) context.Context {
	return context.WithValue(ctx, userInputChKey{}, ch)
}

// UserInputChFromContext retrieves the user input channel from the context.
// Returns nil when no channel has been attached.
func UserInputChFromContext(ctx context.Context) <-chan UserInput {
	if ctx == nil {
		return nil
	}
	if ch, ok := ctx.Value(userInputChKey{}).(<-chan UserInput); ok {
		return ch
	}
	return nil
}
