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

	for i, req := range events {
		// Override calendarID so the caller controls the target calendar.
		req.CalendarID = calendarID
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
	for i, id := range eventIDs {
		if err := s.DeleteEvent(ctx, calendarID, id, opts...); err != nil {
			errs[i] = fmt.Errorf("event[%d] %q: %w", i, id, err)
		}
	}
	return errs
}
