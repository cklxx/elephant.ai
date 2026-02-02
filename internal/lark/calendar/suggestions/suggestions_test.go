package suggestions

import (
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// date builds a time.Time in UTC from y-m-d h:m for concise test fixtures.
func date(year, month, day, hour, min int) time.Time {
	return time.Date(year, time.Month(month), day, hour, min, 0, 0, time.UTC)
}

// makeEvent is a shorthand for building ExistingEvent fixtures.
func makeEvent(id, summary string, start, end time.Time, status string) ExistingEvent {
	return ExistingEvent{
		EventID:   id,
		Summary:   summary,
		StartTime: start,
		EndTime:   end,
		Status:    status,
	}
}

// ---------------------------------------------------------------------------
// FindFreeWindows tests
// ---------------------------------------------------------------------------

func TestFindFreeWindows_NoEvents(t *testing.T) {
	rangeStart := date(2026, 2, 2, 9, 0)
	rangeEnd := date(2026, 2, 2, 17, 0)

	slots := FindFreeWindows(nil, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 1 {
		t.Fatalf("expected 1 free window, got %d", len(slots))
	}
	if !slots[0].Start.Equal(rangeStart) || !slots[0].End.Equal(rangeEnd) {
		t.Errorf("expected full range as free window, got %v - %v", slots[0].Start, slots[0].End)
	}
	if !slots[0].Available {
		t.Error("expected Available = true")
	}
}

func TestFindFreeWindows_FullDayBusy(t *testing.T) {
	rangeStart := date(2026, 2, 2, 9, 0)
	rangeEnd := date(2026, 2, 2, 17, 0)

	events := []ExistingEvent{
		makeEvent("1", "All day", rangeStart, rangeEnd, "confirmed"),
	}

	slots := FindFreeWindows(events, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 0 {
		t.Fatalf("expected 0 free windows when day is fully booked, got %d", len(slots))
	}
}

func TestFindFreeWindows_OverlappingEvents(t *testing.T) {
	rangeStart := date(2026, 2, 2, 9, 0)
	rangeEnd := date(2026, 2, 2, 17, 0)

	events := []ExistingEvent{
		makeEvent("1", "Meeting A", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 11, 30), "confirmed"),
		makeEvent("2", "Meeting B", date(2026, 2, 2, 11, 0), date(2026, 2, 2, 12, 0), "confirmed"),
		makeEvent("3", "Meeting C", date(2026, 2, 2, 14, 0), date(2026, 2, 2, 15, 0), "confirmed"),
	}

	slots := FindFreeWindows(events, rangeStart, rangeEnd, 30*time.Minute)

	// Expect: 9:00-10:00 (1h), 12:00-14:00 (2h), 15:00-17:00 (2h)
	if len(slots) != 3 {
		t.Fatalf("expected 3 free windows, got %d", len(slots))
	}

	expected := []struct{ start, end time.Time }{
		{date(2026, 2, 2, 9, 0), date(2026, 2, 2, 10, 0)},
		{date(2026, 2, 2, 12, 0), date(2026, 2, 2, 14, 0)},
		{date(2026, 2, 2, 15, 0), date(2026, 2, 2, 17, 0)},
	}
	for i, exp := range expected {
		if !slots[i].Start.Equal(exp.start) || !slots[i].End.Equal(exp.end) {
			t.Errorf("slot[%d]: expected %v-%v, got %v-%v", i, exp.start, exp.end, slots[i].Start, slots[i].End)
		}
	}
}

func TestFindFreeWindows_CancelledEventsIgnored(t *testing.T) {
	rangeStart := date(2026, 2, 2, 9, 0)
	rangeEnd := date(2026, 2, 2, 17, 0)

	events := []ExistingEvent{
		makeEvent("1", "Cancelled", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 11, 0), "cancelled"),
	}

	slots := FindFreeWindows(events, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 1 {
		t.Fatalf("expected 1 free window (cancelled ignored), got %d", len(slots))
	}
	if !slots[0].Start.Equal(rangeStart) || !slots[0].End.Equal(rangeEnd) {
		t.Error("expected full range when only event is cancelled")
	}
}

func TestFindFreeWindows_MinDurationFilter(t *testing.T) {
	rangeStart := date(2026, 2, 2, 9, 0)
	rangeEnd := date(2026, 2, 2, 12, 0)

	events := []ExistingEvent{
		makeEvent("1", "Short gap", date(2026, 2, 2, 9, 15), date(2026, 2, 2, 11, 45), "confirmed"),
	}

	// 15-min gaps at start and end — too short for 30 min.
	slots := FindFreeWindows(events, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 0 {
		t.Fatalf("expected 0 free windows (gaps too short), got %d", len(slots))
	}

	// But fine for 15-min meetings.
	slots = FindFreeWindows(events, rangeStart, rangeEnd, 15*time.Minute)
	if len(slots) != 2 {
		t.Fatalf("expected 2 free windows for 15-min minimum, got %d", len(slots))
	}
}

func TestFindFreeWindows_EventsOutsideRange(t *testing.T) {
	rangeStart := date(2026, 2, 2, 9, 0)
	rangeEnd := date(2026, 2, 2, 12, 0)

	events := []ExistingEvent{
		makeEvent("1", "Before range", date(2026, 2, 2, 7, 0), date(2026, 2, 2, 8, 0), "confirmed"),
		makeEvent("2", "After range", date(2026, 2, 2, 13, 0), date(2026, 2, 2, 14, 0), "confirmed"),
	}

	slots := FindFreeWindows(events, rangeStart, rangeEnd, 30*time.Minute)
	if len(slots) != 1 {
		t.Fatalf("expected 1 free window (events outside range), got %d", len(slots))
	}
	if !slots[0].Start.Equal(rangeStart) || !slots[0].End.Equal(rangeEnd) {
		t.Error("expected full range when events are outside")
	}
}

// ---------------------------------------------------------------------------
// AnalyzePatterns tests
// ---------------------------------------------------------------------------

func TestAnalyzePatterns_Empty(t *testing.T) {
	p := AnalyzePatterns(nil)
	if p.PreferredStartHour != 0 || p.PreferredDuration != 0 || len(p.PreferredDays) != 0 {
		t.Errorf("expected zero pattern for empty events, got %+v", p)
	}
}

func TestAnalyzePatterns_AllCancelled(t *testing.T) {
	events := []ExistingEvent{
		makeEvent("1", "A", date(2026, 1, 5, 10, 0), date(2026, 1, 5, 11, 0), "cancelled"),
		makeEvent("2", "B", date(2026, 1, 6, 14, 0), date(2026, 1, 6, 15, 0), "cancelled"),
	}
	p := AnalyzePatterns(events)
	if p.PreferredStartHour != 0 || p.PreferredDuration != 0 {
		t.Errorf("expected zero pattern for all-cancelled events, got %+v", p)
	}
}

func TestAnalyzePatterns_Computation(t *testing.T) {
	// 5 events: 3 at 10:00, 1 at 14:00, 1 at 10:00
	// Durations: 60m, 60m, 30m, 60m, 90m
	// Days: Mon, Mon, Tue, Wed, Mon
	events := []ExistingEvent{
		makeEvent("1", "A", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 11, 0), "confirmed"),  // Mon, 10, 60m
		makeEvent("2", "B", date(2026, 2, 2, 10, 30), date(2026, 2, 2, 11, 30), "confirmed"), // Mon, 10, 60m
		makeEvent("3", "C", date(2026, 2, 3, 10, 0), date(2026, 2, 3, 10, 30), "confirmed"),  // Tue, 10, 30m
		makeEvent("4", "D", date(2026, 2, 4, 14, 0), date(2026, 2, 4, 15, 0), "confirmed"),   // Wed, 14, 60m
		makeEvent("5", "E", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 11, 30), "confirmed"),  // Mon, 10, 90m
	}

	p := AnalyzePatterns(events)

	// Most common start hour: 10 (4 times) vs 14 (1 time)
	if p.PreferredStartHour != 10 {
		t.Errorf("expected preferred start hour 10, got %d", p.PreferredStartHour)
	}

	// Most common duration: 60m (3 times, rounded)
	if p.PreferredDuration != 60*time.Minute {
		t.Errorf("expected preferred duration 60m, got %v", p.PreferredDuration)
	}

	// Most common day: Monday (3 events), then Tue (1), Wed (1)
	if len(p.PreferredDays) == 0 || p.PreferredDays[0] != time.Monday {
		t.Errorf("expected Monday as top preferred day, got %v", p.PreferredDays)
	}

	// Avg meetings per day: 5 events across 3 days = ~1.67
	expectedAvg := 5.0 / 3.0
	if math.Abs(p.AvgMeetingsPerDay-expectedAvg) > 0.01 {
		t.Errorf("expected avg meetings/day ~%.2f, got %.2f", expectedAvg, p.AvgMeetingsPerDay)
	}
}

func TestAnalyzePatterns_RoundTo15Min(t *testing.T) {
	events := []ExistingEvent{
		// Duration: 37 minutes — rounds to 30m? No: 37 is closer to 30 (diff=7) than 45 (diff=8) → 30m.
		// Actually: (37+7.5)/15 = 2.97 → 2*15 = 30. Wait, let me recalculate:
		// roundTo15Min: (37m + 7.5m) / 15m = 44.5m/15m = 2 (integer) * 15m = 30m.
		makeEvent("1", "A", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 10, 37), "confirmed"),
		makeEvent("2", "B", date(2026, 2, 3, 10, 0), date(2026, 2, 3, 10, 37), "confirmed"),
	}

	p := AnalyzePatterns(events)
	if p.PreferredDuration != 30*time.Minute {
		t.Errorf("expected 37min rounded to 30min, got %v", p.PreferredDuration)
	}
}

