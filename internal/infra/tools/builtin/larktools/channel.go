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

// actionSafetyLevel returns the appropriate safety level for each action.
// Read-only actions return L1 (no approval), write actions return their
// original safety levels from the pre-consolidated individual tools.
func actionSafetyLevel(action string) (dangerous bool, level int) {
	switch action {
	case "history", "query_events", "list_tasks":
		return false, ports.SafetyLevelReadOnly
	case "send_message", "upload_file":
		return true, ports.SafetyLevelReversible
	case "create_event", "update_event", "create_task", "update_task":
		return true, ports.SafetyLevelHighImpact
	case "delete_event", "delete_task":
		return true, ports.SafetyLevelIrreversible
	default:
		return true, ports.SafetyLevelHighImpact
	}
}

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
				Description: "Unified Lark channel tool. Dispatches to thread messaging, calendar, and task operations via the 'action' parameter. " +
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
							Description: "Message text for send_message.",
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
				Tags:        []string{"lark", "channel", "chat", "thread", "message", "status_update", "notify", "calendar", "tasks"},
				Dangerous:   false, // Per-action approval handled inside Execute
				SafetyLevel: ports.SafetyLevelReadOnly,
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

	// Per-action approval: check safety level and request approval for
	// dangerous actions. Read-only actions (history, query_events, list_tasks)
	// skip approval entirely.
	dangerous, safetyLevel := actionSafetyLevel(action)
	if dangerous {
		if result, err := c.requireActionApproval(ctx, call, action, safetyLevel); result != nil || err != nil {
			return result, err
		}
	}

	// Strip "action" key and delegate to the appropriate handler.
	stripped := c.stripAction(call)

	switch action {
	case "send_message":
		return c.chat.Execute(ctx, c.rewriteCall(call, "content"))
	case "upload_file":
		return c.upload.Execute(ctx, stripped)
	case "history":
		return c.history.Execute(ctx, stripped)
	case "create_event":
		return c.calCreate.Execute(ctx, stripped)
	case "query_events":
		return c.calQuery.Execute(ctx, stripped)
	case "update_event":
		return c.calUpdate.Execute(ctx, stripped)
	case "delete_event":
		return c.calDelete.Execute(ctx, stripped)
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

// requireActionApproval checks with the approver for dangerous actions.
// Returns (nil, nil) if approved or no approver is set.
func (c *larkChannel) requireActionApproval(ctx context.Context, call ports.ToolCall, action string, safetyLevel int) (*ports.ToolResult, error) {
	approver := shared.GetApproverFromContext(ctx)
	if approver == nil || shared.GetAutoApproveFromContext(ctx) {
		return nil, nil
	}

	req := &tools.ApprovalRequest{
		Operation:   fmt.Sprintf("channel.%s", action),
		Summary:     fmt.Sprintf("Approval required for channel.%s (L%d)", action, safetyLevel),
		AutoApprove: false,
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		Arguments:   call.Arguments,
		SafetyLevel: safetyLevel,
	}
	if safetyLevel >= ports.SafetyLevelHighImpact {
		req.RollbackSteps = fmt.Sprintf("Revert the channel.%s operation via Lark admin or API.", action)
	}
	if safetyLevel >= ports.SafetyLevelIrreversible {
		req.AlternativePlan = "Prefer archive/disable first; verify impact in read-only mode before irreversible deletion."
	}

	resp, err := approver.RequestApproval(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if resp == nil || !resp.Approved {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("operation rejected")}, nil
	}
	return nil, nil
}

// stripAction creates a copy of the call with the "action" key removed from arguments.
func (c *larkChannel) stripAction(call ports.ToolCall) ports.ToolCall {
	args := make(map[string]any, len(call.Arguments))
	for k, v := range call.Arguments {
		if k == "action" {
			continue
		}
		args[k] = v
	}
	return ports.ToolCall{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: args,
		SessionID: call.SessionID,
		TaskID:    call.TaskID,
	}
}

// rewriteCall creates a new ToolCall that only passes through the named
// parameters, filtering out "action" and all other keys that the delegate
// tool would reject. Used for send_message where the delegate validates strictly.
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
