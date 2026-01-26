package builtin

import (
	"context"

	agent "alex/internal/agent/ports/agent"
)

// WithSessionID adds a session ID to the context using the shared SessionContextKey
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, agent.SessionContextKey{}, sessionID)
}

// GetSessionID retrieves the session ID from the context
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(agent.SessionContextKey{}).(string)
	return sessionID, ok
}
