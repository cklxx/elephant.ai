package lark

import (
	"sort"
	"time"
)

// ConflictResult holds the outcome of a conflict detection check.
type ConflictResult struct {
	HasConflict bool
	Conflicts   []CalendarEvent
}

// TimeSlot represents a contiguous free window on the calendar.
type TimeSlot struct {
	Start time.Time
	End   time.Time
}

// DetectConflicts returns all non-cancelled events that overlap with the
// proposed [startTime, endTime) window.
//
// Two intervals overlap when: event.StartTime < endTime AND event.EndTime > startTime.
func DetectConflicts(events []CalendarEvent, startTime, endTime time.Time) ConflictResult {
	var conflicts []CalendarEvent
	for _, ev := range events {
		if ev.Status == "cancelled" {
			continue
		}
		if ev.StartTime.Before(endTime) && ev.EndTime.After(startTime) {
			conflicts = append(conflicts, ev)
		}
	}
	return ConflictResult{
		HasConflict: len(conflicts) > 0,
		Conflicts:   conflicts,
	}
}

// FindFreeSlots scans the range [rangeStart, rangeEnd) and returns every gap
// between non-cancelled events that is at least `duration` long.
//
// Events are sorted by start time before scanning. Overlapping or adjacent
// busy blocks are merged so that only true gaps are reported.
func FindFreeSlots(events []CalendarEvent, rangeStart, rangeEnd time.Time, duration time.Duration) []TimeSlot {
	// Filter out cancelled events and collect only those that overlap with
	// the requested range.
	var active []CalendarEvent
	for _, ev := range events {
		if ev.Status == "cancelled" {
			continue
		}
		// Keep events that overlap with [rangeStart, rangeEnd).
		if ev.StartTime.Before(rangeEnd) && ev.EndTime.After(rangeStart) {
			active = append(active, ev)
		}
	}

	// Sort by start time (ties broken by end time descending for correct merging).
	sort.Slice(active, func(i, j int) bool {
		if active[i].StartTime.Equal(active[j].StartTime) {
			return active[j].EndTime.Before(active[i].EndTime)
		}
		return active[i].StartTime.Before(active[j].StartTime)
	})

	// Merge overlapping/adjacent busy intervals.
	type interval struct{ start, end time.Time }
	var merged []interval
	for _, ev := range active {
		s := ev.StartTime
		e := ev.EndTime
		if len(merged) > 0 && !s.After(merged[len(merged)-1].end) {
			// Overlapping or adjacent — extend.
			if e.After(merged[len(merged)-1].end) {
				merged[len(merged)-1].end = e
			}
		} else {
			merged = append(merged, interval{start: s, end: e})
		}
	}

	// Walk merged busy blocks and collect gaps that meet the minimum duration.
	var slots []TimeSlot
	cursor := rangeStart
	for _, blk := range merged {
		gapStart := cursor
		gapEnd := blk.start
		// Clamp to range.
		if gapStart.Before(rangeStart) {
			gapStart = rangeStart
		}
		if gapEnd.After(rangeEnd) {
			gapEnd = rangeEnd
		}
		if gapEnd.Sub(gapStart) >= duration {
			slots = append(slots, TimeSlot{Start: gapStart, End: gapEnd})
		}
		// Advance cursor past this busy block.
		if blk.end.After(cursor) {
			cursor = blk.end
		}
	}

	// Trailing gap after the last busy block.
	if cursor.Before(rangeEnd) {
		gapStart := cursor
		gapEnd := rangeEnd
		if gapEnd.Sub(gapStart) >= duration {
			slots = append(slots, TimeSlot{Start: gapStart, End: gapEnd})
		}
	}

	return slots
}
