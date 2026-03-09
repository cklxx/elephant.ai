package coordinator

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// capturingLogger records formatted log messages for assertions.
type capturingLogger struct {
	entries []string
}

func (l *capturingLogger) Debug(format string, args ...interface{}) {
	l.entries = append(l.entries, fmt.Sprintf(format, args...))
}
func (l *capturingLogger) Info(format string, args ...interface{}) {
	l.entries = append(l.entries, fmt.Sprintf(format, args...))
}
func (l *capturingLogger) Warn(format string, args ...interface{}) {
	l.entries = append(l.entries, fmt.Sprintf(format, args...))
}
func (l *capturingLogger) Error(format string, args ...interface{}) {
	l.entries = append(l.entries, fmt.Sprintf(format, args...))
}

func TestStepTimer_TrackAndLogStep(t *testing.T) {
	timer := newStepTimer()
	logger := &capturingLogger{}

	start := time.Now()
	time.Sleep(time.Millisecond)
	timer.track("prepare_context", start)

	rec := timer.records[0]
	timer.logStep(logger, "task-123", rec)

	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logger.entries))
	}

	entry := logger.entries[0]
	if !strings.Contains(entry, "step_timing") {
		t.Errorf("expected 'step_timing' prefix, got: %s", entry)
	}
	if !strings.Contains(entry, "task_id=task-123") {
		t.Errorf("expected task_id, got: %s", entry)
	}
	if !strings.Contains(entry, "step=prepare_context") {
		t.Errorf("expected step=prepare_context, got: %s", entry)
	}
	if !strings.Contains(entry, "duration_ms=") {
		t.Errorf("expected duration_ms, got: %s", entry)
	}
	if rec.DurationMs < 1.0 {
		t.Errorf("expected duration >= 1ms, got %.2fms", rec.DurationMs)
	}
}

func TestStepTimer_LogSummary(t *testing.T) {
	timer := newStepTimer()
	logger := &capturingLogger{}

	// Simulate multiple steps.
	for _, name := range []string{"prepare_context", "pre_hooks", "execute", "finalize"} {
		start := time.Now()
		time.Sleep(time.Millisecond)
		timer.track(name, start)
	}

	timer.logSummary(logger, "run-abc")

	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 summary entry, got %d", len(logger.entries))
	}

	summary := logger.entries[0]
	if !strings.Contains(summary, "task_timing") {
		t.Errorf("expected 'task_timing' prefix, got: %s", summary)
	}
	if !strings.Contains(summary, "task_id=run-abc") {
		t.Errorf("expected task_id, got: %s", summary)
	}
	if !strings.Contains(summary, "total_ms=") {
		t.Errorf("expected total_ms, got: %s", summary)
	}
	// Verify all step names appear in summary.
	for _, name := range []string{"prepare_context", "pre_hooks", "execute", "finalize"} {
		if !strings.Contains(summary, name) {
			t.Errorf("expected step %q in summary, got: %s", name, summary)
		}
	}
	// Verify steps array format.
	if !strings.Contains(summary, "steps=[{") {
		t.Errorf("expected steps=[ format, got: %s", summary)
	}
}

func TestStepTimer_EmptyRecords(t *testing.T) {
	timer := newStepTimer()
	logger := &capturingLogger{}

	timer.logSummary(logger, "empty-task")

	if len(logger.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(logger.entries))
	}
	if !strings.Contains(logger.entries[0], "steps=[]") {
		t.Errorf("expected empty steps array, got: %s", logger.entries[0])
	}
}
