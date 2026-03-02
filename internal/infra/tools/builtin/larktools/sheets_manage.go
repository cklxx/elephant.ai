package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"
)

// larkSheetsManage handles spreadsheet operations via the unified channel tool.
type larkSheetsManage struct{}

func (t *larkSheetsManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	sdkClient, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
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

	grantSenderEditPermission(ctx, client, ss.SpreadsheetToken, "sheet")

	payload, _ := json.MarshalIndent(ss, "", "  ")
	metadata := map[string]any{
		"spreadsheet_token": ss.SpreadsheetToken,
		"title":             ss.Title,
	}
	content := fmt.Sprintf("Spreadsheet created.\n%s", string(payload))
	if ssURL := ss.URL; ssURL != "" {
		metadata["url"] = ssURL
	} else if ssURL := larkapi.BuildSpreadsheetURL(shared.LarkBaseDomainFromContext(ctx), ss.SpreadsheetToken); ssURL != "" {
		metadata["url"] = ssURL
		content = fmt.Sprintf("Spreadsheet created.\nURL: %s\n%s", ssURL, string(payload))
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
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
	metadata := map[string]any{
		"spreadsheet_token": ss.SpreadsheetToken,
		"title":             ss.Title,
	}
	content := fmt.Sprintf("Spreadsheet metadata:\n%s", string(payload))
	if ssURL := ss.URL; ssURL != "" {
		metadata["url"] = ssURL
	} else if ssURL := larkapi.BuildSpreadsheetURL(shared.LarkBaseDomainFromContext(ctx), ss.SpreadsheetToken); ssURL != "" {
		metadata["url"] = ssURL
		content = fmt.Sprintf("Spreadsheet metadata:\nURL: %s\n%s", ssURL, string(payload))
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
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
