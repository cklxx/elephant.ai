package larktools

import (
	"context"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

type larkCalendarDelete struct {
	shared.BaseTool
}

// NewLarkCalendarDelete constructs a tool for deleting a calendar event.
func NewLarkCalendarDelete() tools.ToolExecutor {
	return &larkCalendarDelete{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_calendar_delete",
				Description: "Delete a calendar event from the caller's primary calendar. Requires approval.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"event_id": {
							Type:        "string",
							Description: "The ID of the event to delete.",
						},
					},
					Required: []string{"event_id"},
				},
			},
			ports.ToolMetadata{
				Name:        "lark_calendar_delete",
				Version:     "0.1.0",
				Category:    "lark",
				Tags:        []string{"lark", "calendar", "delete"},
				Dangerous:   true,
				SafetyLevel: ports.SafetyLevelIrreversible,
			},
		),
	}
}

func (t *larkCalendarDelete) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	client, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	eventID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "event_id")
	if errResult != nil {
		return errResult, nil
	}

	auth, errResult := resolveLarkCalendarAuth(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}
	callOpt, reqOpt := buildLarkAuthOptions(auth)
	calendarID, errResult := resolveCalendarID(ctx, call.ID, client, auth, callOpt, "lark_calendar_delete")
	if errResult != nil {
		return errResult, nil
	}

	builder := larkcalendar.NewDeleteCalendarEventReqBuilder().
		CalendarId(calendarID).
		EventId(eventID)

	options := []larkcore.RequestOptionFunc{reqOpt}
	resp, err := client.Calendar.CalendarEvent.Delete(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_calendar_delete", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_calendar_delete", resp.Code, resp.Msg), nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Calendar event deleted successfully.",
		Metadata: map[string]any{
			"calendar_id": calendarID,
			"event_id":    eventID,
		},
	}, nil
}
