package calendar

import (
	"context"
	"testing"
	"time"

	domain "alex/internal/domain/calendar"
)

// Compile-time check that LarkCalendarProvider implements CalendarPort.
var _ domain.CalendarPort = (*LarkCalendarProvider)(nil)

func TestLarkCalendarProvider_MissingCredentials(t *testing.T) {
	provider := NewLarkCalendarProvider(LarkCalendarConfig{}, nil)

	_, err := provider.ListUpcoming1on1s(context.Background(), "alice", 30*time.Minute)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
	if got := err.Error(); got != "lark calendar: missing app credentials" {
		t.Errorf("error = %q, want missing credentials message", got)
	}
}

func TestLarkCalendarProvider_StubReturnsEmpty(t *testing.T) {
	provider := NewLarkCalendarProvider(LarkCalendarConfig{
		AppID:     "cli_test",
		AppSecret: "secret",
	}, nil)

	meetings, err := provider.ListUpcoming1on1s(context.Background(), "alice", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meetings) != 0 {
		t.Errorf("stub should return empty slice, got %d meetings", len(meetings))
	}
}