// ---------------------------------------------------------------------------
// SuggestTimes tests
// ---------------------------------------------------------------------------

func TestSuggestTimes_InvalidRange(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart: date(2026, 2, 2, 17, 0),
		RangeEnd:   date(2026, 2, 2, 9, 0),
		Duration:   30 * time.Minute,
	}
	_, err := SuggestTimes(nil, constraints, nil)
	if err == nil {
		t.Error("expected error for invalid range")
	}
}

func TestSuggestTimes_InvalidDuration(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart: date(2026, 2, 2, 9, 0),
		RangeEnd:   date(2026, 2, 2, 17, 0),
		Duration:   0,
	}
	_, err := SuggestTimes(nil, constraints, nil)
	if err == nil {
		t.Error("expected error for zero duration")
	}
}

func TestSuggestTimes_BasicNoEvents(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 9, 0), // Monday
		RangeEnd:       date(2026, 2, 2, 12, 0),
		Duration:       30 * time.Minute,
		MaxSuggestions: 3,
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %d", len(suggestions))
	}

	// All should have score >= 0.5 (base) and have "No conflicts" reason.
	for i, s := range suggestions {
		if s.Score < 0.5 {
			t.Errorf("suggestion[%d]: score %.3f below base 0.5", i, s.Score)
		}
		if len(s.Reasons) == 0 {
			t.Errorf("suggestion[%d]: expected reasons", i)
		}
	}
}

