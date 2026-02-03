package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcalendar "github.com/larksuite/oapi-sdk-go/v3/service/calendar/v4"
)

const (
	calendarDefaultPageSize = 50
	calendarMaxPageSize     = 1000
)

type larkCalendarQuery struct {
	shared.BaseTool
}

// NewLarkCalendarQuery constructs a tool for querying calendar events.
func NewLarkCalendarQuery() tools.ToolExecutor {
	return &larkCalendarQuery{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_calendar_query",
				Description: "Query calendar events by calendar_id and time range. Requires calendar_id, start_time, and end_time (Unix seconds).",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"calendar_id": {
							Type:        "string",
							Description: "Calendar ID to query. Use \"primary\" to auto-resolve a user's primary calendar ID (see calendar_owner_id).",
						},
						"calendar_owner_id": {
							Type:        "string",
							Description: "Optional calendar owner user ID. When calendar_id is \"primary\", resolve this user's primary calendar_id (e.g. open_id from @mention).",
						},
						"calendar_owner_id_type": {
							Type:        "string",
							Description: "Type of calendar_owner_id (open_id, user_id, union_id). Default open_id.",
						},
						"start_time": {
							Type:        "string",
							Description: "Start time as Unix timestamp in seconds.",
						},
						"end_time": {
							Type:        "string",
							Description: "End time as Unix timestamp in seconds.",
						},
						"page_size": {
							Type:        "integer",
							Description: "Page size (default 50, max 1000).",
						},
						"page_token": {
							Type:        "string",
							Description: "Pagination token from previous response.",
						},
						"user_id_type": {
							Type:        "string",
							Description: "User ID type (open_id, user_id, union_id).",
						},
						"user_access_token": {
							Type:        "string",
							Description: "Optional user access token for user-scoped calendar queries.",
						},
					},
					Required: []string{"calendar_id", "start_time", "end_time"},
				},
			},
			ports.ToolMetadata{
				Name:     "lark_calendar_query",
				Version:  "0.1.0",
				Category: "lark",
				Tags:     []string{"lark", "calendar", "query"},
			},
		),
	}
}

