package contextkeys

import "context"

// silentModeKey is used to suppress event output in subagent execution
type silentModeKey struct{}

// WithSilentMode marks the context to suppress event output
func WithSilentMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, silentModeKey{}, true)
}

// IsSilentMode checks if the context is in silent mode
func IsSilentMode(ctx context.Context) bool {
	val, ok := ctx.Value(silentModeKey{}).(bool)
	return ok && val
}
