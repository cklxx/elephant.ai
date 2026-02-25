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

// larkSheetsManage handles spreadsheet operations via the unified channel tool.
type larkSheetsManage struct{}

func (t *larkSheetsManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "sheets operations require a Lark chat context.",
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
	case "create":
		return t.createSpreadsheet(ctx, client, call)
	case "get":
		return t.getSpreadsheet(ctx, client, call)
	case "list_sheets":
		return t.listSheets(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported sheets action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkSheetsManage) createSpreadsheet(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	title := shared.StringArg(call.Arguments, "title")
	folderToken := shared.StringArg(call.Arguments, "folder_token")

	ss, err := client.Sheets().CreateSpreadsheet(ctx, larkapi.CreateSpreadsheetRequest{
		Title:       title,
		FolderToken: folderToken,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to create spreadsheet: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(ss, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Spreadsheet created.\n%s", string(payload)),
		Metadata: map[string]any{
			"spreadsheet_token": ss.SpreadsheetToken,
			"title":             ss.Title,
		},
	}, nil
}

func (t *larkSheetsManage) getSpreadsheet(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	token, errResult := shared.RequireStringArg(call.Arguments, call.ID, "spreadsheet_token")
	if errResult != nil {
		return errResult, nil
	}

	ss, err := client.Sheets().GetSpreadsheet(ctx, token)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to get spreadsheet: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(ss, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Spreadsheet metadata:\n%s", string(payload)),
		Metadata: map[string]any{
			"spreadsheet_token": ss.SpreadsheetToken,
			"title":             ss.Title,
		},
	}, nil
}

func (t *larkSheetsManage) listSheets(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	token, errResult := shared.RequireStringArg(call.Arguments, call.ID, "spreadsheet_token")
	if errResult != nil {
		return errResult, nil
	}

	sheets, err := client.Sheets().ListSheets(ctx, token)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Failed to list sheets: %v", err), Error: err}, nil
	}

	payload, _ := json.MarshalIndent(sheets, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Found %d sheet tab(s).\n%s", len(sheets), string(payload)),
		Metadata: map[string]any{
			"count": len(sheets),
		},
	}, nil
}
