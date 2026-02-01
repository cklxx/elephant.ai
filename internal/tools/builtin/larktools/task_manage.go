package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

type larkTaskManage struct {
	shared.BaseTool
}

// NewLarkTaskManage constructs a tool for listing, creating, updating, and deleting tasks.
func NewLarkTaskManage() tools.ToolExecutor {
	return &larkTaskManage{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_task_manage",
				Description: "List, create, update, or delete Lark tasks. Write actions (create, update, delete) require approval.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform: list, create, update, or delete.",
							Enum:        []any{"list", "create", "update", "delete"},
						},
						"task_id": {
							Type:        "string",
							Description: "Task GUID for update or delete.",
						},
						"page_size": {
							Type:        "integer",
							Description: "Page size for list (default 50).",
						},
						"page_token": {
							Type:        "string",
							Description: "Pagination token for list.",
						},
						"completed": {
							Type:        "boolean",
							Description: "Filter by completed status for list.",
						},
						"type": {
							Type:        "string",
							Description: "Task list scope (e.g., my_tasks).",
						},
						"summary": {
							Type:        "string",
							Description: "Task summary for create or update.",
						},
						"description": {
							Type:        "string",
							Description: "Task description for create or update.",
						},
						"due_at": {
							Type:        "string",
							Description: "Due time as Unix seconds for create.",
						},
						"due_date": {
							Type:        "string",
							Description: "Due date (YYYY-MM-DD) for all-day tasks.",
						},
						"due_time": {
							Type:        "string",
							Description: "Due time as Unix seconds for update.",
						},
						"assignee_ids": {
							Type:        "array",
							Description: "Assignee IDs for create.",
							Items:       &ports.Property{Type: "string"},
						},
						"assignee_type": {
							Type:        "string",
							Description: "Assignee type for create (default user).",
						},
						"user_id_type": {
							Type:        "string",
							Description: "User ID type (open_id, user_id, union_id).",
						},
						"user_access_token": {
							Type:        "string",
							Description: "Optional user access token for user-scoped task operations.",
						},
						"client_token": {
							Type:        "string",
							Description: "Optional idempotency token for create.",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:     "lark_task_manage",
				Version:  "0.1.0",
				Category: "lark",
				Tags:     []string{"lark", "tasks"},
			},
		),
	}
}

