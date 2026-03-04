package larktools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

type larkCalendarCreate struct {
	shared.BaseTool
}

// NewLarkCalendarCreate constructs a tool for creating calendar events.
func NewLarkCalendarCreate() tools.ToolExecutor {
	return &larkCalendarCreate{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_calendar_create",
				Description: "Create a calendar event in the caller's primary calendar by summary, start_time, and end_time (Unix seconds). Requires approval.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"summary": {
							Type:        "string",
							Description: "Event title.",
						},
						"start_time": {
							Type:        "string",
							Description: "Start time as Unix timestamp in seconds.",
						},
						"end_time": {
							Type:        "string",
							Description: "End time as Unix timestamp in seconds.",
						},
						"description": {
							Type:        "string",
							Description: "Optional event description.",
						},
						"timezone": {
							Type:        "string",
							Description: "Optional IANA timezone (defaults to Lark settings).",
						},
						"need_notification": {
							Type:        "boolean",
							Description: "Whether to notify attendees (default true).",
						},
						"idempotency_key": {
							Type:        "string",
							Description: "Optional idempotency key for safe retries.",
						},
					},
					Required: []string{"summary", "start_time", "end_time"},
				},
			},
			ports.ToolMetadata{
				Name:        "lark_calendar_create",
				Version:     "0.1.0",
				Category:    "lark",
				Tags:        []string{"lark", "calendar", "create"},
				Dangerous:   true,
				SafetyLevel: ports.SafetyLevelHighImpact,
			},
		),
	}
}

func (t *larkCalendarCreate) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	client, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	summary, errResult := shared.RequireStringArg(call.Arguments, call.ID, "summary")
	if errResult != nil {
		return errResult, nil
	}
	startTime, startUnix, errResult := requireUnixSeconds(call.Arguments, call.ID, "start_time")
	if errResult != nil {
		return errResult, nil
	}
	endTime, endUnix, errResult := requireUnixSeconds(call.Arguments, call.ID, "end_time")
	if errResult != nil {
		return errResult, nil
	}
	if endUnix <= startUnix {
		err := fmt.Errorf("end_time must be greater than start_time")
		return shared.ToolError(call.ID, "%v", err)
	}

	description := shared.StringArg(call.Arguments, "description")
	timezone := shared.StringArg(call.Arguments, "timezone")
	needNotification, hasNeedNotification := boolArg(call.Arguments, "need_notification")

	auth, errResult := resolveLarkCalendarAuth(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}
	callOpt, reqOpt := buildLarkAuthOptions(auth)
	calendarID, errResult := resolveCalendarID(ctx, call.ID, client, auth, callOpt, "lark_calendar_create")
	if errResult != nil {
		return errResult, nil
	}

	startInfo := &larkcalendar.TimeInfo{Timestamp: &startTime}
	endInfo := &larkcalendar.TimeInfo{Timestamp: &endTime}
	if timezone != "" {
		startInfo.Timezone = &timezone
		endInfo.Timezone = &timezone
	}

	event := &larkcalendar.CalendarEvent{
		Summary:   &summary,
		StartTime: startInfo,
		EndTime:   endInfo,
	}
	if description != "" {
		event.Description = &description
	}
	if hasNeedNotification {
		event.NeedNotification = &needNotification
	}
	autoAddedSender := ensureTenantCalendarVisibilityAttendee(ctx, event, auth)

	builder := larkcalendar.NewCreateCalendarEventReqBuilder().
		CalendarId(calendarID).
		CalendarEvent(event)
	if autoAddedSender {
		builder.UserIdType(larkcalendar.UserIdTypeCreateCalendarEventOpenId)
	}

	if idempotencyKey := shared.StringArg(call.Arguments, "idempotency_key"); idempotencyKey != "" {
		builder.IdempotencyKey(idempotencyKey)
	}

	options := []larkcore.RequestOptionFunc{reqOpt}
	resp, err := client.Calendar.CalendarEvent.Create(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_calendar_create", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_calendar_create", resp.Code, resp.Msg), nil
	}

	eventID := ""
	if resp.Data != nil && resp.Data.Event != nil && resp.Data.Event.EventId != nil {
		eventID = *resp.Data.Event.EventId
	}

	metadata := map[string]any{
		"calendar_id": calendarID,
		"event_id":    eventID,
		"start_time":  startTime,
		"end_time":    endTime,
		"auth_mode":   auth.mode(),
	}
	if autoAddedSender {
		metadata["sender_added_as_attendee"] = true
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "Calendar event created successfully.",
		Metadata: metadata,
	}, nil
}

func ensureTenantCalendarVisibilityAttendee(ctx context.Context, event *larkcalendar.CalendarEvent, auth larkAccessToken) bool {
	if auth.kind != larkTokenTenant || event == nil {
		return false
	}

	senderID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if senderID == "" {
		return false
	}
	for _, attendee := range event.Attendees {
		if attendee == nil || attendee.AttendeeId == nil {
			continue
		}
		if strings.TrimSpace(*attendee.AttendeeId) != senderID {
			continue
		}
		attendeeType := "user"
		if attendee.Type != nil && strings.TrimSpace(*attendee.Type) != "" {
			attendeeType = strings.TrimSpace(*attendee.Type)
		}
		if strings.EqualFold(attendeeType, "user") {
			return false
		}
	}

	attendeeType := "user"
	attendeeID := senderID
	event.Attendees = append(event.Attendees, &larkcalendar.CalendarEventAttendee{
		Type:       &attendeeType,
		AttendeeId: &attendeeID,
	})
	return true
}

func boolArg(args map[string]any, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	value, ok := args[key]
	if !ok {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed, true
		}
	case float64:
		return v != 0, true
	case int:
		return v != 0, true
	}
	return false, false
}
