package lark

import (
	"context"
	"fmt"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

// CalendarService provides typed access to Lark Calendar APIs.
type CalendarService struct {
	client *lark.Client
}

// CalendarEvent is a simplified view of a Lark calendar event.
type CalendarEvent struct {
	EventID     string
	Summary     string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	Location    string
	Status      string
}

// ListEventsRequest defines parameters for listing calendar events.
type ListEventsRequest struct {
	CalendarID string // default "primary"
	StartTime  time.Time
	EndTime    time.Time
	PageSize   int
	PageToken  string
}

// ListEventsResponse contains paginated calendar events.
type ListEventsResponse struct {
	Events    []CalendarEvent
	PageToken string
	HasMore   bool
}

// ListEvents returns calendar events within a time range.
func (s *CalendarService) ListEvents(ctx context.Context, req ListEventsRequest, opts ...CallOption) (*ListEventsResponse, error) {
	calID := req.CalendarID
	if calID == "" {
		calID = "primary"
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	builder := larkcalendar.NewListCalendarEventReqBuilder().
		CalendarId(calID).
		StartTime(fmt.Sprintf("%d", req.StartTime.Unix())).
		EndTime(fmt.Sprintf("%d", req.EndTime.Unix())).
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Calendar.CalendarEvent.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list calendar events: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	events := make([]CalendarEvent, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		events = append(events, parseCalendarEvent(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListEventsResponse{
		Events:    events,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// CreateEventRequest defines parameters for creating a calendar event.
type CreateEventRequest struct {
	CalendarID  string
	Summary     string
	Description string
	StartTime   time.Time
	EndTime     time.Time
}

// CreateEvent creates a new calendar event.
func (s *CalendarService) CreateEvent(ctx context.Context, req CreateEventRequest, opts ...CallOption) (*CalendarEvent, error) {
	calID := req.CalendarID
	if calID == "" {
		calID = "primary"
	}

	eventBuilder := larkcalendar.NewCalendarEventBuilder().
		Summary(req.Summary).
		Description(req.Description).
		StartTime(larkcalendar.NewTimeInfoBuilder().
			Timestamp(fmt.Sprintf("%d", req.StartTime.Unix())).
			Build()).
		EndTime(larkcalendar.NewTimeInfoBuilder().
			Timestamp(fmt.Sprintf("%d", req.EndTime.Unix())).
			Build())

	createReq := larkcalendar.NewCreateCalendarEventReqBuilder().
		CalendarId(calID).
		CalendarEvent(eventBuilder.Build()).
		Build()

	resp, err := s.client.Calendar.CalendarEvent.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create calendar event: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	ev := parseCalendarEvent(resp.Data.Event)
	return &ev, nil
}

// PatchEventRequest defines parameters for updating a calendar event.
type PatchEventRequest struct {
	CalendarID  string
	EventID     string
	Summary     *string
	Description *string
	StartTime   *time.Time
	EndTime     *time.Time
}

// PatchEvent updates an existing calendar event. Only non-nil fields are patched.
func (s *CalendarService) PatchEvent(ctx context.Context, req PatchEventRequest, opts ...CallOption) (*CalendarEvent, error) {
	calID := req.CalendarID
	if calID == "" {
		calID = "primary"
	}

	eventBuilder := larkcalendar.NewCalendarEventBuilder()
	if req.Summary != nil {
		eventBuilder.Summary(*req.Summary)
	}
	if req.Description != nil {
		eventBuilder.Description(*req.Description)
	}
	if req.StartTime != nil {
		eventBuilder.StartTime(larkcalendar.NewTimeInfoBuilder().
			Timestamp(fmt.Sprintf("%d", req.StartTime.Unix())).
			Build())
	}
	if req.EndTime != nil {
		eventBuilder.EndTime(larkcalendar.NewTimeInfoBuilder().
			Timestamp(fmt.Sprintf("%d", req.EndTime.Unix())).
			Build())
	}

	patchReq := larkcalendar.NewPatchCalendarEventReqBuilder().
		CalendarId(calID).
		EventId(req.EventID).
		CalendarEvent(eventBuilder.Build()).
		Build()

	resp, err := s.client.Calendar.CalendarEvent.Patch(ctx, patchReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("patch calendar event: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	ev := parseCalendarEvent(resp.Data.Event)
	return &ev, nil
}

// DeleteEvent deletes a calendar event.
func (s *CalendarService) DeleteEvent(ctx context.Context, calendarID, eventID string, opts ...CallOption) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	req := larkcalendar.NewDeleteCalendarEventReqBuilder().
		CalendarId(calendarID).
		EventId(eventID).
		Build()

	resp, err := s.client.Calendar.CalendarEvent.Delete(ctx, req, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// --- helpers ---

func parseCalendarEvent(item *larkcalendar.CalendarEvent) CalendarEvent {
	if item == nil {
		return CalendarEvent{}
	}
	ev := CalendarEvent{}
	if item.EventId != nil {
		ev.EventID = *item.EventId
	}
	if item.Summary != nil {
		ev.Summary = *item.Summary
	}
	if item.Description != nil {
		ev.Description = *item.Description
	}
	if item.Status != nil {
		ev.Status = *item.Status
	}
	if item.Location != nil && item.Location.Name != nil {
		ev.Location = *item.Location.Name
	}
	if item.StartTime != nil && item.StartTime.Timestamp != nil {
		ev.StartTime = parseTimestamp(*item.StartTime.Timestamp)
	}
	if item.EndTime != nil && item.EndTime.Timestamp != nil {
		ev.EndTime = parseTimestamp(*item.EndTime.Timestamp)
	}
	return ev
}

func parseTimestamp(ts string) time.Time {
	var sec int64
	fmt.Sscanf(ts, "%d", &sec)
	return time.Unix(sec, 0)
}