func TestSuggestTimes_DefaultMaxSuggestions(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart: date(2026, 2, 2, 9, 0),
		RangeEnd:   date(2026, 2, 2, 17, 0),
		Duration:   30 * time.Minute,
		// MaxSuggestions not set — defaults to 5
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) > defaultMaxSuggestions {
		t.Errorf("expected at most %d suggestions, got %d", defaultMaxSuggestions, len(suggestions))
	}
}

func TestSuggestTimes_PreferredHoursScoring(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart: date(2026, 2, 2, 8, 0), // Monday
		RangeEnd:   date(2026, 2, 2, 18, 0),
		Duration:   1 * time.Hour,
		PreferredHours: &HourRange{
			StartHour: 10,
			EndHour:   12,
		},
		MaxSuggestions: 10,
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}

	// All suggestions should be within preferred hours since we filter.
	for _, s := range suggestions {
		if s.Start.Hour() < 10 || s.Start.Hour() >= 12 {
			t.Errorf("suggestion at %v is outside preferred hours 10-12", s.Start)
		}
	}
}

func TestSuggestTimes_WithHistoricalPatterns(t *testing.T) {
	patterns := &HistoricalPattern{
		PreferredStartHour: 10,
		PreferredDuration:  1 * time.Hour,
		PreferredDays:      []time.Weekday{time.Monday, time.Wednesday},
		AvgMeetingsPerDay:  2.0,
	}

	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 9, 0), // Monday
		RangeEnd:       date(2026, 2, 2, 17, 0),
		Duration:       1 * time.Hour,
		MaxSuggestions: 20,
	}

	suggestions, err := SuggestTimes(nil, constraints, patterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions")
	}

	// The top suggestion should benefit from preferred day (Monday) and preferred start hour.
	top := suggestions[0]

	// Monday + near hour 10 + morning should yield high score.
	if top.Score <= 0.5 {
		t.Errorf("top suggestion score %.3f should be above base 0.5", top.Score)
	}

	// Check it has pattern-related reasons.
	hasPreferredDay := false
	for _, r := range top.Reasons {
		if r == "On preferred day (Monday)" {
			hasPreferredDay = true
		}
	}
	if !hasPreferredDay {
		t.Errorf("expected 'On preferred day (Monday)' reason in top suggestion, got %v", top.Reasons)
	}
}

