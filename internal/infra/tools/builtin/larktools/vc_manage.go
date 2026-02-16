package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// larkVCManage handles video conference operations via the unified channel tool.
type larkVCManage struct{}

func (t *larkVCManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "vc operations require a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	sdkClient, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}
	client := larkapi.Wrap(sdkClient)

	action := strings.ToLower(strings.TrimSpace(shared.StringArg(call.Arguments, "action")))
	switch action {
	case "list_meetings":
		return t.listMeetings(ctx, client, call)
	case "get_meeting":
		return t.getMeeting(ctx, client, call)
	case "list_rooms":
		return t.listRooms(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported vc action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkVCManage) listMeetings(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	startTime, errResult := shared.RequireStringArg(call.Arguments, call.ID, "start_time")
	if errResult != nil {
		return errResult, nil
	}
	endTime, errResult := shared.RequireStringArg(call.Arguments, call.ID, "end_time")
	if errResult != nil {
		return errResult, nil
	}
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.VC().ListMeetings(ctx, larkapi.ListMeetingsRequest{
		StartTime: startTime,
		EndTime:   endTime,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list meetings: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(resp, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d meeting(s).\n%s", len(resp.Meetings), string(payload)),
		Metadata: map[string]any{
			"count":    len(resp.Meetings),
			"has_more": resp.HasMore,
		},
	}, nil
}

func (t *larkVCManage) getMeeting(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	meetingID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "meeting_id")
	if errResult != nil {
		return errResult, nil
	}

	meeting, err := client.VC().GetMeeting(ctx, meetingID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to get meeting: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(meeting, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Meeting: %s\n%s", meeting.Topic, string(payload)),
		Metadata: map[string]any{
			"meeting_id": meeting.MeetingID,
			"topic":      meeting.Topic,
		},
	}, nil
}

func (t *larkVCManage) listRooms(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	roomLevelID := shared.StringArg(call.Arguments, "room_level_id")
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.VC().ListRooms(ctx, larkapi.ListRoomsRequest{
		RoomLevelID: roomLevelID,
		PageSize:    pageSize,
		PageToken:   pageToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list rooms: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(resp, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d room(s).\n%s", len(resp.Rooms), string(payload)),
		Metadata: map[string]any{
			"count":    len(resp.Rooms),
			"has_more": resp.HasMore,
		},
	}, nil
}
