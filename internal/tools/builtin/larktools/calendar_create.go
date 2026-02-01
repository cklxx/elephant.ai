package larktools

import (
	"context"
	"fmt"
	"strconv"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
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
				Description: "Create a calendar event by calendar_id, summary, start_time, and end_time (Unix seconds). Requires approval.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"calendar_id": {
							Type:        "string",
							Description: "Calendar ID to create the event in.",
						},
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
						"user_id_type": {
							Type:        "string",
							Description: "User ID type (open_id, user_id, union_id).",
						},
						"user_access_token": {
							Type:        "string",
							Description: "Optional user access token for user-scoped calendar creation.",
						},
						"idempotency_key": {
							Type:        "string",
							Description: "Optional idempotency key for safe retries.",
						},
					},
					Required: []string{"calendar_id", "summary", "start_time", "end_time"},
				},
			},
			ports.ToolMetadata{
				Name:      "lark_calendar_create",
				Version:   "0.1.0",
				Category:  "lark",
				Tags:      []string{"lark", "calendar", "create"},
				Dangerous: true,
			},
		),
	}
}

func (t *larkCalendarCreate) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_create is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_create: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	calendarID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "calendar_id")
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
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	description := shared.StringArg(call.Arguments, "description")
	timezone := shared.StringArg(call.Arguments, "timezone")
	needNotification, hasNeedNotification := boolArg(call.Arguments, "need_notification")

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

	builder := larkcalendar.NewCreateCalendarEventReqBuilder().
		CalendarId(calendarID).
		CalendarEvent(event)

	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}
	if idempotencyKey := shared.StringArg(call.Arguments, "idempotency_key"); idempotencyKey != "" {
		builder.IdempotencyKey(idempotencyKey)
	}

	options := calendarRequestOptions(call.Arguments)
	resp, err := client.Calendar.CalendarEvent.Create(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_create: API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_create: API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	eventID := ""
	if resp.Data != nil && resp.Data.Event != nil && resp.Data.Event.EventId != nil {
		eventID = *resp.Data.Event.EventId
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Calendar event created successfully.",
		Metadata: map[string]any{
			"calendar_id": calendarID,
			"event_id":    eventID,
			"start_time":  startTime,
			"end_time":    endTime,
		},
	}, nil
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
