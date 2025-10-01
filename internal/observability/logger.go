package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog for structured logging
type Logger struct {
	logger *slog.Logger
}

// LogConfig configures the logger
type LogConfig struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output io.Writer
}

// NewLogger creates a new structured logger
func NewLogger(config LogConfig) *Logger {
	// Default to info level
	level := slog.LevelInfo
	switch strings.ToLower(config.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// Default to stdout
	output := config.Output
	if output == nil {
		output = os.Stdout
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if config.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{
		logger: slog.New(handler),
	}
}

// WithContext adds context fields to logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract trace_id and session_id from context if available
	var args []any

	if traceID := TraceIDFromContext(ctx); traceID != "" {
		args = append(args, "trace_id", traceID)
	}

	if sessionID := SessionIDFromContext(ctx); sessionID != "" {
		args = append(args, "session_id", sessionID)
	}

	if len(args) == 0 {
		return l
	}

	return &Logger{
		logger: l.logger.With(args...),
	}
}

// With adds additional fields to the logger
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger: l.logger.With(args...),
	}
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs at info level
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs at warn level
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs at error level
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// DebugContext logs at debug level with context
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Debug(msg, args...)
}

// InfoContext logs at info level with context
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Info(msg, args...)
}

// WarnContext logs at warn level with context
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Warn(msg, args...)
}

// ErrorContext logs at error level with context
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).Error(msg, args...)
}

// SanitizeAPIKey masks API key for security
func SanitizeAPIKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// Context key types
type contextKey string

const (
	traceIDKey   contextKey = "trace_id"
	sessionIDKey contextKey = "session_id"
)

// ContextWithTraceID adds trace ID to context
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext extracts trace ID from context
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// ContextWithSessionID adds session ID to context
func ContextWithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// SessionIDFromContext extracts session ID from context
func SessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(sessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}
