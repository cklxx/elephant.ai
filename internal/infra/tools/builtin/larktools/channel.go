package larktools

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type larkChannel struct {
	shared.BaseTool
	chat     *larkSendMessage
	history  *larkChatHistory
	upload   *larkUploadFile
	calCreate *larkCalendarCreate
	calQuery  *larkCalendarQuery
	calUpdate *larkCalendarUpdate
	calDelete *larkCalendarDelete
	task      *larkTaskManage
}

// NewLarkChannel constructs a unified Lark channel tool that dispatches to
// sub-handlers via the "action" parameter. It replaces the 8 individual Lark
// tools (send_message, history, upload_file, create_event, query_events,
// update_event, delete_event, list_tasks/create_task/update_task/delete_task).
func NewLarkChannel() tools.ToolExecutor {
	return &larkChannel{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "channel",
				Description: "Unified Lark channel tool. Dispatches to messaging, calendar, and task operations via the 'action' parameter. " +
					"Actions: send_message (text to chat), upload_file (file attachment), history (chat history), " +
					"create_event/query_events/update_event/delete_event (calendar), " +
					"list_tasks/create_task/update_task/delete_task (tasks). " +
					"Write actions require approval. Only available inside a Lark chat context.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type: "string",
							Description: "Action to perform.",
							Enum: []any{
								"send_message", "upload_file", "history",
								"create_event", "query_events", "update_event", "delete_event",
								"list_tasks", "create_task", "update_task", "delete_task",
							},
						},
						// send_message params
						"content": {
							Type:        "string",
							Description: "Message text for send_message; task summary for create_task/update_task.",
						},
						// upload_file params
						"path": {
							Type:        "string",
							Description: "Local file path for upload_file.",
						},
						"attachment_name": {
							Type:        "string",
							Description: "Attachment name for upload_file.",
						},
						"file_name": {
							Type:        "string",
							Description: "Override filename for upload_file.",
						},
						"max_bytes": {
							Type:        "integer",
							Description: "Max upload size for upload_file (default 20MiB).",
						},
						// history params
						"page_size": {
							Type:        "integer",
							Description: "Number of items to retrieve (history: default 20 max 50; list_tasks: default 50 max 100; query_events: default 50 max 1000).",
						},
						"page_token": {
							Type:        "string",
							Description: "Pagination token for history/list_tasks/query_events.",
						},
						// calendar params
						"summary": {
							Type:        "string",
							Description: "Event title for create_event/update_event; task summary for create_task/update_task.",
						},
						"start_time": {
							Type:        "string",
							Description: "Unix seconds: start time for create_event/query_events/update_event; filter for history.",
						},
						"end_time": {
							Type:        "string",
							Description: "Unix seconds: end time for create_event/query_events/update_event; filter for history.",
						},
						"description": {
							Type:        "string",
							Description: "Description for create_event/update_event or create_task/update_task.",
						},
						"timezone": {
							Type:        "string",
							Description: "IANA timezone for create_event/update_event.",
						},
						"need_notification": {
							Type:        "boolean",
							Description: "Notify attendees for create_event (default true).",
						},
						"idempotency_key": {
							Type:        "string",
							Description: "Idempotency key for create_event.",
						},
						"event_id": {
							Type:        "string",
							Description: "Event ID for update_event/delete_event.",
						},
						// task params
						"task_id": {
							Type:        "string",
							Description: "Task GUID for update_task/delete_task.",
						},
						"completed": {
							Type:        "boolean",
							Description: "Filter by completed status for list_tasks.",
						},
						"type": {
							Type:        "string",
							Description: "Task list scope for list_tasks.",
						},
						"due_at": {
							Type:        "string",
							Description: "Due time as Unix seconds for create_task.",
						},
						"due_date": {
							Type:        "string",
							Description: "Due date (YYYY-MM-DD) for create_task.",
						},
						"due_time": {
							Type:        "string",
							Description: "Due time as Unix seconds for update_task.",
						},
						"assignee_ids": {
							Type:        "array",
							Description: "Assignee IDs for create_task.",
							Items:       &ports.Property{Type: "string"},
						},
						"assignee_type": {
							Type:        "string",
							Description: "Assignee type for create_task (default user).",
						},
						"user_id_type": {
							Type:        "string",
							Description: "User ID type for task operations.",
						},
						"user_access_token": {
							Type:        "string",
							Description: "User access token for task operations.",
						},
						"client_token": {
							Type:        "string",
							Description: "Idempotency token for create_task.",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:        "channel",
				Version:     "1.0.0",
				Category:    "lark",
				Tags:        []string{"lark", "channel", "chat", "calendar", "tasks"},
				SafetyLevel: ports.SafetyLevelReversible,
			},
		),
		chat:      &larkSendMessage{},
		history:   &larkChatHistory{},
		upload:    &larkUploadFile{},
		calCreate: &larkCalendarCreate{},
		calQuery:  &larkCalendarQuery{},
		calUpdate: &larkCalendarUpdate{},
		calDelete: &larkCalendarDelete{},
		task:      &larkTaskManage{},
	}
}