func TestSuggestTimes_AvoidsWeekends(t *testing.T) {
	// Saturday 2026-02-07
	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 7, 9, 0), // Saturday
		RangeEnd:       date(2026, 2, 7, 17, 0),
		Duration:       1 * time.Hour,
		AvoidWeekends:  true,
		MaxSuggestions: 10,
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions on weekend with AvoidWeekends=true, got %d", len(suggestions))
	}
}

func TestSuggestTimes_AvoidsWeekends_WeekdaysKept(t *testing.T) {
	// Span Friday to Monday to test filtering.
	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 6, 9, 0),  // Friday
		RangeEnd:       date(2026, 2, 9, 17, 0),  // Monday
		Duration:       1 * time.Hour,
		AvoidWeekends:  true,
		MaxSuggestions: 100,
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range suggestions {
		if isWeekend(s.Start) {
			t.Errorf("suggestion on weekend: %v (%s)", s.Start, s.Start.Weekday())
		}
	}
}

func TestSuggestTimes_ScoreRanking(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 8, 0), // Monday
		RangeEnd:       date(2026, 2, 2, 18, 0),
		Duration:       1 * time.Hour,
		MaxSuggestions: 50,
	}

	patterns := &HistoricalPattern{
		PreferredStartHour: 10,
		PreferredDuration:  1 * time.Hour,
		PreferredDays:      []time.Weekday{time.Monday},
		AvgMeetingsPerDay:  2.0,
	}

	suggestions, err := SuggestTimes(nil, constraints, patterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify descending score order.
	for i := 1; i < len(suggestions); i++ {
		if suggestions[i].Score > suggestions[i-1].Score {
			t.Errorf("suggestions not sorted: [%d].Score=%.3f > [%d].Score=%.3f",
				i, suggestions[i].Score, i-1, suggestions[i-1].Score)
		}
	}

	// Verify all scores are in [0, 1].
	for i, s := range suggestions {
		if s.Score < 0 || s.Score > 1 {
			t.Errorf("suggestion[%d]: score %.3f outside [0,1]", i, s.Score)
		}
	}
}

func TestSuggestTimes_WithConflicts(t *testing.T) {
	events := []ExistingEvent{
		makeEvent("1", "Standup", date(2026, 2, 2, 9, 0), date(2026, 2, 2, 9, 30), "confirmed"),
		makeEvent("2", "Lunch", date(2026, 2, 2, 12, 0), date(2026, 2, 2, 13, 0), "confirmed"),
	}

	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 9, 0),
		RangeEnd:       date(2026, 2, 2, 14, 0),
		Duration:       1 * time.Hour,
		MaxSuggestions: 20,
	}

	suggestions, err := SuggestTimes(events, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No suggestion should overlap with existing events.
	for _, s := range suggestions {
		for _, ev := range events {
			if ev.Status == "cancelled" {
				continue
			}
			if s.Start.Before(ev.EndTime) && s.End.After(ev.StartTime) {
				t.Errorf("suggestion %v-%v overlaps with event %s (%v-%v)",
					s.Start, s.End, ev.EventID, ev.StartTime, ev.EndTime)
			}
		}
	}
}

