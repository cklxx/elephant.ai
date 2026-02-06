package lark

import (
	"context"
	"fmt"
	"strings"
)

// ResolveCalendarID resolves well-known aliases (currently "primary") to a real
// Lark calendar_id (e.g. "cal_xxx").
//
// Lark Calendar v4 APIs require a concrete calendar_id; passing "primary" as the
// calendar_id is invalid. This helper keeps higher-level code ergonomic by
// allowing "primary" while still calling the APIs with a valid calendar_id.
func (s *CalendarService) ResolveCalendarID(ctx context.Context, calendarID string, opts ...CallOption) (string, error) {
	trimmed := strings.TrimSpace(calendarID)
	if trimmed == "" {
		trimmed = "primary"
	}
	if !strings.EqualFold(trimmed, "primary") {
		return trimmed, nil
	}

	calendars, err := s.ListCalendars(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("resolve primary calendar id: %w", err)
	}
	if len(calendars) == 0 {
		return "", fmt.Errorf("resolve primary calendar id: no calendars returned")
	}

	for _, cal := range calendars {
		if cal.ID == "" {
			continue
		}
		if strings.EqualFold(cal.Type, "primary") {
			return cal.ID, nil
		}
	}
	for _, cal := range calendars {
		if cal.ID == "" {
			continue
		}
		if strings.EqualFold(cal.Role, "owner") {
			return cal.ID, nil
		}
	}
	for _, cal := range calendars {
		if cal.ID != "" {
			return cal.ID, nil
		}
	}
	return "", fmt.Errorf("resolve primary calendar id: calendars missing IDs")
}
