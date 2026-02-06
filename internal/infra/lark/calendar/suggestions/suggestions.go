// Package suggestions provides pure-computation meeting time suggestions
// based on free slots, historical patterns, and schedule constraints.
// It performs no API calls — callers supply pre-fetched events and constraints.
package suggestions

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// HourRange restricts suggestions to a preferred time-of-day window.
// StartHour and EndHour use 24-hour format (0-23).
type HourRange struct {
	StartHour int // inclusive, 0-23
	EndHour   int // exclusive, 0-23
}

// ScheduleConstraints defines the search parameters for meeting suggestions.
type ScheduleConstraints struct {
	RangeStart     time.Time      // beginning of the search window
	RangeEnd       time.Time      // end of the search window
	Duration       time.Duration  // desired meeting length
	Attendees      []string       // attendee IDs (informational; filtering is the caller's job)
	PreferredHours *HourRange     // optional preferred time-of-day window
	AvoidWeekends  bool           // skip Saturday and Sunday
	MaxSuggestions int            // cap on returned suggestions; defaults to 5
}

// CalendarSlot represents a free or busy window on a calendar.
type CalendarSlot struct {
	Start     time.Time
	End       time.Time
	Available bool
}

// ExistingEvent represents a calendar event fetched from an external source.
type ExistingEvent struct {
	EventID   string
	Summary   string
	StartTime time.Time
	EndTime   time.Time
	Status    string // "cancelled" events are ignored
}

// HistoricalPattern captures aggregated meeting behaviour derived from past events.
type HistoricalPattern struct {
	PreferredStartHour int             // most common meeting start hour (mode)
	PreferredDuration  time.Duration   // most common duration rounded to 15 min
	PreferredDays      []time.Weekday  // most common meeting days
	AvgMeetingsPerDay  float64         // average meetings per calendar day
}

// Suggestion is a ranked meeting-time proposal.
type Suggestion struct {
	Start   time.Time
	End     time.Time
	Score   float64  // 0-1, higher is better
	Reasons []string // human-readable explanations
}

// defaultMaxSuggestions is used when MaxSuggestions is zero.
const defaultMaxSuggestions = 5

// SuggestTimes returns ranked meeting time suggestions within the given constraints.
//
// It finds free windows from the supplied events, scores each candidate slot
// using the constraints and optional historical patterns, then returns the top
// results sorted by score descending.
func SuggestTimes(events []ExistingEvent, constraints ScheduleConstraints, patterns *HistoricalPattern) ([]Suggestion, error) {
	if constraints.RangeEnd.Before(constraints.RangeStart) || constraints.RangeEnd.Equal(constraints.RangeStart) {
		return nil, fmt.Errorf("suggestions: RangeEnd (%v) must be after RangeStart (%v)", constraints.RangeEnd, constraints.RangeStart)
	}
	if constraints.Duration <= 0 {
		return nil, fmt.Errorf("suggestions: Duration must be positive, got %v", constraints.Duration)
	}

	maxSugg := constraints.MaxSuggestions
	if maxSugg <= 0 {
		maxSugg = defaultMaxSuggestions
	}

	freeWindows := FindFreeWindows(events, constraints.RangeStart, constraints.RangeEnd, constraints.Duration)

	// Pre-compute meetings per day for pattern penalty.
	meetingsPerDay := countMeetingsPerDay(events, constraints.RangeStart, constraints.RangeEnd)

	var suggestions []Suggestion
	for _, window := range freeWindows {
		// Generate candidate slots from this window. We step through the window
		// in Duration increments to produce multiple candidates from long gaps.
		cursor := window.Start
		for !cursor.Add(constraints.Duration).After(window.End) {
			candidateEnd := cursor.Add(constraints.Duration)

			if constraints.AvoidWeekends && isWeekend(cursor) {
				cursor = cursor.Add(15 * time.Minute)
				continue
			}

			if constraints.PreferredHours != nil && !withinHourRange(cursor, candidateEnd, constraints.PreferredHours) {
				cursor = cursor.Add(15 * time.Minute)
				continue
			}

			score, reasons := scoreSlot(cursor, candidateEnd, constraints, patterns, meetingsPerDay)

			suggestions = append(suggestions, Suggestion{
				Start:   cursor,
				End:     candidateEnd,
				Score:   clampScore(score),
				Reasons: reasons,
			})

			cursor = cursor.Add(15 * time.Minute)
		}
	}

	// Sort by score descending; break ties by earlier start time.
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Score != suggestions[j].Score {
			return suggestions[i].Score > suggestions[j].Score
		}
		return suggestions[i].Start.Before(suggestions[j].Start)
	})

	if len(suggestions) > maxSugg {
		suggestions = suggestions[:maxSugg]
	}

	return suggestions, nil
}

