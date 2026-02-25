package context

import (
	"strings"
	"testing"
	"time"

	"alex/internal/infra/lark"
)

func refTime() time.Time {
	return time.Date(2026, time.February, 1, 10, 30, 0, 0, time.UTC)
}

func TestBuild_NoEvents(t *testing.T) {
	b := &CalendarSummaryBuilder{}

	if got := b.Build(nil, refTime()); got != "" {
		t.Fatalf("expected empty string for nil events, got %q", got)
	}
	if got := b.Build([]lark.CalendarEvent{}, refTime()); got != "" {
		t.Fatalf("expected empty string for empty events, got %q", got)
	}
}

func TestBuild_CurrentEvent(t *testing.T) {
	now := refTime()
	events := []lark.CalendarEvent{
		{
			EventID:   "e1",
			Summary:   "Standup",
			StartTime: now.Add(-15 * time.Minute),
			EndTime:   now.Add(15 * time.Minute),
			Location:  "Room A",
			Status:    "confirmed",
		},
	}

	b := &CalendarSummaryBuilder{}
	got := b.Build(events, now)

	if !strings.Contains(got, "Now:") {
		t.Fatalf("expected Now section, got %q", got)
	}
	if !strings.Contains(got, "10:15-10:45 Standup [Room A]") {
		t.Fatalf("expected formatted event with location, got %q", got)
	}
}

func TestBuild_GroupsByPeriod(t *testing.T) {
	now := refTime() // 2026-02-01 10:30 UTC

	events := []lark.CalendarEvent{
		{
			EventID:   "now-1",
			Summary:   "Current Meeting",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(20 * time.Minute),
			Status:    "confirmed",
		},
		{
			EventID:   "next-1",
			Summary:   "Next Meeting",
			StartTime: now.Add(1 * time.Hour),
			EndTime:   now.Add(2 * time.Hour),
			Location:  "Room B",
			Status:    "confirmed",
		},
		{
			EventID:   "today-1",
			Summary:   "Afternoon Sync",
			StartTime: now.Add(4 * time.Hour),
			EndTime:   now.Add(5 * time.Hour),
			Status:    "confirmed",
		},
		{
			EventID:   "tomorrow-1",
			Summary:   "Planning",
			StartTime: now.Add(24 * time.Hour),
			EndTime:   now.Add(25 * time.Hour),
			Location:  "Room C",
			Status:    "confirmed",
		},
	}

	b := &CalendarSummaryBuilder{}
	got := b.Build(events, now)

	sections := []string{"Now:", "Next:", "Today:", "Tomorrow:"}
	for _, s := range sections {
		if !strings.Contains(got, s) {
			t.Fatalf("expected section %q in output, got %q", s, got)
		}
	}

	if !strings.Contains(got, "Current Meeting") {
		t.Fatalf("expected Current Meeting in Now section, got %q", got)
	}
	if !strings.Contains(got, "Next Meeting") {
		t.Fatalf("expected Next Meeting in Next section, got %q", got)
	}
	if !strings.Contains(got, "Afternoon Sync") {
		t.Fatalf("expected Afternoon Sync in Today section, got %q", got)
	}
	if !strings.Contains(got, "Planning") {
		t.Fatalf("expected Planning in Tomorrow section, got %q", got)
	}
	if !strings.Contains(got, "[Room B]") {
		t.Fatalf("expected location for Next Meeting, got %q", got)
	}
	if !strings.Contains(got, "[Room C]") {
		t.Fatalf("expected location for Planning, got %q", got)
	}

	// Verify ordering: Now < Next < Today < Tomorrow.
	nowIdx := strings.Index(got, "Now:")
	nextIdx := strings.Index(got, "Next:")
	todayIdx := strings.Index(got, "Today:")
	tomorrowIdx := strings.Index(got, "Tomorrow:")
	if nowIdx >= nextIdx || nextIdx >= todayIdx || todayIdx >= tomorrowIdx {
		t.Fatalf("sections out of order: Now@%d Next@%d Today@%d Tomorrow@%d",
			nowIdx, nextIdx, todayIdx, tomorrowIdx)
	}
}

