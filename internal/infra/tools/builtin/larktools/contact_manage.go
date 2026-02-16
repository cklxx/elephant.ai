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

// larkContactManage handles contact/directory operations via the unified channel tool.
type larkContactManage struct{}

func (t *larkContactManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "contact operations require a Lark chat context.",
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
	case "get_user":
		return t.getUser(ctx, client, call)
	case "list_users":
		return t.listUsers(ctx, client, call)
	case "get_department":
		return t.getDepartment(ctx, client, call)
	case "list_departments":
		return t.listDepartments(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported contact action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkContactManage) getUser(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	userID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "user_id")
	if errResult != nil {
		return errResult, nil
	}
	userIDType := shared.StringArg(call.Arguments, "user_id_type")

	user, err := client.Contact().GetUser(ctx, userID, userIDType)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to get user: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(user, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("User found: %s (%s)\n%s", user.Name, user.OpenID, string(payload)),
		Metadata: map[string]any{
			"user_id": user.UserID,
			"open_id": user.OpenID,
			"name":    user.Name,
		},
	}, nil
}

func (t *larkContactManage) listUsers(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	departmentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "department_id")
	if errResult != nil {
		return errResult, nil
	}
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")
	userIDType := shared.StringArg(call.Arguments, "user_id_type")

	resp, err := client.Contact().ListUsers(ctx, larkapi.ListUsersRequest{
		DepartmentID: departmentID,
		UserIDType:   userIDType,
		PageSize:     pageSize,
		PageToken:    pageToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list users: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(resp, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d user(s).\n%s", len(resp.Users), string(payload)),
		Metadata: map[string]any{
			"count":    len(resp.Users),
			"has_more": resp.HasMore,
		},
	}, nil
}

func (t *larkContactManage) getDepartment(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	departmentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "department_id")
	if errResult != nil {
		return errResult, nil
	}

	dept, err := client.Contact().GetDepartment(ctx, departmentID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to get department: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(dept, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Department: %s\n%s", dept.Name, string(payload)),
		Metadata: map[string]any{
			"department_id": dept.DepartmentID,
			"name":          dept.Name,
		},
	}, nil
}

func (t *larkContactManage) listDepartments(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	parentID := shared.StringArg(call.Arguments, "parent_department_id")
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Contact().ListDepartments(ctx, larkapi.ListDepartmentsRequest{
		ParentDepartmentID: parentID,
		PageSize:           pageSize,
		PageToken:          pageToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list departments: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(resp, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d department(s).\n%s", len(resp.Departments), string(payload)),
		Metadata: map[string]any{
			"count":    len(resp.Departments),
			"has_more": resp.HasMore,
		},
	}, nil
}