// AnalyzePatterns derives a HistoricalPattern from a set of past events.
// Cancelled events are excluded.
func AnalyzePatterns(events []ExistingEvent) HistoricalPattern {
	var active []ExistingEvent
	for _, ev := range events {
		if ev.Status == "cancelled" {
			continue
		}
		active = append(active, ev)
	}

	if len(active) == 0 {
		return HistoricalPattern{}
	}

	// Most common start hour (mode).
	hourCounts := make(map[int]int)
	for _, ev := range active {
		hourCounts[ev.StartTime.Hour()]++
	}
	preferredHour := modeInt(hourCounts)

	// Most common duration, rounded to 15 min.
	durationCounts := make(map[time.Duration]int)
	for _, ev := range active {
		dur := roundTo15Min(ev.EndTime.Sub(ev.StartTime))
		durationCounts[dur]++
	}
	preferredDuration := modeDuration(durationCounts)

	// Most common days.
	dayCounts := make(map[time.Weekday]int)
	for _, ev := range active {
		dayCounts[ev.StartTime.Weekday()]++
	}
	preferredDays := topDays(dayCounts)

	// Average meetings per day.
	daySet := make(map[string]int)
	for _, ev := range active {
		key := ev.StartTime.Format("2006-01-02")
		daySet[key]++
	}
	var totalDays int
	for range daySet {
		totalDays++
	}
	avgPerDay := float64(len(active)) / float64(totalDays)

	return HistoricalPattern{
		PreferredStartHour: preferredHour,
		PreferredDuration:  preferredDuration,
		PreferredDays:      preferredDays,
		AvgMeetingsPerDay:  avgPerDay,
	}
}

