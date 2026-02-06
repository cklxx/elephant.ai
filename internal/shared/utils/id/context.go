package id

import "context"

type contextKey string

const (
	sessionKey     contextKey = "alex_session_id"
	runKey         contextKey = "alex_run_id"
	parentRunKey   contextKey = "alex_parent_run_id"
	userKey        contextKey = "alex_user_id"
	logKey         contextKey = "alex_log_id"
	correlationKey contextKey = "alex_correlation_id"
	causationKey   contextKey = "alex_causation_id"
)

// SessionContextKey is the shared context key for storing session IDs across packages.
// This ensures consistent session ID propagation from server layer to agent layer.
type SessionContextKey struct{}

// IDs captures the identifiers propagated across agent execution boundaries.
type IDs struct {
	SessionID     string
	RunID         string
	ParentRunID   string
	LogID         string
	CorrelationID string
	CausationID   string
}

// WithSessionID stores the provided session identifier on the context.
// It also populates the shared SessionContextKey for cross-package access.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, sessionKey, sessionID)
	ctx = context.WithValue(ctx, SessionContextKey{}, sessionID)
	return ctx
}

// WithUserID stores the authenticated user identifier on the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, userKey, userID)
}

// WithRunID stores the current run identifier on the context.
func WithRunID(ctx context.Context, runID string) context.Context {
	if runID == "" {
		return ctx
	}
	return context.WithValue(ctx, runKey, runID)
}

// WithParentRunID stores the parent run identifier (if any) on the context.
func WithParentRunID(ctx context.Context, parentRunID string) context.Context {
	if parentRunID == "" {
		return ctx
	}
	return context.WithValue(ctx, parentRunKey, parentRunID)
}

// WithCorrelationID stores the correlation identifier on the context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	if correlationID == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationKey, correlationID)
}

// WithCausationID stores the causation identifier on the context.
func WithCausationID(ctx context.Context, causationID string) context.Context {
	if causationID == "" {
		return ctx
	}
	return context.WithValue(ctx, causationKey, causationID)
}

// WithIDs stores any provided identifiers on the context.
func WithIDs(ctx context.Context, ids IDs) context.Context {
	if ids.SessionID != "" {
		ctx = WithSessionID(ctx, ids.SessionID)
	}
	if ids.RunID != "" {
		ctx = WithRunID(ctx, ids.RunID)
	}
	if ids.ParentRunID != "" {
		ctx = WithParentRunID(ctx, ids.ParentRunID)
	}
	if ids.LogID != "" {
		ctx = WithLogID(ctx, ids.LogID)
	}
	if ids.CorrelationID != "" {
		ctx = WithCorrelationID(ctx, ids.CorrelationID)
	}
	if ids.CausationID != "" {
		ctx = WithCausationID(ctx, ids.CausationID)
	}
	return ctx
}

// SessionIDFromContext extracts the session identifier from context.
func SessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if sessionID, ok := ctx.Value(sessionKey).(string); ok && sessionID != "" {
		return sessionID
	}
	if sessionID, ok := ctx.Value(SessionContextKey{}).(string); ok {
		return sessionID
	}
	return ""
}

// RunIDFromContext extracts the run identifier from context.
func RunIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if runID, ok := ctx.Value(runKey).(string); ok {
		return runID
	}
	return ""
}

// ParentRunIDFromContext extracts the parent run identifier from context.
func ParentRunIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if parentID, ok := ctx.Value(parentRunKey).(string); ok {
		return parentID
	}
	return ""
}

// UserIDFromContext extracts the authenticated user identifier from context.
func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if userID, ok := ctx.Value(userKey).(string); ok {
		return userID
	}
	return ""
}

// WithLogID stores the provided log identifier on the context.
func WithLogID(ctx context.Context, logID string) context.Context {
	if logID == "" {
		return ctx
	}
	return context.WithValue(ctx, logKey, logID)
}

// LogIDFromContext extracts the log identifier from context.
func LogIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if logID, ok := ctx.Value(logKey).(string); ok {
		return logID
	}
	return ""
}

// CorrelationIDFromContext extracts the correlation identifier from context.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if cid, ok := ctx.Value(correlationKey).(string); ok {
		return cid
	}
	return ""
}

// CausationIDFromContext extracts the causation identifier from context.
func CausationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if cid, ok := ctx.Value(causationKey).(string); ok {
		return cid
	}
	return ""
}

// IDsFromContext collects all known identifiers from the context.
func IDsFromContext(ctx context.Context) IDs {
	return IDs{
		SessionID:     SessionIDFromContext(ctx),
		RunID:         RunIDFromContext(ctx),
		ParentRunID:   ParentRunIDFromContext(ctx),
		LogID:         LogIDFromContext(ctx),
		CorrelationID: CorrelationIDFromContext(ctx),
		CausationID:   CausationIDFromContext(ctx),
	}
}

// EnsureRunID guarantees a run identifier is present on the context.
// It returns the updated context and the resulting identifier.
func EnsureRunID(ctx context.Context, generator func() string) (context.Context, string) {
	if existing := RunIDFromContext(ctx); existing != "" {
		return ctx, existing
	}
	next := ""
	if generator != nil {
		next = generator()
	}
	if next == "" {
		return ctx, ""
	}
	ctx = WithRunID(ctx, next)
	return ctx, next
}

// EnsureLogID guarantees a log identifier is present on the context.
// It returns the updated context and the resulting identifier.
func EnsureLogID(ctx context.Context, generator func() string) (context.Context, string) {
	if existing := LogIDFromContext(ctx); existing != "" {
		return ctx, existing
	}
	next := ""
	if generator != nil {
		next = generator()
	}
	if next == "" {
		return ctx, ""
	}
	ctx = WithLogID(ctx, next)
	return ctx, next
}
