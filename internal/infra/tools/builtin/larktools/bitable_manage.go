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

// larkBitableManage handles bitable table/record operations via the unified channel tool.
type larkBitableManage struct{}

func (t *larkBitableManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "bitable operations require a Lark chat context.",
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
	case "list_tables":
		return t.listTables(ctx, client, call)
	case "list_records":
		return t.listRecords(ctx, client, call)
	case "create_record":
		return t.createRecord(ctx, client, call)
	case "update_record":
		return t.updateRecord(ctx, client, call)
	case "delete_record":
		return t.deleteRecord(ctx, client, call)
	case "list_fields":
		return t.listFields(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported bitable action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkBitableManage) listTables(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	appToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "app_token")
	if errResult != nil {
		return errResult, nil
	}

	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Bitable().ListTables(ctx, larkapi.ListTablesRequest{
		AppToken:  appToken,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to list tables: %v", err),
			Error:   err,
		}, nil
	}

	if len(resp.Tables) == 0 {
		return &ports.ToolResult{CallID: call.ID, Content: "No tables found."}, nil
	}

	payload, _ := json.MarshalIndent(resp.Tables, "", "  ")
	metadata := map[string]any{"table_count": len(resp.Tables)}
	if resp.HasMore {
		metadata["has_more"] = true
		metadata["page_token"] = resp.PageToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d tables:\n%s", len(resp.Tables), string(payload)),
		Metadata: metadata,
	}, nil
}

func (t *larkBitableManage) listRecords(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	appToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "app_token")
	if errResult != nil {
		return errResult, nil
	}
	tableID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "table_id")
	if errResult != nil {
		return errResult, nil
	}

	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Bitable().ListRecords(ctx, larkapi.ListRecordsRequest{
		AppToken:  appToken,
		TableID:   tableID,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to list records: %v", err),
			Error:   err,
		}, nil
	}

	if len(resp.Records) == 0 {
		return &ports.ToolResult{CallID: call.ID, Content: "No records found."}, nil
	}

	payload, _ := json.MarshalIndent(resp.Records, "", "  ")
	metadata := map[string]any{"record_count": len(resp.Records), "total": resp.Total}
	if resp.HasMore {
		metadata["has_more"] = true
		metadata["page_token"] = resp.PageToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d records (total %d):\n%s", len(resp.Records), resp.Total, string(payload)),
		Metadata: metadata,
	}, nil
}

func (t *larkBitableManage) createRecord(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	appToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "app_token")
	if errResult != nil {
		return errResult, nil
	}
	tableID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "table_id")
	if errResult != nil {
		return errResult, nil
	}

	fieldsRaw, ok := call.Arguments["fields"]
	if !ok || fieldsRaw == nil {
		err := fmt.Errorf("fields parameter is required for create_record")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	fieldsMap, ok := fieldsRaw.(map[string]interface{})
	if !ok {
		err := fmt.Errorf("fields must be a JSON object, got %T", fieldsRaw)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	record, err := client.Bitable().CreateRecord(ctx, appToken, tableID, fieldsMap)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to create record: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Record created successfully.",
		Metadata: map[string]any{
			"record_id": record.RecordID,
		},
	}, nil
}

func (t *larkBitableManage) updateRecord(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	appToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "app_token")
	if errResult != nil {
		return errResult, nil
	}
	tableID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "table_id")
	if errResult != nil {
		return errResult, nil
	}
	recordID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "record_id")
	if errResult != nil {
		return errResult, nil
	}

	fieldsRaw, ok := call.Arguments["fields"]
	if !ok || fieldsRaw == nil {
		err := fmt.Errorf("fields parameter is required for update_record")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	fieldsMap, ok := fieldsRaw.(map[string]interface{})
	if !ok {
		err := fmt.Errorf("fields must be a JSON object, got %T", fieldsRaw)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	_, err := client.Bitable().UpdateRecord(ctx, appToken, tableID, recordID, fieldsMap)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to update record: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Record updated successfully.",
		Metadata: map[string]any{
			"record_id": recordID,
		},
	}, nil
}

func (t *larkBitableManage) deleteRecord(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	appToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "app_token")
	if errResult != nil {
		return errResult, nil
	}
	tableID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "table_id")
	if errResult != nil {
		return errResult, nil
	}
	recordID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "record_id")
	if errResult != nil {
		return errResult, nil
	}

	err := client.Bitable().DeleteRecord(ctx, appToken, tableID, recordID)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to delete record: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Record deleted successfully.",
		Metadata: map[string]any{
			"record_id": recordID,
		},
	}, nil
}

func (t *larkBitableManage) listFields(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	appToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "app_token")
	if errResult != nil {
		return errResult, nil
	}
	tableID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "table_id")
	if errResult != nil {
		return errResult, nil
	}

	fields, err := client.Bitable().ListFields(ctx, appToken, tableID)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to list fields: %v", err),
			Error:   err,
		}, nil
	}

	payload, _ := json.MarshalIndent(fields, "", "  ")
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d fields:\n%s", len(fields), string(payload)),
		Metadata: map[string]any{"field_count": len(fields)},
	}, nil
}