func TestBuild_SkipsCancelled(t *testing.T) {
	now := refTime()
	events := []lark.CalendarEvent{
		{
			EventID:   "cancelled-1",
			Summary:   "Cancelled Standup",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(20 * time.Minute),
			Status:    "cancelled",
		},
		{
			EventID:   "active-1",
			Summary:   "Active Standup",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(20 * time.Minute),
			Status:    "confirmed",
		},
	}

	b := &CalendarSummaryBuilder{}
	got := b.Build(events, now)

	if strings.Contains(got, "Cancelled Standup") {
		t.Fatalf("expected cancelled event to be skipped, got %q", got)
	}
	if !strings.Contains(got, "Active Standup") {
		t.Fatalf("expected active event to be present, got %q", got)
	}
}

func TestBuild_TruncatesLongOutput(t *testing.T) {
	now := refTime()
	var events []lark.CalendarEvent
	for i := 0; i < 40; i++ {
		events = append(events, lark.CalendarEvent{
			EventID:   "e" + time.Duration(i).String(),
			Summary:   "Meeting with a rather lengthy title to fill space " + time.Duration(i).String(),
			StartTime: now.Add(time.Duration(i+1) * 30 * time.Minute),
			EndTime:   now.Add(time.Duration(i+1)*30*time.Minute + time.Hour),
			Location:  "Conference Room " + time.Duration(i).String(),
			Status:    "confirmed",
		})
	}

	b := &CalendarSummaryBuilder{}
	got := b.Build(events, now)

	if len(got) > calendarSummaryMaxChars {
		t.Fatalf("expected output to be at most %d chars, got %d: %q",
			calendarSummaryMaxChars, len(got), got)
	}
	if !strings.Contains(got, "... and") {
		t.Fatalf("expected truncation indicator, got %q", got)
	}
	if !strings.Contains(got, "more events") {
		t.Fatalf("expected 'more events' suffix, got %q", got)
	}
}

func TestBuild_NoLocationOmitsBrackets(t *testing.T) {
	now := refTime()
	events := []lark.CalendarEvent{
		{
			EventID:   "e1",
			Summary:   "Quick Chat",
			StartTime: now.Add(1 * time.Hour),
			EndTime:   now.Add(2 * time.Hour),
			Status:    "confirmed",
		},
	}

	b := &CalendarSummaryBuilder{}
	got := b.Build(events, now)

	if strings.Contains(got, "[") || strings.Contains(got, "]") {
		t.Fatalf("expected no brackets when location is empty, got %q", got)
	}
	if !strings.Contains(got, "Quick Chat") {
		t.Fatalf("expected event summary in output, got %q", got)
	}
}

func TestBuild_AllCancelledReturnsEmpty(t *testing.T) {
	now := refTime()
	events := []lark.CalendarEvent{
		{
			EventID:   "c1",
			Summary:   "Cancelled A",
			StartTime: now.Add(1 * time.Hour),
			EndTime:   now.Add(2 * time.Hour),
			Status:    "cancelled",
		},
		{
			EventID:   "c2",
			Summary:   "Cancelled B",
			StartTime: now.Add(3 * time.Hour),
			EndTime:   now.Add(4 * time.Hour),
			Status:    "cancelled",
		},
	}

	b := &CalendarSummaryBuilder{}
	if got := b.Build(events, now); got != "" {
		t.Fatalf("expected empty string when all events cancelled, got %q", got)
	}
}

func TestBuild_PastEventsExcluded(t *testing.T) {
	now := refTime()
	events := []lark.CalendarEvent{
		{
			EventID:   "past-1",
			Summary:   "Past Event",
			StartTime: now.Add(-2 * time.Hour),
			EndTime:   now.Add(-1 * time.Hour),
			Status:    "confirmed",
		},
	}

	b := &CalendarSummaryBuilder{}
	if got := b.Build(events, now); got != "" {
		t.Fatalf("expected empty string for past events, got %q", got)
	}
}
