package lark

import (
	"context"
	"fmt"

	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

// CalendarInfo describes a single Lark calendar.
type CalendarInfo struct {
	ID          string
	Summary     string
	Description string
	Type        string // e.g. "primary", "shared", "google", "resource", "exchange"
	TimeZone    string // IANA timezone; may be empty for some calendar types
	Role        string // access role for the current identity
}

// ListCalendarsResponse contains paginated calendar list results.
type ListCalendarsResponse struct {
	Calendars []CalendarInfo
	PageToken string
	HasMore   bool
}

// ListCalendars returns all accessible calendars for the current identity.
// It paginates automatically and returns the full list.
func (s *CalendarService) ListCalendars(ctx context.Context, opts ...CallOption) ([]CalendarInfo, error) {
	var all []CalendarInfo
	var pageToken string

	for {
		builder := larkcalendar.NewListCalendarReqBuilder().
			PageSize(50)
		if pageToken != "" {
			builder.PageToken(pageToken)
		}

		resp, err := s.client.Calendar.Calendar.List(ctx, builder.Build(), buildOpts(opts)...)
		if err != nil {
			return nil, fmt.Errorf("list calendars: %w", err)
		}
		if !resp.Success() {
			return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
		}

		for _, cal := range resp.Data.CalendarList {
			all = append(all, parseCalendarInfo(cal))
		}

		if resp.Data.HasMore != nil && *resp.Data.HasMore && resp.Data.PageToken != nil {
			pageToken = *resp.Data.PageToken
		} else {
			break
		}
	}

	return all, nil
}

// ListCalendarsPage returns a single page of calendars. Use the returned
// page token to fetch subsequent pages.
func (s *CalendarService) ListCalendarsPage(ctx context.Context, pageToken string, pageSize int, opts ...CallOption) (*ListCalendarsResponse, error) {
	if pageSize <= 0 {
		pageSize = 50
	}

	builder := larkcalendar.NewListCalendarReqBuilder().
		PageSize(pageSize)
	if pageToken != "" {
		builder.PageToken(pageToken)
	}

	resp, err := s.client.Calendar.Calendar.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list calendars page: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	calendars := make([]CalendarInfo, 0, len(resp.Data.CalendarList))
	for _, cal := range resp.Data.CalendarList {
		calendars = append(calendars, parseCalendarInfo(cal))
	}

	var pt string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pt = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListCalendarsResponse{
		Calendars: calendars,
		PageToken: pt,
		HasMore:   hasMore,
	}, nil
}

// --- helpers ---

func parseCalendarInfo(cal *larkcalendar.Calendar) CalendarInfo {
	if cal == nil {
		return CalendarInfo{}
	}
	info := CalendarInfo{}
	if cal.CalendarId != nil {
		info.ID = *cal.CalendarId
	}
	if cal.Summary != nil {
		info.Summary = *cal.Summary
	}
	if cal.Description != nil {
		info.Description = *cal.Description
	}
	if cal.Type != nil {
		info.Type = *cal.Type
	}
	if cal.Role != nil {
		info.Role = *cal.Role
	}
	return info
}
