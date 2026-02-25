package context

import "context"

type sessionHistoryKey struct{}

// WithSessionHistory enables or disables injecting session history into context.
// Default behavior is enabled when no value is set.
func WithSessionHistory(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, sessionHistoryKey{}, enabled)
}

// SessionHistoryEnabled returns true when session history injection is enabled.
// Defaults to true when unset or context is nil.
func SessionHistoryEnabled(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	if enabled, ok := ctx.Value(sessionHistoryKey{}).(bool); ok {
		return enabled
	}
	return true
}
