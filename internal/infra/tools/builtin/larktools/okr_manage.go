package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// larkOKRManage handles OKR operations via the unified channel tool.
type larkOKRManage struct{}

func (t *larkOKRManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "OKR operations require a Lark chat context.",
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
	case "list_periods":
		return t.listPeriods(ctx, client, call)
	case "list_user_okrs":
		return t.listUserOKRs(ctx, client, call)
	case "batch_get":
		return t.batchGetOKRs(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported OKR action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkOKRManage) listPeriods(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.OKR().ListPeriods(ctx, larkapi.ListPeriodsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list OKR periods: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(resp, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d OKR period(s).\n%s", len(resp.Periods), string(payload)),
		Metadata: map[string]any{
			"count":    len(resp.Periods),
			"has_more": resp.HasMore,
		},
	}, nil
}

func (t *larkOKRManage) listUserOKRs(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	userID := shared.StringArg(call.Arguments, "user_id")
	if userID == "" {
		// Auto-resolve from context: the sender's open_id is stored by BuildBaseContext.
		userID = id.UserIDFromContext(ctx)
	}
	if userID == "" {
		err := fmt.Errorf("user_id is required (provide it explicitly or send from a Lark chat)")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	periodID := shared.StringArg(call.Arguments, "period_id")

	okrs, err := client.OKR().ListUserOKRs(ctx, userID, periodID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list user OKRs: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(okrs, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d OKR(s) for user %s.\n%s", len(okrs), userID, string(payload)),
		Metadata: map[string]any{
			"count":   len(okrs),
			"user_id": userID,
		},
	}, nil
}

func (t *larkOKRManage) batchGetOKRs(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	okrIDsRaw := call.Arguments["okr_ids"]
	var okrIDs []string
	switch v := okrIDsRaw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				okrIDs = append(okrIDs, s)
			}
		}
	case []string:
		okrIDs = v
	}
	if len(okrIDs) == 0 {
		err := fmt.Errorf("okr_ids parameter is required and must be a non-empty array")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	okrs, err := client.OKR().BatchGetOKRs(ctx, okrIDs)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to batch get OKRs: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(okrs, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Retrieved %d OKR(s).\n%s", len(okrs), string(payload)),
		Metadata: map[string]any{
			"count": len(okrs),
		},
	}, nil
}