func TestSuggestTimes_BusyDayPenalty(t *testing.T) {
	// Create a day with 5 events so AvgMeetingsPerDay > 4.
	events := []ExistingEvent{
		makeEvent("1", "M1", date(2026, 2, 2, 8, 0), date(2026, 2, 2, 8, 30), "confirmed"),
		makeEvent("2", "M2", date(2026, 2, 2, 8, 30), date(2026, 2, 2, 9, 0), "confirmed"),
		makeEvent("3", "M3", date(2026, 2, 2, 13, 0), date(2026, 2, 2, 13, 30), "confirmed"),
		makeEvent("4", "M4", date(2026, 2, 2, 13, 30), date(2026, 2, 2, 14, 0), "confirmed"),
		makeEvent("5", "M5", date(2026, 2, 2, 16, 0), date(2026, 2, 2, 16, 30), "confirmed"),
	}

	patterns := &HistoricalPattern{
		PreferredStartHour: 10,
		PreferredDuration:  30 * time.Minute,
		PreferredDays:      []time.Weekday{time.Monday},
		AvgMeetingsPerDay:  5.0, // > 4 triggers penalty
	}

	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 9, 0),
		RangeEnd:       date(2026, 2, 2, 13, 0),
		Duration:       30 * time.Minute,
		MaxSuggestions: 5,
	}

	suggestions, err := SuggestTimes(events, constraints, patterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify busy day penalty is applied — check reasons.
	hasBusyReason := false
	for _, s := range suggestions {
		for _, r := range s.Reasons {
			if r == "Busy day (many meetings)" {
				hasBusyReason = true
				break
			}
		}
	}
	if !hasBusyReason {
		t.Error("expected 'Busy day (many meetings)' reason due to high avg meetings per day")
	}
}

func TestSuggestTimes_MaxSuggestionsLimit(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 9, 0),
		RangeEnd:       date(2026, 2, 2, 17, 0),
		Duration:       30 * time.Minute,
		MaxSuggestions: 3,
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 3 {
		t.Errorf("expected exactly 3 suggestions, got %d", len(suggestions))
	}
}

func TestSuggestTimes_MorningBonus(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart:     date(2026, 2, 2, 9, 0), // Monday
		RangeEnd:       date(2026, 2, 2, 17, 0),
		Duration:       1 * time.Hour,
		MaxSuggestions: 50,
	}

	suggestions, err := SuggestTimes(nil, constraints, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find a morning slot and an afternoon slot.
	var morningScore, afternoonScore float64
	for _, s := range suggestions {
		if s.Start.Hour() == 10 && morningScore == 0 {
			morningScore = s.Score
		}
		if s.Start.Hour() == 15 && afternoonScore == 0 {
			afternoonScore = s.Score
		}
	}

	if morningScore <= afternoonScore {
		t.Errorf("morning slot (score %.3f) should score higher than afternoon (score %.3f)",
			morningScore, afternoonScore)
	}
}

// ---------------------------------------------------------------------------
// Internal helper tests
// ---------------------------------------------------------------------------

func TestRoundTo15Min(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected time.Duration
	}{
		{0, 0},
		{7 * time.Minute, 0},             // 7 rounds down to 0
		{8 * time.Minute, 15 * time.Minute},  // 8 rounds up to 15
		{15 * time.Minute, 15 * time.Minute},
		{22 * time.Minute, 15 * time.Minute}, // 22 rounds down to 15
		{23 * time.Minute, 30 * time.Minute}, // 23 rounds up to 30
		{30 * time.Minute, 30 * time.Minute},
		{37 * time.Minute, 30 * time.Minute}, // 37 rounds down to 30
		{45 * time.Minute, 45 * time.Minute},
		{60 * time.Minute, 60 * time.Minute},
		{90 * time.Minute, 90 * time.Minute},
	}

	for _, tt := range tests {
		got := roundTo15Min(tt.input)
		if got != tt.expected {
			t.Errorf("roundTo15Min(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestWithinHourRange(t *testing.T) {
	hr := &HourRange{StartHour: 9, EndHour: 17}

	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected bool
	}{
		{"inside", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 11, 0), true},
		{"at start boundary", date(2026, 2, 2, 9, 0), date(2026, 2, 2, 10, 0), true},
		{"at end boundary", date(2026, 2, 2, 16, 0), date(2026, 2, 2, 17, 0), true},
		{"before start", date(2026, 2, 2, 8, 0), date(2026, 2, 2, 9, 0), false},
		{"after end", date(2026, 2, 2, 17, 0), date(2026, 2, 2, 18, 0), false},
		{"spanning end", date(2026, 2, 2, 16, 30), date(2026, 2, 2, 17, 30), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withinHourRange(tt.start, tt.end, hr)
			if got != tt.expected {
				t.Errorf("withinHourRange(%v, %v, 9-17) = %v, want %v", tt.start, tt.end, got, tt.expected)
			}
		})
	}
}