func (c *larkChannel) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "channel tool is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	if _, ok := rawClient.(*lark.Client); !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "channel: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	action, errResult := shared.RequireStringArg(call.Arguments, call.ID, "action")
	if errResult != nil {
		return errResult, nil
	}
	action = strings.ToLower(strings.TrimSpace(action))

	// Delegate to the appropriate handler. Each handler performs its own
	// parameter validation and context checks, so we pass the call through
	// with the original arguments (minus "action" which is consumed here).
	switch action {
	case "send_message":
		return c.chat.Execute(ctx, c.rewriteCall(call, "content"))
	case "upload_file":
		return c.upload.Execute(ctx, call)
	case "history":
		return c.history.Execute(ctx, call)
	case "create_event":
		return c.calCreate.Execute(ctx, call)
	case "query_events":
		return c.calQuery.Execute(ctx, call)
	case "update_event":
		return c.calUpdate.Execute(ctx, call)
	case "delete_event":
		return c.calDelete.Execute(ctx, call)
	case "list_tasks":
		return c.task.Execute(ctx, c.taskCall(call, "list"))
	case "create_task":
		return c.task.Execute(ctx, c.taskCall(call, "create"))
	case "update_task":
		return c.task.Execute(ctx, c.taskCall(call, "update"))
	case "delete_task":
		return c.task.Execute(ctx, c.taskCall(call, "delete"))
	default:
		err := fmt.Errorf("unsupported channel action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

// rewriteCall creates a new ToolCall that only passes through the named
// parameters, filtering out "action" and unknown keys that the delegate
// tool would reject. For send_message, the delegate validates "content" only.
func (c *larkChannel) rewriteCall(call ports.ToolCall, allowedKeys ...string) ports.ToolCall {
	allowed := make(map[string]bool, len(allowedKeys))
	for _, k := range allowedKeys {
		allowed[k] = true
	}
	args := make(map[string]any, len(allowedKeys))
	for k, v := range call.Arguments {
		if allowed[k] {
			args[k] = v
		}
	}
	return ports.ToolCall{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: args,
		SessionID: call.SessionID,
		TaskID:    call.TaskID,
	}
}

// taskCall rewrites the channel call into a task_manage call by injecting the
// correct "action" value for the task sub-handler.
func (c *larkChannel) taskCall(call ports.ToolCall, taskAction string) ports.ToolCall {
	args := make(map[string]any, len(call.Arguments))
	for k, v := range call.Arguments {
		if k == "action" {
			continue
		}
		args[k] = v
	}
	args["action"] = taskAction
	// Map "summary" to task's "summary" field (already matches).
	return ports.ToolCall{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: args,
		SessionID: call.SessionID,
		TaskID:    call.TaskID,
	}
}
