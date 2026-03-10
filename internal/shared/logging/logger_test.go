package logging

import (
	"testing"

	"alex/internal/shared/utils"
)

func TestOrNopHandlesTypedNilPointers(t *testing.T) {
	var legacy *utils.Logger
	var logger Logger = legacy
	if !IsNil(logger) {
		t.Fatalf("expected typed nil pointer to be detected")
	}
	safe := OrNop(logger)
	if IsNil(safe) {
		t.Fatalf("expected OrNop to return a usable logger")
	}
	safe.Info("hello %s", "world") // should not panic
}

type captureLogger struct {
	receivedLogID string
}

func (*captureLogger) Debug(string, ...any) {}
func (*captureLogger) Info(string, ...any)  {}
func (*captureLogger) Warn(string, ...any)  {}
func (*captureLogger) Error(string, ...any) {}

func (l *captureLogger) WithLogID(logID string) Logger {
	return &captureLogger{receivedLogID: logID}
}

func TestWithLogIDTagsMultiLoggerChildren(t *testing.T) {
	first := &captureLogger{}
	second := &captureLogger{}

	tagged := WithLogID(Multi(first, second), "log-123")
	ml, ok := tagged.(*multiLogger)
	if !ok {
		t.Fatalf("expected multiLogger, got %T", tagged)
	}
	if len(ml.loggers) != 2 {
		t.Fatalf("expected 2 child loggers, got %d", len(ml.loggers))
	}

	for i, child := range ml.loggers {
		captured, ok := child.(*captureLogger)
		if !ok {
			t.Fatalf("child %d has unexpected type %T", i, child)
		}
		if captured.receivedLogID != "log-123" {
			t.Fatalf("child %d received log id %q", i, captured.receivedLogID)
		}
	}
}