// FindFreeWindows returns gaps between non-cancelled events that are at least
// minDuration long within [rangeStart, rangeEnd).
func FindFreeWindows(events []ExistingEvent, rangeStart, rangeEnd time.Time, minDuration time.Duration) []CalendarSlot {
	// Filter cancelled events and those outside the range.
	var active []ExistingEvent
	for _, ev := range events {
		if ev.Status == "cancelled" {
			continue
		}
		if ev.StartTime.Before(rangeEnd) && ev.EndTime.After(rangeStart) {
			active = append(active, ev)
		}
	}

	// Sort by start time; ties broken by longer event first for correct merging.
	sort.Slice(active, func(i, j int) bool {
		if active[i].StartTime.Equal(active[j].StartTime) {
			return active[j].EndTime.Before(active[i].EndTime)
		}
		return active[i].StartTime.Before(active[j].StartTime)
	})

	// Merge overlapping / adjacent busy intervals.
	type interval struct{ start, end time.Time }
	var merged []interval
	for _, ev := range active {
		s := ev.StartTime
		e := ev.EndTime
		if len(merged) > 0 && !s.After(merged[len(merged)-1].end) {
			if e.After(merged[len(merged)-1].end) {
				merged[len(merged)-1].end = e
			}
		} else {
			merged = append(merged, interval{start: s, end: e})
		}
	}

	// Walk merged blocks and collect free gaps.
	var slots []CalendarSlot
	cursor := rangeStart
	for _, blk := range merged {
		gapStart := cursor
		gapEnd := blk.start
		if gapStart.Before(rangeStart) {
			gapStart = rangeStart
		}
		if gapEnd.After(rangeEnd) {
			gapEnd = rangeEnd
		}
		if gapEnd.Sub(gapStart) >= minDuration {
			slots = append(slots, CalendarSlot{Start: gapStart, End: gapEnd, Available: true})
		}
		if blk.end.After(cursor) {
			cursor = blk.end
		}
	}

	// Trailing gap.
	if cursor.Before(rangeEnd) {
		gapStart := cursor
		gapEnd := rangeEnd
		if gapEnd.Sub(gapStart) >= minDuration {
			slots = append(slots, CalendarSlot{Start: gapStart, End: gapEnd, Available: true})
		}
	}

	return slots
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// scoreSlot computes a score and human-readable reasons for a candidate slot.
func scoreSlot(start, end time.Time, c ScheduleConstraints, p *HistoricalPattern, meetingsPerDay map[string]int) (float64, []string) {
	score := 0.5
	var reasons []string

	reasons = append(reasons, "No conflicts")

	// Preferred hours bonus.
	if c.PreferredHours != nil && withinHourRange(start, end, c.PreferredHours) {
		score += 0.2
		reasons = append(reasons, "Within preferred hours")
	}

	// Historical pattern bonuses.
	if p != nil {
		// Preferred day bonus.
		if containsWeekday(p.PreferredDays, start.Weekday()) {
			score += 0.15
			reasons = append(reasons, fmt.Sprintf("On preferred day (%s)", start.Weekday()))
		}

		// Preferred start hour bonus (within 1 hour).
		hourDiff := absInt(start.Hour() - p.PreferredStartHour)
		if hourDiff <= 1 {
			score += 0.1
			reasons = append(reasons, fmt.Sprintf("Near preferred start hour (%d:00)", p.PreferredStartHour))
		}

		// Busy day penalty.
		dayKey := start.Format("2006-01-02")
		if p.AvgMeetingsPerDay > 4 || meetingsPerDay[dayKey] >= 4 {
			score -= 0.1
			reasons = append(reasons, "Busy day (many meetings)")
		}
	}

	// Morning bonus.
	hour := start.Hour()
	if hour >= 9 && hour < 12 {
		score += 0.05
		reasons = append(reasons, "Morning slot (9-12)")
	}

	return score, reasons
}

// withinHourRange checks whether the entire [start, end) slot falls within the
// preferred hour range on the same day.
func withinHourRange(start, end time.Time, hr *HourRange) bool {
	startHour := start.Hour()
	endHour := end.Hour()
	endMin := end.Minute()

	// If end is exactly on the hour, the meeting ends at that hour boundary.
	// For a range like 9-17, a meeting 16:00-17:00 is fine (endHour==17, min==0).
	if endMin == 0 && endHour > 0 {
		endHour--
	}

	return startHour >= hr.StartHour && endHour < hr.EndHour
}

// isWeekend returns true for Saturday or Sunday.
func isWeekend(t time.Time) bool {
	day := t.Weekday()
	return day == time.Saturday || day == time.Sunday
}

// clampScore ensures the score stays within [0, 1].
func clampScore(s float64) float64 {
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	// Round to avoid floating-point noise.
	return math.Round(s*1000) / 1000
}

// countMeetingsPerDay counts non-cancelled events per calendar date within the range.
func countMeetingsPerDay(events []ExistingEvent, rangeStart, rangeEnd time.Time) map[string]int {
	counts := make(map[string]int)
	for _, ev := range events {
		if ev.Status == "cancelled" {
			continue
		}
		if ev.StartTime.Before(rangeEnd) && ev.EndTime.After(rangeStart) {
			key := ev.StartTime.Format("2006-01-02")
			counts[key]++
		}
	}
	return counts
}

// modeInt returns the key with the highest count. Ties broken by smallest key.
func modeInt(counts map[int]int) int {
	best := -1
	bestCount := 0
	for k, c := range counts {
		if c > bestCount || (c == bestCount && (best == -1 || k < best)) {
			best = k
			bestCount = c
		}
	}
	return best
}

// modeDuration returns the duration with the highest count.
func modeDuration(counts map[time.Duration]int) time.Duration {
	var best time.Duration
	bestCount := 0
	for k, c := range counts {
		if c > bestCount || (c == bestCount && k < best) {
			best = k
			bestCount = c
		}
	}
	return best
}

// roundTo15Min rounds a duration to the nearest 15-minute increment.
func roundTo15Min(d time.Duration) time.Duration {
	const q = 15 * time.Minute
	return ((d + q/2) / q) * q
}

// topDays returns the weekdays sorted by descending frequency, then by day value.
func topDays(counts map[time.Weekday]int) []time.Weekday {
	type entry struct {
		day   time.Weekday
		count int
	}
	var entries []entry
	for d, c := range counts {
		entries = append(entries, entry{day: d, count: c})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].day < entries[j].day
	})
	days := make([]time.Weekday, len(entries))
	for i, e := range entries {
		days[i] = e.day
	}
	return days
}

// containsWeekday checks membership in a weekday slice.
func containsWeekday(days []time.Weekday, d time.Weekday) bool {
	for _, day := range days {
		if day == d {
			return true
		}
	}
	return false
}

// absInt returns the absolute value of an integer.
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
