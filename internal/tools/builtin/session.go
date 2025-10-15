package builtin

import (
	"context"

	"alex/internal/agent/ports"
)

// WithSessionID adds a session ID to the context using the shared SessionContextKey
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ports.SessionContextKey{}, sessionID)
}

// GetSessionID retrieves the session ID from the context
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(ports.SessionContextKey{}).(string)
	return sessionID, ok
}
