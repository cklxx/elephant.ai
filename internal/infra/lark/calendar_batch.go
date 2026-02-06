package lark

import (
	"context"
	"fmt"
)

// BatchCreateEvents creates multiple calendar events sequentially, collecting per-event errors.
// On success the corresponding error entry is nil; on failure the event entry is zero-valued.
func (s *CalendarService) BatchCreateEvents(ctx context.Context, calendarID string, events []CreateEventRequest, opts ...CallOption) ([]CalendarEvent, []error) {
	results := make([]CalendarEvent, len(events))
	errs := make([]error, len(events))
	if len(events) == 0 {
		return results, errs
	}

	resolvedID, err := s.ResolveCalendarID(ctx, calendarID, opts...)
	if err != nil {
		for i := range errs {
			errs[i] = fmt.Errorf("resolve calendar id: %w", err)
		}
		return results, errs
	}

	for i, req := range events {
		// Override calendarID so the caller controls the target calendar.
		req.CalendarID = resolvedID
		ev, err := s.CreateEvent(ctx, req, opts...)
		if err != nil {
			errs[i] = fmt.Errorf("event[%d] %q: %w", i, req.Summary, err)
			continue
		}
		results[i] = *ev
	}
	return results, errs
}

// BatchDeleteEvents deletes multiple calendar events sequentially, collecting per-event errors.
// Each entry in the returned slice corresponds to the eventID at the same index.
func (s *CalendarService) BatchDeleteEvents(ctx context.Context, calendarID string, eventIDs []string, opts ...CallOption) []error {
	errs := make([]error, len(eventIDs))
	if len(eventIDs) == 0 {
		return errs
	}

	resolvedID, err := s.ResolveCalendarID(ctx, calendarID, opts...)
	if err != nil {
		for i := range errs {
			errs[i] = fmt.Errorf("resolve calendar id: %w", err)
		}
		return errs
	}
	for i, id := range eventIDs {
		if err := s.DeleteEvent(ctx, resolvedID, id, opts...); err != nil {
			errs[i] = fmt.Errorf("event[%d] %q: %w", i, id, err)
		}
	}
	return errs
}
