package larktools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils"
	"alex/internal/shared/utils/id"

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
							Description: "Action to perform: list, create, update, delete, list_subtasks, or create_subtask.",
							Enum:        []any{"list", "create", "update", "delete", "list_subtasks", "create_subtask"},
						},
						"task_id": {
							Type:        "string",
							Description: "Task GUID for update or delete.",
						},
						"parent_task_id": {
							Type:        "string",
							Description: "Parent task GUID for list_subtasks or create_subtask.",
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
							Description: "Task title (Feishu field: summary). Required for create; keep concise.",
						},
						"description": {
							Type:        "string",
							Description: "Task body text (Feishu field: description) for create or update.",
						},
						"content": {
							Type:        "string",
							Description: "Alias of description. Put detailed content here instead of summary. If summary has multiple lines, first line becomes title and remaining lines become description.",
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
				Name:        "lark_task_manage",
				Version:     "0.1.0",
				Category:    "lark",
				Tags:        []string{"lark", "tasks"},
				SafetyLevel: ports.SafetyLevelReversible,
			},
		),
	}
}

func (t *larkTaskManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	client, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	action, errResult := shared.RequireStringArg(call.Arguments, call.ID, "action")
	if errResult != nil {
		return errResult, nil
	}
	action = utils.TrimLower(action)

	switch action {
	case "list":
		return t.listTasks(ctx, client, call)
	case "create":
		return t.createTask(ctx, client, call)
	case "update":
		return t.updateTask(ctx, client, call)
	case "delete":
		return t.deleteTask(ctx, client, call)
	case "list_subtasks":
		return t.listSubtasks(ctx, client, call)
	case "create_subtask":
		return t.createSubtask(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported action: %s", action)
		return shared.ToolError(call.ID, "%w", err)
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

	_, options, errResult := taskRequestOptions(ctx, call.ID, call.Arguments, false)
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.Task.V2.Task.List(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_task_manage(list)", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_task_manage(list)", resp.Code, resp.Msg), nil
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

	summary, description := normalizeCreateTaskTextFields(
		shared.StringArg(call.Arguments, "summary"),
		shared.StringArg(call.Arguments, "description"),
		shared.StringArg(call.Arguments, "content"),
	)
	if summary == "" {
		err := fmt.Errorf("summary is required")
		return shared.ToolError(call.ID, "%w", err)
	}

	due, errResult := parseDue(call.Arguments, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	auth, options, errResult := taskRequestOptions(ctx, call.ID, call.Arguments, true)
	if errResult != nil {
		return errResult, nil
	}

	userIDType := shared.StringArg(call.Arguments, "user_id_type")
	members := buildMembers(shared.StringSliceArg(call.Arguments, "assignee_ids"), shared.StringArg(call.Arguments, "assignee_type"))
	members, autoAddedSender := ensureTenantTaskVisibilityMember(ctx, members, auth, userIDType)

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
	if userIDType != "" {
		builder.UserIdType(userIDType)
	}

	resp, err := client.Task.V2.Task.Create(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_task_manage(create)", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_task_manage(create)", resp.Code, resp.Msg), nil
	}

	guid := ""
	if resp.Data != nil && resp.Data.Task != nil && resp.Data.Task.Guid != nil {
		guid = *resp.Data.Task.Guid
	}

	metadata := map[string]any{
		"task_id":   guid,
		"auth_mode": auth.mode(),
	}
	if autoAddedSender {
		metadata["sender_added_as_member"] = true
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "Task created successfully.",
		Metadata: metadata,
	}, nil
}

func (t *larkTaskManage) listSubtasks(ctx context.Context, client *lark.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	parentTaskID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "parent_task_id")
	if errResult != nil {
		return errResult, nil
	}

	pageSize := clampTaskPageSize(call.Arguments)
	builder := larktask.NewListTaskSubtaskReqBuilder().
		TaskGuid(parentTaskID).
		PageSize(pageSize)

	if pageToken := shared.StringArg(call.Arguments, "page_token"); pageToken != "" {
		builder.PageToken(pageToken)
	}
	if userIDType := shared.StringArg(call.Arguments, "user_id_type"); userIDType != "" {
		builder.UserIdType(userIDType)
	}

	_, options, errResult := taskRequestOptions(ctx, call.ID, call.Arguments, false)
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.Task.V2.TaskSubtask.List(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_task_manage(list_subtasks)", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_task_manage(list_subtasks)", resp.Code, resp.Msg), nil
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No subtasks found.",
			Metadata: map[string]any{
				"parent_task_id": parentTaskID,
			},
		}, nil
	}

	summaries := summarizeTasks(resp.Data.Items)
	payload, _ := json.MarshalIndent(summaries, "", "  ")
	content := fmt.Sprintf("Found %d subtasks:\n%s", len(summaries), string(payload))

	metadata := map[string]any{
		"parent_task_id": parentTaskID,
		"subtask_count":  len(summaries),
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

func (t *larkTaskManage) createSubtask(ctx context.Context, client *lark.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	if approvalErr := requireActionApproval(ctx, call, "lark_task_create_subtask"); approvalErr != nil {
		return approvalErr, nil
	}

	parentTaskID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "parent_task_id")
	if errResult != nil {
		return errResult, nil
	}

	summary, description := normalizeCreateTaskTextFields(
		shared.StringArg(call.Arguments, "summary"),
		shared.StringArg(call.Arguments, "description"),
		shared.StringArg(call.Arguments, "content"),
	)
	if summary == "" {
		err := fmt.Errorf("summary is required")
		return shared.ToolError(call.ID, "%w", err)
	}

	due, errResult := parseDue(call.Arguments, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	auth, options, errResult := taskRequestOptions(ctx, call.ID, call.Arguments, true)
	if errResult != nil {
		return errResult, nil
	}

	userIDType := shared.StringArg(call.Arguments, "user_id_type")
	members := buildMembers(shared.StringSliceArg(call.Arguments, "assignee_ids"), shared.StringArg(call.Arguments, "assignee_type"))
	members, autoAddedSender := ensureTenantTaskVisibilityMember(ctx, members, auth, userIDType)

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

	builder := larktask.NewCreateTaskSubtaskReqBuilder().
		TaskGuid(parentTaskID).
		InputTask(input)
	if userIDType != "" {
		builder.UserIdType(userIDType)
	}

	resp, err := client.Task.V2.TaskSubtask.Create(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_task_manage(create_subtask)", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_task_manage(create_subtask)", resp.Code, resp.Msg), nil
	}

	guid := ""
	if resp.Data != nil && resp.Data.Subtask != nil && resp.Data.Subtask.Guid != nil {
		guid = *resp.Data.Subtask.Guid
	}

	metadata := map[string]any{
		"task_id":        guid,
		"parent_task_id": parentTaskID,
		"auth_mode":      auth.mode(),
	}
	if autoAddedSender {
		metadata["sender_added_as_member"] = true
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "Subtask created successfully.",
		Metadata: metadata,
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

	summary, description := normalizeUpdateTaskTextFields(
		shared.StringArg(call.Arguments, "summary"),
		shared.StringArg(call.Arguments, "description"),
		shared.StringArg(call.Arguments, "content"),
	)

	if summary != "" {
		input.Summary = &summary
		updateFields = append(updateFields, "summary")
	}
	if description != "" {
		input.Description = &description
		updateFields = append(updateFields, "description")
	}
	if dueTimeRaw := shared.StringArg(call.Arguments, "due_time"); dueTimeRaw != "" {
		seconds, err := parseUnixSecondsString(dueTimeRaw)
		if err != nil {
			return shared.ToolError(call.ID, "%w", err)
		}
		ms := seconds * 1000
		value := fmt.Sprintf("%d", ms)
		input.Due = &larktask.Due{Timestamp: &value}
		updateFields = append(updateFields, "due")
	}

	if len(updateFields) == 0 {
		err := fmt.Errorf("update requires at least one field to change (summary, description, or due_time)")
		return shared.ToolError(call.ID, "%w", err)
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

	_, options, errResult := taskRequestOptions(ctx, call.ID, call.Arguments, true)
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.Task.V2.Task.Patch(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_task_manage(update)", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_task_manage(update)", resp.Code, resp.Msg), nil
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

	_, options, errResult := taskRequestOptions(ctx, call.ID, call.Arguments, true)
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.Task.V2.Task.Delete(ctx, builder.Build(), options...)
	if err != nil {
		return sdkCallErr(call.ID, "lark_task_manage(delete)", err), nil
	}
	if !resp.Success() {
		return sdkRespErr(call.ID, "lark_task_manage(delete)", resp.Code, resp.Msg), nil
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

func ensureTenantTaskVisibilityMember(ctx context.Context, members []*larktask.Member, auth larkAccessToken, userIDType string) ([]*larktask.Member, bool) {
	if auth.kind != larkTokenTenant {
		return members, false
	}
	if userIDType != "" && !strings.EqualFold(strings.TrimSpace(userIDType), "open_id") {
		return members, false
	}

	senderID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if senderID == "" || hasTaskUserMember(members, senderID) {
		return members, false
	}

	memberID := senderID
	memberType := "user"
	role := "follower"
	members = append(members, &larktask.Member{
		Id:   &memberID,
		Type: &memberType,
		Role: &role,
	})
	return members, true
}

func hasTaskUserMember(members []*larktask.Member, memberID string) bool {
	for _, member := range members {
		if member == nil || member.Id == nil {
			continue
		}
		if strings.TrimSpace(*member.Id) != memberID {
			continue
		}
		memberType := "user"
		if member.Type != nil && strings.TrimSpace(*member.Type) != "" {
			memberType = strings.TrimSpace(*member.Type)
		}
		if strings.EqualFold(memberType, "user") {
			return true
		}
	}
	return false
}

func parseDue(args map[string]any, callID string) (*larktask.Due, *ports.ToolResult) {
	dueAtRaw := shared.StringArg(args, "due_at")
	dueDate := shared.StringArg(args, "due_date")
	if dueAtRaw == "" && dueDate == "" {
		return nil, nil
	}
	if dueAtRaw != "" && dueDate != "" {
		err := fmt.Errorf("provide only one of due_at or due_date")
		return nil, toolErrorResult(callID, "%w", err)
	}

	if dueDate != "" {
		parsed, err := time.Parse("2006-01-02", dueDate)
		if err != nil {
			err = fmt.Errorf("due_date must be YYYY-MM-DD")
			return nil, toolErrorResult(callID, "%w", err)
		}
		ms := parsed.UnixMilli()
		value := fmt.Sprintf("%d", ms)
		isAllDay := true
		return &larktask.Due{Timestamp: &value, IsAllDay: &isAllDay}, nil
	}

	seconds, err := parseUnixSecondsString(dueAtRaw)
	if err != nil {
		return nil, toolErrorResult(callID, "%w", err)
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

func normalizeCreateTaskTextFields(summary, description, content string) (string, string) {
	summary = strings.TrimSpace(summary)
	description = strings.TrimSpace(description)
	content = strings.TrimSpace(content)
	if description == "" && content != "" {
		description = content
	}
	if description == "" && summary != "" {
		title, body := splitTitleAndBody(summary)
		if body != "" {
			return title, body
		}
		compact := compactTaskTitle(title)
		if compact != "" && compact != title {
			return compact, title
		}
		return title, ""
	}
	if summary == "" && description != "" {
		title, body := splitTitleAndBody(description)
		if body != "" {
			return title, body
		}
		compact := compactTaskTitle(title)
		if compact == "" {
			return "", ""
		}
		if compact != title {
			return compact, title
		}
		return compact, ""
	}
	return summary, description
}

func normalizeUpdateTaskTextFields(summary, description, content string) (string, string) {
	summary = strings.TrimSpace(summary)
	description = strings.TrimSpace(description)
	content = strings.TrimSpace(content)
	if description == "" && content != "" {
		description = content
	}
	if description == "" && summary != "" {
		title, body := splitTitleAndBody(summary)
		if body != "" {
			return title, body
		}
	}
	return summary, description
}

func splitTitleAndBody(text string) (string, string) {
	normalized := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if normalized == "" {
		return "", ""
	}
	lines := strings.Split(normalized, "\n")
	title := strings.TrimSpace(lines[0])
	if len(lines) == 1 {
		return title, ""
	}
	body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	return title, body
}

const taskAutoTitleMaxRunes = 40

func compactTaskTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	if boundary := firstSentenceBoundary(text); boundary > 0 {
		text = strings.TrimSpace(text[:boundary])
	}
	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= taskAutoTitleMaxRunes {
		return text
	}
	return strings.TrimSpace(string(runes[:taskAutoTitleMaxRunes])) + "..."
}

func firstSentenceBoundary(text string) int {
	for idx, ch := range text {
		switch ch {
		case '。', '！', '？', '!', '?', ';', '；', ':', '：':
			return idx
		}
	}
	return -1
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
		return toolErrorResult(call.ID, "%w", err)
	}
	if resp == nil || !resp.Approved {
		return toolErrorResult(call.ID, "operation rejected")
	}
	return nil
}

func taskRequestOptions(ctx context.Context, callID string, args map[string]any, allowTenantFallback bool) (larkAccessToken, []larkcore.RequestOptionFunc, *ports.ToolResult) {
	token := strings.TrimSpace(shared.StringArg(args, "user_access_token"))
	if token != "" {
		auth := larkAccessToken{token: token, kind: larkTokenUser}
		_, reqOpt := buildLarkAuthOptions(auth)
		return auth, []larkcore.RequestOptionFunc{reqOpt}, nil
	}

	if !allowTenantFallback {
		resolved, errResult := resolveTaskUserToken(ctx, callID)
		if errResult != nil {
			return larkAccessToken{}, nil, errResult
		}
		auth := larkAccessToken{token: resolved, kind: larkTokenUser}
		_, reqOpt := buildLarkAuthOptions(auth)
		return auth, []larkcore.RequestOptionFunc{reqOpt}, nil
	}

	auth := resolveLarkTaskAuth(ctx)
	_, reqOpt := buildLarkAuthOptions(auth)
	return auth, []larkcore.RequestOptionFunc{reqOpt}, nil
}

func resolveTaskUserToken(ctx context.Context, callID string) (string, *ports.ToolResult) {
	svc := shared.LarkOAuthFromContext(ctx)
	if svc == nil {
		err := fmt.Errorf("task list requires user OAuth token")
		return "", toolErrorResult(callID, "%w", err)
	}

	openID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if openID == "" {
		err := fmt.Errorf("task list requires sender open_id")
		return "", toolErrorResult(callID, "%w", err)
	}

	token, err := svc.UserAccessToken(ctx, openID)
	if err != nil {
		var need *larkoauth.NeedUserAuthError
		if errors.As(err, &need) {
			url := strings.TrimSpace(need.AuthURL)
			if url == "" {
				url = strings.TrimSpace(svc.StartURL())
			}
			if url == "" {
				url = "(missing auth url)"
			}
			return "", &ports.ToolResult{
				CallID:  callID,
				Content: fmt.Sprintf("Please authorize Lark task access first: %s", url),
				Error:   err,
			}
		}
		return "", apiErr(callID, "load Lark user access token", err)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		err := fmt.Errorf("empty user_access_token returned")
		return "", toolErrorResult(callID, "%w", err)
	}
	return token, nil
}

func (a larkAccessToken) mode() string {
	switch a.kind {
	case larkTokenUser:
		return "user"
	case larkTokenTenant:
		return "tenant"
	default:
		return ""
	}
}