func (t *larkTaskManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_task_manage is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_task_manage: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	action, errResult := shared.RequireStringArg(call.Arguments, call.ID, "action")
	if errResult != nil {
		return errResult, nil
	}
	action = strings.ToLower(strings.TrimSpace(action))

	switch action {
	case "list":
		return t.listTasks(ctx, client, call)
	case "create":
		return t.createTask(ctx, client, call)
	case "update":
		return t.updateTask(ctx, client, call)
	case "delete":
		return t.deleteTask(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkTaskManage) listTasks(ctx context.Context, client *lark.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	pageSize := clampTaskPageSize(call.Arguments)
	builder := larktask.NewListTaskReqBuilder().PageSize(pageSize)

	if pageToken := shared.StringArg(call.Arguments, "page_token"); pageToken != "" {
		builder.PageToken(pageToken)
	}
	if completed, ok := boolArg(call.Arguments, "completed"); ok {
		builder.Completed(completed)
	}
	if scope := shared.StringArg(call.Arguments, "type"); scope != "" {
		builder.Type(scope)
	}
	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}

	options := taskRequestOptions(call.Arguments)
	resp, err := client.Task.V2.Task.List(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(list): API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(list): API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No tasks found.",
		}, nil
	}

	summaries := summarizeTasks(resp.Data.Items)
	payload, _ := json.MarshalIndent(summaries, "", "  ")
	content := fmt.Sprintf("Found %d tasks:\n%s", len(summaries), string(payload))

	metadata := map[string]any{
		"task_count": len(summaries),
	}
	if resp.Data.PageToken != nil {
		metadata["page_token"] = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		metadata["has_more"] = *resp.Data.HasMore
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func (t *larkTaskManage) createTask(ctx context.Context, client *lark.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	if approvalErr := requireActionApproval(ctx, call, "lark_task_create"); approvalErr != nil {
		return approvalErr, nil
	}

	summary, errResult := shared.RequireStringArg(call.Arguments, call.ID, "summary")
	if errResult != nil {
		return errResult, nil
	}

	description := shared.StringArg(call.Arguments, "description")

	due, errResult := parseDue(call.Arguments, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	members := buildMembers(shared.StringSliceArg(call.Arguments, "assignee_ids"), shared.StringArg(call.Arguments, "assignee_type"))

	input := &larktask.InputTask{
		Summary: &summary,
	}
	if description != "" {
		input.Description = &description
	}
	if due != nil {
		input.Due = due
	}
	if len(members) > 0 {
		input.Members = members
	}
	if clientToken := shared.StringArg(call.Arguments, "client_token"); clientToken != "" {
		input.ClientToken = &clientToken
	}

	builder := larktask.NewCreateTaskReqBuilder().InputTask(input)
	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}

	options := taskRequestOptions(call.Arguments)
	resp, err := client.Task.V2.Task.Create(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(create): API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(create): API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	guid := ""
	if resp.Data != nil && resp.Data.Task != nil && resp.Data.Task.Guid != nil {
		guid = *resp.Data.Task.Guid
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Task created successfully.",
		Metadata: map[string]any{
			"task_id": guid,
		},
	}, nil
}

func (t *larkTaskManage) updateTask(ctx context.Context, client *lark.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	if approvalErr := requireActionApproval(ctx, call, "lark_task_update"); approvalErr != nil {
		return approvalErr, nil
	}

	taskID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "task_id")
	if errResult != nil {
		return errResult, nil
	}

	input := &larktask.InputTask{}
	var updateFields []string

	if summary := shared.StringArg(call.Arguments, "summary"); summary != "" {
		input.Summary = &summary
		updateFields = append(updateFields, "summary")
	}
	if description := shared.StringArg(call.Arguments, "description"); description != "" {
		input.Description = &description
		updateFields = append(updateFields, "description")
	}
	if dueTimeRaw := shared.StringArg(call.Arguments, "due_time"); dueTimeRaw != "" {
		seconds, err := parseUnixSecondsString(dueTimeRaw)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		ms := seconds * 1000
		value := fmt.Sprintf("%d", ms)
		input.Due = &larktask.Due{Timestamp: &value}
		updateFields = append(updateFields, "due")
	}

	if len(updateFields) == 0 {
		err := fmt.Errorf("update requires at least one field to change (summary, description, or due_time)")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	body := larktask.NewPatchTaskReqBodyBuilder().
		Task(input).
		UpdateFields(updateFields).
		Build()

	builder := larktask.NewPatchTaskReqBuilder().
		TaskGuid(taskID).
		Body(body)
	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}

	options := taskRequestOptions(call.Arguments)
	resp, err := client.Task.V2.Task.Patch(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(update): API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(update): API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Task updated successfully.",
		Metadata: map[string]any{
			"task_id":        taskID,
			"updated_fields": updateFields,
		},
	}, nil
}

func (t *larkTaskManage) deleteTask(ctx context.Context, client *lark.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	if approvalErr := requireActionApproval(ctx, call, "lark_task_delete"); approvalErr != nil {
		return approvalErr, nil
	}

	taskID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "task_id")
	if errResult != nil {
		return errResult, nil
	}

	builder := larktask.NewDeleteTaskReqBuilder().TaskGuid(taskID)

	options := taskRequestOptions(call.Arguments)
	resp, err := client.Task.V2.Task.Delete(ctx, builder.Build(), options...)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(delete): API call failed: %v", err),
			Error:   fmt.Errorf("lark API call failed: %w", err),
		}, nil
	}
	if !resp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_task_manage(delete): API error code=%d msg=%s", resp.Code, resp.Msg),
			Error:   fmt.Errorf("lark API error: code=%d msg=%s", resp.Code, resp.Msg),
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Task deleted successfully.",
		Metadata: map[string]any{
			"task_id": taskID,
		},
	}, nil
}

