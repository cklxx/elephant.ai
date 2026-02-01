package larktools

import (
	"context"
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

type larkCalendarUpdate struct {
	shared.BaseTool
}

// NewLarkCalendarUpdate constructs a tool for updating an existing calendar event.
func NewLarkCalendarUpdate() tools.ToolExecutor {
	return &larkCalendarUpdate{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_calendar_update",
				Description: "Update an existing calendar event",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"event_id": {
							Type:        "string",
							Description: "The ID of the event to update.",
						},
						"calendar_id": {
							Type:        "string",
							Description: "Calendar ID (defaults to \"primary\").",
						},
						"summary": {
							Type:        "string",
							Description: "New event title.",
						},
						"description": {
							Type:        "string",
							Description: "New event description.",
						},
						"start_time": {
							Type:        "string",
							Description: "New start time as Unix timestamp in seconds.",
						},
						"end_time": {
							Type:        "string",
							Description: "New end time as Unix timestamp in seconds.",
						},
						"user_access_token": {
							Type:        "string",
							Description: "Optional user access token for user-scoped calendar update.",
						},
					},
					Required: []string{"event_id"},
				},
			},
			ports.ToolMetadata{
				Name:      "lark_calendar_update",
				Version:   "0.1.0",
				Category:  "lark",
				Tags:      []string{"lark", "calendar", "update"},
				Dangerous: true,
			},
		),
	}
}

func (t *larkCalendarUpdate) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_update is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_update: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	eventID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "event_id")
	if errResult != nil {
		return errResult, nil
	}

	calendarID := shared.StringArg(call.Arguments, "calendar_id")
	if calendarID == "" {
		calendarID = "primary"
	}

	summary := shared.StringArg(call.Arguments, "summary")
	description := shared.StringArg(call.Arguments, "description")
	startTimeStr := shared.StringArg(call.Arguments, "start_time")
	endTimeStr := shared.StringArg(call.Arguments, "end_time")

	// Build a CalendarEvent with only the fields that are provided.
	event := &larkcalendar.CalendarEvent{}
	hasFields := false

	if summary != "" {
		event.Summary = &summary
		hasFields = true
	}
	if description != "" {
		event.Description = &description
		hasFields = true
	}
	if startTimeStr != "" {
		startTime, _, errResult := parseUnixSecondsValue(call.ID, "start_time", startTimeStr)
		if errResult != nil {
			return errResult, nil
		}
		event.StartTime = &larkcalendar.TimeInfo{Timestamp: &startTime}
		hasFields = true
	}
	if endTimeStr != "" {
		endTime, _, errResult := parseUnixSecondsValue(call.ID, "end_time", endTimeStr)
		if errResult != nil {
			return errResult, nil
		}
		event.EndTime = &larkcalendar.TimeInfo{Timestamp: &endTime}
		hasFields = true
	}

	if !hasFields {
		err := fmt.Errorf("at least one field to update must be provided (summary, description, start_time, end_time)")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	builder := larkcalendar.NewPatchCalendarEventReqBuilder().
		CalendarId(calendarID).
		EventId(eventID).
		CalendarEvent(event)

	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}

	options := calendarRequestOptions(call.Arguments)
	resp, err := client.Calendar.CalendarEvent.Patch(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_update: API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_update: API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	updatedSummary := ""
	if resp.Data != nil && resp.Data.Event != nil && resp.Data.Event.Summary != nil {
		updatedSummary = *resp.Data.Event.Summary
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Calendar event updated successfully. Summary: %s", updatedSummary),
		Metadata: map[string]any{
			"calendar_id": calendarID,
			"event_id":    eventID,
		},
	}, nil
}
