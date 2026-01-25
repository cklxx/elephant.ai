package logging

import (
	"context"

	"alex/internal/utils"
	"alex/internal/utils/id"
)

type logIDCapable interface {
	WithLogID(string) Logger
}

type utilsLogIDCapable interface {
	WithLogID(string) *utils.Logger
}

// WithLogID returns a logger that tags log lines with a log id.
func WithLogID(logger Logger, logID string) Logger {
	if IsNil(logger) {
		return Nop()
	}
	if logID == "" {
		return logger
	}
	if capable, ok := logger.(logIDCapable); ok {
		return capable.WithLogID(logID)
	}
	if capable, ok := logger.(utilsLogIDCapable); ok {
		return capable.WithLogID(logID)
	}
	return &logIDLogger{logger: OrNop(logger), logID: logID}
}

// FromContext returns a logger tagged with the log id found in context, if any.
func FromContext(ctx context.Context, logger Logger) Logger {
	return WithLogID(logger, id.LogIDFromContext(ctx))
}

type logIDLogger struct {
	logger Logger
	logID  string
}

func (l *logIDLogger) Debug(format string, args ...any) {
	l.logger.Debug(prefixLogID(l.logID, format), args...)
}

func (l *logIDLogger) Info(format string, args ...any) {
	l.logger.Info(prefixLogID(l.logID, format), args...)
}

func (l *logIDLogger) Warn(format string, args ...any) {
	l.logger.Warn(prefixLogID(l.logID, format), args...)
}

func (l *logIDLogger) Error(format string, args ...any) {
	l.logger.Error(prefixLogID(l.logID, format), args...)
}

func prefixLogID(logID, format string) string {
	if logID == "" {
		return format
	}
	return "logid=" + logID + " " + format
}