type taskSummary struct {
	ID          string `json:"id"`
	Summary     string `json:"summary,omitempty"`
	Due         string `json:"due,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Assignees   int    `json:"assignees,omitempty"`
}

func summarizeTasks(items []*larktask.Task) []taskSummary {
	summaries := make([]taskSummary, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		summary := taskSummary{}
		if item.Guid != nil {
			summary.ID = *item.Guid
		}
		if item.Summary != nil {
			summary.Summary = *item.Summary
		}
		if item.Due != nil && item.Due.Timestamp != nil {
			summary.Due = *item.Due.Timestamp
		}
		if item.CompletedAt != nil {
			summary.CompletedAt = *item.CompletedAt
		}
		summary.Assignees = len(item.Members)
		summaries = append(summaries, summary)
	}
	return summaries
}

func buildMembers(ids []string, memberType string) []*larktask.Member {
	if len(ids) == 0 {
		return nil
	}
	memberType = strings.TrimSpace(memberType)
	if memberType == "" {
		memberType = "user"
	}
	members := make([]*larktask.Member, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		memberID := trimmed
		typeValue := memberType
		members = append(members, &larktask.Member{Id: &memberID, Type: &typeValue})
	}
	return members
}

func parseDue(args map[string]any, callID string) (*larktask.Due, *ports.ToolResult) {
	dueAtRaw := shared.StringArg(args, "due_at")
	dueDate := shared.StringArg(args, "due_date")
	if dueAtRaw == "" && dueDate == "" {
		return nil, nil
	}
	if dueAtRaw != "" && dueDate != "" {
		err := fmt.Errorf("provide only one of due_at or due_date")
		return nil, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	if dueDate != "" {
		parsed, err := time.Parse("2006-01-02", dueDate)
		if err != nil {
			err = fmt.Errorf("due_date must be YYYY-MM-DD")
			return nil, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
		}
		ms := parsed.UnixMilli()
		value := fmt.Sprintf("%d", ms)
		isAllDay := true
		return &larktask.Due{Timestamp: &value, IsAllDay: &isAllDay}, nil
	}

	seconds, err := parseUnixSecondsString(dueAtRaw)
	if err != nil {
		return nil, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	ms := seconds * 1000
	value := fmt.Sprintf("%d", ms)
	return &larktask.Due{Timestamp: &value}, nil
}

func parseUnixSecondsString(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("due_at cannot be empty")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("due_at must be a unix seconds timestamp")
	}
	return parsed, nil
}

func clampTaskPageSize(args map[string]any) int {
	pageSize := 50
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
		return 50
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

func requireActionApproval(ctx context.Context, call ports.ToolCall, operation string) *ports.ToolResult {
	approver := shared.GetApproverFromContext(ctx)
	if approver == nil || shared.GetAutoApproveFromContext(ctx) {
		return nil
	}

	req := &tools.ApprovalRequest{
		Operation:   operation,
		Summary:     fmt.Sprintf("Approval required for %s", operation),
		AutoApprove: shared.GetAutoApproveFromContext(ctx),
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		Arguments:   call.Arguments,
	}
	resp, err := approver.RequestApproval(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}
	}
	if resp == nil || !resp.Approved {
		return &ports.ToolResult{CallID: call.ID, Content: "operation rejected", Error: fmt.Errorf("operation rejected")}
	}
	return nil
}

func taskRequestOptions(args map[string]any) []larkcore.RequestOptionFunc {
	token := shared.StringArg(args, "user_access_token")
	if token == "" {
		return nil
	}
	return []larkcore.RequestOptionFunc{larkcore.WithUserAccessToken(token)}
}
