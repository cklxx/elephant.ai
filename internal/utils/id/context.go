package id

import (
	"context"

	agentports "alex/internal/agent/ports"
)

type contextKey string

const (
	sessionKey    contextKey = "alex_session_id"
	taskKey       contextKey = "alex_task_id"
	parentTaskKey contextKey = "alex_parent_task_id"
)

// IDs captures the identifiers propagated across agent execution boundaries.
type IDs struct {
	SessionID    string
	TaskID       string
	ParentTaskID string
}

// WithSessionID stores the provided session identifier on the context.
// It also populates the shared SessionContextKey for backward compatibility.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, sessionKey, sessionID)
	ctx = context.WithValue(ctx, agentports.SessionContextKey{}, sessionID)
	return ctx
}

// WithTaskID stores the current task identifier on the context.
func WithTaskID(ctx context.Context, taskID string) context.Context {
	if taskID == "" {
		return ctx
	}
	return context.WithValue(ctx, taskKey, taskID)
}

// WithParentTaskID stores the parent task identifier (if any) on the context.
func WithParentTaskID(ctx context.Context, parentTaskID string) context.Context {
	if parentTaskID == "" {
		return ctx
	}
	return context.WithValue(ctx, parentTaskKey, parentTaskID)
}

// WithIDs stores any provided identifiers on the context.
func WithIDs(ctx context.Context, ids IDs) context.Context {
	if ids.SessionID != "" {
		ctx = WithSessionID(ctx, ids.SessionID)
	}
	if ids.TaskID != "" {
		ctx = WithTaskID(ctx, ids.TaskID)
	}
	if ids.ParentTaskID != "" {
		ctx = WithParentTaskID(ctx, ids.ParentTaskID)
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
	if sessionID, ok := ctx.Value(agentports.SessionContextKey{}).(string); ok {
		return sessionID
	}
	return ""
}

// TaskIDFromContext extracts the task identifier from context.
func TaskIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if taskID, ok := ctx.Value(taskKey).(string); ok {
		return taskID
	}
	return ""
}

// ParentTaskIDFromContext extracts the parent task identifier from context.
func ParentTaskIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if parentID, ok := ctx.Value(parentTaskKey).(string); ok {
		return parentID
	}
	return ""
}

// IDsFromContext collects all known identifiers from the context.
func IDsFromContext(ctx context.Context) IDs {
	return IDs{
		SessionID:    SessionIDFromContext(ctx),
		TaskID:       TaskIDFromContext(ctx),
		ParentTaskID: ParentTaskIDFromContext(ctx),
	}
}

// EnsureTaskID guarantees a task identifier is present on the context.
// It returns the updated context and the resulting identifier.
func EnsureTaskID(ctx context.Context, generator func() string) (context.Context, string) {
	if existing := TaskIDFromContext(ctx); existing != "" {
		return ctx, existing
	}
	next := ""
	if generator != nil {
		next = generator()
	}
	if next == "" {
		return ctx, ""
	}
	ctx = WithTaskID(ctx, next)
	return ctx, next
}