func (t *larkCalendarQuery) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_query is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_calendar_query: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	calendarID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "calendar_id")
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
	if endUnix < startUnix {
		err := fmt.Errorf("end_time must be >= start_time")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolvedID, errResult := resolveCalendarID(ctx, client, call.ID, calendarID, call.Arguments)
	if errResult != nil {
		return errResult, nil
	}
	calendarID = resolvedID

	builder := larkcalendar.NewListCalendarEventReqBuilder().
		CalendarId(calendarID).
		StartTime(startTime).
		EndTime(endTime)

	pageSize := clampCalendarPageSize(call.Arguments)
	if pageSize > 0 {
		builder.PageSize(pageSize)
	}
	if pageToken := shared.StringArg(call.Arguments, "page_token"); pageToken != "" {
		builder.PageToken(pageToken)
	}
	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}

	options := calendarRequestOptions(ctx, call.Arguments)
	resp, err := client.Calendar.CalendarEvent.List(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_query: API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_calendar_query: API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No calendar events found for the given range.",
			Metadata: map[string]any{
				"calendar_id": calendarID,
			},
		}, nil
	}

	summaries := summarizeCalendarEvents(resp.Data.Items)
	payload, _ := json.MarshalIndent(summaries, "", "  ")
	content := fmt.Sprintf("Found %d events:\n%s", len(summaries), string(payload))

	metadata := map[string]any{
		"calendar_id": calendarID,
		"event_count": len(summaries),
	}
	if resp.Data.HasMore != nil {
		metadata["has_more"] = *resp.Data.HasMore
	}
	if resp.Data.PageToken != nil {
		metadata["page_token"] = *resp.Data.PageToken
	}
	if resp.Data.SyncToken != nil {
		metadata["sync_token"] = *resp.Data.SyncToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

type calendarEventSummary struct {
	EventID           string `json:"event_id"`
	Summary           string `json:"summary,omitempty"`
	StartTime         string `json:"start_time,omitempty"`
	EndTime           string `json:"end_time,omitempty"`
	OrganizerCalendar string `json:"organizer_calendar_id,omitempty"`
	Status            string `json:"status,omitempty"`
	HasAttendees      bool   `json:"has_attendees,omitempty"`
	HasMoreAttendees  bool   `json:"has_more_attendee,omitempty"`
	NeedNotification  *bool  `json:"need_notification,omitempty"`
	FreeBusyStatus    string `json:"free_busy_status,omitempty"`
	Visibility        string `json:"visibility,omitempty"`
}

func summarizeCalendarEvents(items []*larkcalendar.CalendarEvent) []calendarEventSummary {
	summaries := make([]calendarEventSummary, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		summary := calendarEventSummary{}
		if item.EventId != nil {
			summary.EventID = *item.EventId
		}
		if item.Summary != nil {
			summary.Summary = *item.Summary
		}
		summary.StartTime = formatTimeInfo(item.StartTime)
		summary.EndTime = formatTimeInfo(item.EndTime)
		if item.OrganizerCalendarId != nil {
			summary.OrganizerCalendar = *item.OrganizerCalendarId
		}
		if item.Status != nil {
			summary.Status = *item.Status
		}
		if item.FreeBusyStatus != nil {
			summary.FreeBusyStatus = *item.FreeBusyStatus
		}
		if item.Visibility != nil {
			summary.Visibility = *item.Visibility
		}
		if item.NeedNotification != nil {
			summary.NeedNotification = item.NeedNotification
		}
		summary.HasAttendees = len(item.Attendees) > 0
		if item.HasMoreAttendee != nil {
			summary.HasMoreAttendees = *item.HasMoreAttendee
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func formatTimeInfo(info *larkcalendar.TimeInfo) string {
	if info == nil {
		return ""
	}
	if info.Timestamp != nil && *info.Timestamp != "" {
		return *info.Timestamp
	}
	if info.Date != nil {
		return *info.Date
	}
	return ""
}

func clampCalendarPageSize(args map[string]any) int {
	pageSize := calendarDefaultPageSize
	if raw, ok := args["page_size"]; ok {
		switch v := raw.(type) {
		case float64:
			pageSize = int(v)
		case int:
			pageSize = v
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				pageSize = parsed
			}
		}
	}
	if pageSize <= 0 {
		return calendarDefaultPageSize
	}
	if pageSize > calendarMaxPageSize {
		return calendarMaxPageSize
	}
	return pageSize
}

func requireUnixSeconds(args map[string]any, callID, key string) (string, int64, *ports.ToolResult) {
	if args == nil {
		err := fmt.Errorf("missing '%s'", key)
		return "", 0, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	raw, ok := args[key]
	if !ok {
		err := fmt.Errorf("missing '%s'", key)
		return "", 0, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	return parseUnixSecondsValue(callID, key, raw)
}

func parseUnixSecondsValue(callID, key string, raw any) (string, int64, *ports.ToolResult) {
	var value string
	var parsed int64
	switch v := raw.(type) {
	case string:
		value = v
		if value == "" {
			break
		}
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			err = fmt.Errorf("%s must be a unix seconds timestamp", key)
			return "", 0, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
		}
		parsed = val
	case float64:
		parsed = int64(v)
		value = fmt.Sprintf("%d", parsed)
	case int:
		parsed = int64(v)
		value = fmt.Sprintf("%d", parsed)
	case int64:
		parsed = v
		value = fmt.Sprintf("%d", parsed)
	default:
		err := fmt.Errorf("%s must be a unix seconds timestamp", key)
		return "", 0, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if value == "" {
		err := fmt.Errorf("%s cannot be empty", key)
		return "", 0, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	return value, parsed, nil
}

func calendarRequestOptions(ctx context.Context, args map[string]any) []larkcore.RequestOptionFunc {
	token := shared.StringArg(args, "user_access_token")
	if token == "" {
		return nil
	}

	// If the tool targets someone else's calendar (calendar_owner_id), do not
	// blindly apply the caller's user token: it likely belongs to the initiator
	// and may not have permission to operate on the owner's calendar. In that
	// case prefer tenant-scoped calls.
	ownerID := strings.TrimSpace(shared.StringArg(args, "calendar_owner_id"))
	if ownerID != "" {
		ctxUserID := strings.TrimSpace(id.UserIDFromContext(ctx))
		if ctxUserID == "" || ownerID != ctxUserID {
			return nil
		}
	}

	return []larkcore.RequestOptionFunc{larkcore.WithUserAccessToken(token)}
}
