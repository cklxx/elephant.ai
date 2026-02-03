package larktools

import (
	"context"
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	larkapi "alex/internal/lark"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
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
				Name:      "lark_calendar_delete",
				Version:   "0.1.0",
				Category:  "lark",
				Tags:      []string{"lark", "calendar", "delete"},
				Dangerous: true,
			},
		),
	}
}

func (t *larkCalendarDelete) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_delete is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_delete: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	eventID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "event_id")
	if errResult != nil {
		return errResult, nil
	}

	userToken, errResult := requireLarkUserAccessToken(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}
	calendarID, err := larkapi.Wrap(client).Calendar().ResolveCalendarID(ctx, "primary", larkapi.WithUserToken(userToken))
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_delete: failed to resolve primary calendar_id: %v", err),
			Error:   fmt.Errorf("resolve primary calendar id: %w", err),
		}, nil
	}

	builder := larkcalendar.NewDeleteCalendarEventReqBuilder().
		CalendarId(calendarID).
		EventId(eventID)

	options := []larkcore.RequestOptionFunc{larkcore.WithUserAccessToken(userToken)}
	resp, err := client.Calendar.CalendarEvent.Delete(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_delete: API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_delete: API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
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
