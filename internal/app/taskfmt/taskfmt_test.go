package taskfmt

import (
	"testing"
	"time"

	"alex/internal/domain/task"
)

func TestTruncate(t *testing.T) {
	if got := Truncate("hello world", 5); got != "he..." {
		t.Errorf("Truncate = %q, want he...", got)
	}
	if got := Truncate("hi", 10); got != "hi" {
		t.Errorf("Truncate short = %q, want hi", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30 minutes"},
		{1 * time.Minute, "1 minute"},
		{1 * time.Hour, "1 hour"},
		{6 * time.Hour, "6 hours"},
		{24 * time.Hour, "1 day"},
		{7 * 24 * time.Hour, "7 days"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatDurationCompact(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{90 * time.Minute, "1h30m"},
	}
	for _, tt := range tests {
		got := FormatDurationCompact(tt.d)
		if got != tt.want {
			t.Errorf("FormatDurationCompact(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestTaskLabel_FallbackToID(t *testing.T) {
	tk := &task.Task{TaskID: "abc123"}
	if got := TaskLabel(tk); got != "abc123" {
		t.Errorf("TaskLabel = %q, want abc123", got)
	}
}