func TestIsWeekend(t *testing.T) {
	tests := []struct {
		date     time.Time
		expected bool
	}{
		{date(2026, 2, 2, 0, 0), false},  // Monday
		{date(2026, 2, 6, 0, 0), false},  // Friday
		{date(2026, 2, 7, 0, 0), true},   // Saturday
		{date(2026, 2, 8, 0, 0), true},   // Sunday
	}

	for _, tt := range tests {
		got := isWeekend(tt.date)
		if got != tt.expected {
			t.Errorf("isWeekend(%v %s) = %v, want %v", tt.date, tt.date.Weekday(), got, tt.expected)
		}
	}
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.5, 0.5},
		{1.5, 1.0},
		{-0.3, 0.0},
		{0.0, 0.0},
		{1.0, 1.0},
		{0.755, 0.755},
	}

	for _, tt := range tests {
		got := clampScore(tt.input)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("clampScore(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestContainsWeekday(t *testing.T) {
	days := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	if !containsWeekday(days, time.Monday) {
		t.Error("expected Monday to be found")
	}
	if containsWeekday(days, time.Tuesday) {
		t.Error("expected Tuesday not to be found")
	}
	if containsWeekday(nil, time.Monday) {
		t.Error("expected empty slice to not contain Monday")
	}
}

func TestAbsInt(t *testing.T) {
	tests := []struct {
		input, expected int
	}{
		{5, 5},
		{-3, 3},
		{0, 0},
	}
	for _, tt := range tests {
		if got := absInt(tt.input); got != tt.expected {
			t.Errorf("absInt(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge case: equal range start and end
// ---------------------------------------------------------------------------

func TestSuggestTimes_EqualRange(t *testing.T) {
	constraints := ScheduleConstraints{
		RangeStart: date(2026, 2, 2, 9, 0),
		RangeEnd:   date(2026, 2, 2, 9, 0),
		Duration:   30 * time.Minute,
	}
	_, err := SuggestTimes(nil, constraints, nil)
	if err == nil {
		t.Error("expected error for equal range start/end")
	}
}

// ---------------------------------------------------------------------------
// Integration-style: full workflow
// ---------------------------------------------------------------------------

func TestSuggestTimes_FullWorkflow(t *testing.T) {
	// Simulate a realistic day: events spread across Monday.
	events := []ExistingEvent{
		makeEvent("1", "Standup", date(2026, 2, 2, 9, 0), date(2026, 2, 2, 9, 30), "confirmed"),
		makeEvent("2", "Design Review", date(2026, 2, 2, 10, 0), date(2026, 2, 2, 11, 0), "confirmed"),
		makeEvent("3", "Lunch", date(2026, 2, 2, 12, 0), date(2026, 2, 2, 13, 0), "confirmed"),
		makeEvent("4", "Cancelled 1:1", date(2026, 2, 2, 14, 0), date(2026, 2, 2, 14, 30), "cancelled"),
		makeEvent("5", "Sprint Planning", date(2026, 2, 2, 15, 0), date(2026, 2, 2, 16, 0), "confirmed"),
	}

	// Derive patterns from the same events.
	patterns := AnalyzePatterns(events)

	constraints := ScheduleConstraints{
		RangeStart: date(2026, 2, 2, 9, 0),
		RangeEnd:   date(2026, 2, 2, 17, 0),
		Duration:   30 * time.Minute,
		PreferredHours: &HourRange{
			StartHour: 9,
			EndHour:   17,
		},
		MaxSuggestions: 5,
	}

	suggestions, err := SuggestTimes(events, constraints, &patterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}
	if len(suggestions) > 5 {
		t.Errorf("expected at most 5 suggestions, got %d", len(suggestions))
	}

	// No suggestion should overlap with non-cancelled events.
	for _, s := range suggestions {
		for _, ev := range events {
			if ev.Status == "cancelled" {
				continue
			}
			if s.Start.Before(ev.EndTime) && s.End.After(ev.StartTime) {
				t.Errorf("suggestion %v-%v overlaps with event %q (%v-%v)",
					s.Start, s.End, ev.Summary, ev.StartTime, ev.EndTime)
			}
		}
	}

	// Verify sorted by score descending.
	for i := 1; i < len(suggestions); i++ {
		if suggestions[i].Score > suggestions[i-1].Score {
			t.Errorf("not sorted: [%d].Score=%.3f > [%d].Score=%.3f",
				i, suggestions[i].Score, i-1, suggestions[i-1].Score)
		}
	}
}
