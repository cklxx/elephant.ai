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

// larkMailManage handles mail group operations via the unified channel tool.
type larkMailManage struct{}

func (t *larkMailManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "mail operations require a Lark chat context.",
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
	case "list_mailgroups":
		return t.listMailgroups(ctx, client, call)
	case "get_mailgroup":
		return t.getMailgroup(ctx, client, call)
	case "create_mailgroup":
		return t.createMailgroup(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported mail action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkMailManage) listMailgroups(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Mail().ListMailgroups(ctx, larkapi.ListMailgroupsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list mailgroups: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(resp, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d mailgroup(s).\n%s", len(resp.Mailgroups), string(payload)),
		Metadata: map[string]any{
			"count":    len(resp.Mailgroups),
			"has_more": resp.HasMore,
		},
	}, nil
}

func (t *larkMailManage) getMailgroup(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	mailgroupID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "mailgroup_id")
	if errResult != nil {
		return errResult, nil
	}

	mg, err := client.Mail().GetMailgroup(ctx, mailgroupID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to get mailgroup: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(mg, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Mailgroup: %s (%s)\n%s", mg.Name, mg.Email, string(payload)),
		Metadata: map[string]any{
			"mailgroup_id": mg.MailgroupID,
			"email":        mg.Email,
			"name":         mg.Name,
		},
	}, nil
}

func (t *larkMailManage) createMailgroup(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	email, errResult := shared.RequireStringArg(call.Arguments, call.ID, "email")
	if errResult != nil {
		return errResult, nil
	}
	name := shared.StringArg(call.Arguments, "name")
	description := shared.StringArg(call.Arguments, "description")

	mg, err := client.Mail().CreateMailgroup(ctx, larkapi.CreateMailgroupRequest{
		Email:       email,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to create mailgroup: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(mg, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Mailgroup created.\n%s", string(payload)),
		Metadata: map[string]any{
			"mailgroup_id": mg.MailgroupID,
			"email":        mg.Email,
		},
	}, nil
}
