package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils"
)

// larkDocxManage handles document CRUD via the unified channel tool.
type larkDocxManage struct{}

func (t *larkDocxManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	sdkClient, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}
	client := larkapi.Wrap(sdkClient)

	action := utils.TrimLower(shared.StringArg(call.Arguments, "action"))
	switch action {
	case "create":
		return t.createDoc(ctx, client, call)
	case "read":
		return t.readDoc(ctx, client, call)
	case "read_content":
		return t.readContent(ctx, client, call)
	case "list_blocks":
		return t.listBlocks(ctx, client, call)
	case "update_block_text":
		return t.updateBlockText(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported docx action: %s", action)
		return shared.ToolError(call.ID, "%v", err)
	}
}

func (t *larkDocxManage) createDoc(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	title := shared.StringArg(call.Arguments, "title")
	content := strings.TrimSpace(shared.StringArg(call.Arguments, "content"))
	if content == "" {
		content = strings.TrimSpace(shared.StringArg(call.Arguments, "description"))
	}
	folderID := shared.StringArg(call.Arguments, "folder_token")

	doc, err := client.Docx().CreateDocument(ctx, larkapi.CreateDocumentRequest{
		Title:    title,
		FolderID: folderID,
	})
	if err != nil {
		return apiErr(call.ID, "create document", err), nil
	}

	grantSenderEditPermission(ctx, client, doc.DocumentID, "docx")

	var contentBlockID string
	if content != "" {
		blockID, err := t.writeInitialContent(ctx, client, doc.DocumentID, content)
		if err != nil {
			return apiErr(call.ID, "write initial document content", err), nil
		}
		contentBlockID = blockID
	}

	payload, _ := json.MarshalIndent(doc, "", "  ")
	metadata := map[string]any{
		"document_id": doc.DocumentID,
		"title":       doc.Title,
	}
	if contentBlockID != "" {
		metadata["content_written"] = true
		metadata["content_block_id"] = contentBlockID
	}
	resultContent := fmt.Sprintf("Document created successfully.\n%s", string(payload))
	if docURL := larkapi.BuildDocumentURL(shared.LarkBaseDomainFromContext(ctx), doc.DocumentID); docURL != "" {
		metadata["url"] = docURL
		resultContent = fmt.Sprintf("Document created successfully.\nURL: %s\n%s", docURL, string(payload))
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  resultContent,
		Metadata: metadata,
	}, nil
}

func (t *larkDocxManage) writeInitialContent(ctx context.Context, client *larkapi.Client, documentID, content string) (string, error) {
	blocks, _, _, err := client.Docx().ListDocumentBlocks(ctx, documentID, 50, "")
	if err != nil {
		return "", err
	}
	blockID := pickWritableBlockID(blocks)
	if blockID == "" {
		return "", fmt.Errorf("no writable text block found in document %s", documentID)
	}
	_, err = client.Docx().UpdateDocumentBlockText(ctx, larkapi.UpdateDocumentBlockTextRequest{
		DocumentID:         documentID,
		BlockID:            blockID,
		Content:            content,
		DocumentRevisionID: -1,
	})
	if err != nil {
		return "", err
	}
	return blockID, nil
}

func pickWritableBlockID(blocks []larkapi.DocumentBlock) string {
	for i := range blocks {
		if blocks[i].BlockType == 2 && blocks[i].BlockID != "" {
			return blocks[i].BlockID
		}
	}
	for i := range blocks {
		if blocks[i].BlockType != 1 && blocks[i].BlockID != "" {
			return blocks[i].BlockID
		}
	}
	return ""
}

func (t *larkDocxManage) readDoc(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	doc, err := client.Docx().GetDocument(ctx, documentID)
	if err != nil {
		return apiErr(call.ID, "get document", err), nil
	}

	payload, _ := json.MarshalIndent(doc, "", "  ")
	metadata := map[string]any{
		"document_id": doc.DocumentID,
		"title":       doc.Title,
	}
	content := fmt.Sprintf("Document metadata:\n%s", string(payload))
	if docURL := larkapi.BuildDocumentURL(shared.LarkBaseDomainFromContext(ctx), doc.DocumentID); docURL != "" {
		metadata["url"] = docURL
		content = fmt.Sprintf("Document metadata:\nURL: %s\n%s", docURL, string(payload))
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func (t *larkDocxManage) readContent(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	content, err := client.Docx().GetDocumentRawContent(ctx, documentID)
	if err != nil {
		return apiErr(call.ID, "read document content", err), nil
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

func (t *larkDocxManage) listBlocks(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	blocks, nextToken, hasMore, err := client.Docx().ListDocumentBlocks(ctx, documentID, pageSize, pageToken)
	if err != nil {
		return apiErr(call.ID, "list document blocks", err), nil
	}

	if len(blocks) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No blocks found in document.",
		}, nil
	}

	payload, _ := json.MarshalIndent(blocks, "", "  ")
	metadata := map[string]any{
		"document_id": documentID,
		"block_count": len(blocks),
	}
	if hasMore {
		metadata["has_more"] = true
		metadata["page_token"] = nextToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d blocks:\n%s", len(blocks), string(payload)),
		Metadata: metadata,
	}, nil
}

func (t *larkDocxManage) updateBlockText(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}
	blockID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "block_id")
	if errResult != nil {
		return errResult, nil
	}
	content, errResult := shared.RequireStringArg(call.Arguments, call.ID, "content")
	if errResult != nil {
		return errResult, nil
	}

	documentRevisionID, hasRevision := shared.IntArg(call.Arguments, "document_revision_id")
	if !hasRevision {
		documentRevisionID = -1
	}

	updateResult, err := client.Docx().UpdateDocumentBlockText(ctx, larkapi.UpdateDocumentBlockTextRequest{
		DocumentID:         documentID,
		BlockID:            blockID,
		Content:            content,
		DocumentRevisionID: documentRevisionID,
		ClientToken:        shared.StringArg(call.Arguments, "client_token"),
		UserIDType:         shared.StringArg(call.Arguments, "user_id_type"),
	})
	if err != nil {
		return apiErr(call.ID, "update document block text", err), nil
	}

	payload := map[string]any{
		"block": updateResult.BlockData,
	}
	if updateResult.DocumentRevisionID > 0 {
		payload["document_revision_id"] = updateResult.DocumentRevisionID
	}
	if updateResult.ClientToken != "" {
		payload["client_token"] = updateResult.ClientToken
	}
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")

	metadata := map[string]any{
		"document_id": documentID,
		"block_id":    updateResult.Block.BlockID,
	}
	if updateResult.DocumentRevisionID > 0 {
		metadata["document_revision_id"] = updateResult.DocumentRevisionID
	}
	if updateResult.ClientToken != "" {
		metadata["client_token"] = updateResult.ClientToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Block updated successfully.\n%s", string(payloadJSON)),
		Metadata: metadata,
	}, nil
}
