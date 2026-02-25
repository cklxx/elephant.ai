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

// larkDocxManage handles document CRUD via the unified channel tool.
type larkDocxManage struct{}

func (t *larkDocxManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "docx operations require a Lark chat context.",
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
		return t.createDoc(ctx, client, call)
	case "read":
		return t.readDoc(ctx, client, call)
	case "read_content":
		return t.readContent(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported docx action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkDocxManage) createDoc(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	title := shared.StringArg(call.Arguments, "title")
	folderID := shared.StringArg(call.Arguments, "folder_token")

	doc, err := client.Docx().CreateDocument(ctx, larkapi.CreateDocumentRequest{
		Title:    title,
		FolderID: folderID,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to create document: %v", err),
			Error:   err,
		}, nil
	}

	payload, _ := json.MarshalIndent(doc, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Document created successfully.\n%s", string(payload)),
		Metadata: map[string]any{
			"document_id": doc.DocumentID,
			"title":       doc.Title,
		},
	}, nil
}

func (t *larkDocxManage) readDoc(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	doc, err := client.Docx().GetDocument(ctx, documentID)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to get document: %v", err),
			Error:   err,
		}, nil
	}

	payload, _ := json.MarshalIndent(doc, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Document metadata:\n%s", string(payload)),
		Metadata: map[string]any{
			"document_id": doc.DocumentID,
			"title":       doc.Title,
		},
	}, nil
}

func (t *larkDocxManage) readContent(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	content, err := client.Docx().GetDocumentRawContent(ctx, documentID)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to read document content: %v", err),
			Error:   err,
		}, nil
	}

	if content == "" {
		content = "(empty document)"
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"document_id":    documentID,
			"content_length": len(content),
		},
	}, nil
}
