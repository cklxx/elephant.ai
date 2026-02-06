package shared

import (
	"context"

	id "alex/internal/shared/utils/id"
)

// WithSessionID adds a session ID to the context using the shared SessionContextKey
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, id.SessionContextKey{}, sessionID)
}

// GetSessionID retrieves the session ID from the context
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(id.SessionContextKey{}).(string)
	return sessionID, ok
}
