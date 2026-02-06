package lark

import (
	"context"
	"fmt"
	"strings"

	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

// PrimaryCalendarIDs returns a mapping of user ID -> primary calendar_id for the
// provided users.
//
// userIDType must be one of: open_id, user_id, union_id. If empty, it defaults
// to open_id.
func (s *CalendarService) PrimaryCalendarIDs(ctx context.Context, userIDType string, userIDs []string, opts ...CallOption) (map[string]string, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("primary calendars: nil client")
	}

	t := strings.TrimSpace(userIDType)
	if t == "" {
		t = "open_id"
	}

	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("primary calendars: empty user_ids")
	}

	body := larkcalendar.NewPrimarysCalendarReqBodyBuilder().
		UserIds(ids).
		Build()

	builder := larkcalendar.NewPrimarysCalendarReqBuilder().
		UserIdType(t).
		Body(body)

	resp, err := s.client.Calendar.Calendar.Primarys(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("primary calendars: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("primary calendars: empty response data")
	}

	out := make(map[string]string, len(ids))
	for _, uc := range resp.Data.Calendars {
		if uc == nil || uc.UserId == nil || strings.TrimSpace(*uc.UserId) == "" {
			continue
		}
		userID := strings.TrimSpace(*uc.UserId)
		if uc.Calendar == nil || uc.Calendar.CalendarId == nil || strings.TrimSpace(*uc.Calendar.CalendarId) == "" {
			continue
		}
		if _, exists := out[userID]; exists {
			continue
		}
		out[userID] = strings.TrimSpace(*uc.Calendar.CalendarId)
	}
	return out, nil
}

// PrimaryCalendarID returns a single user's primary calendar_id.
func (s *CalendarService) PrimaryCalendarID(ctx context.Context, userIDType, userID string, opts ...CallOption) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("primary calendar id: empty user_id")
	}
	m, err := s.PrimaryCalendarIDs(ctx, userIDType, []string{userID}, opts...)
	if err != nil {
		return "", err
	}
	if calID := strings.TrimSpace(m[userID]); calID != "" {
		return calID, nil
	}
	return "", fmt.Errorf("primary calendar id: not found (user_id_type=%s user_id=%s)", strings.TrimSpace(userIDType), userID)
}
