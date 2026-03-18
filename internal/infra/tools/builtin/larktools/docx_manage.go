package larktools

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkclient "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"
)

func executeCreateDoc(ctx context.Context, client *larkclient.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	title := strings.TrimSpace(shared.StringArg(call.Arguments, "title"))
	folderID := strings.TrimSpace(shared.StringArg(call.Arguments, "folder_id"))

	doc, err := client.Docx().CreateDocument(ctx, larkclient.CreateDocumentRequest{
		Title:    title,
		FolderID: folderID,
	})
	if err != nil {
		return shared.ToolError(call.ID, "create_doc failed: %v", err)
	}

	// Best-effort: default to org-editable via link.
	_ = client.Drive().SetLinkShareEdit(ctx, doc.DocumentID, "docx")

	content := fmt.Sprintf("文档创建成功\ndocument_id: %s\ntitle: %s\nrevision_id: %d",
		doc.DocumentID, doc.Title, doc.RevisionID)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"action":      ActionCreateDoc,
			"document_id": doc.DocumentID,
			"title":       doc.Title,
			"revision_id": doc.RevisionID,
		},
	}, nil
}

func executeReadDoc(ctx context.Context, client *larkclient.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	doc, err := client.Docx().GetDocument(ctx, documentID)
	if err != nil {
		return shared.ToolError(call.ID, "read_doc failed: %v", err)
	}

	content := fmt.Sprintf("document_id: %s\ntitle: %s\nrevision_id: %d",
		doc.DocumentID, doc.Title, doc.RevisionID)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"action":      ActionReadDoc,
			"document_id": doc.DocumentID,
			"title":       doc.Title,
			"revision_id": doc.RevisionID,
		},
	}, nil
}

func executeReadDocContent(ctx context.Context, client *larkclient.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	raw, err := client.Docx().GetDocumentRawContent(ctx, documentID)
	if err != nil {
		return shared.ToolError(call.ID, "read_doc_content failed: %v", err)
	}

	if raw == "" {
		raw = "(empty)"
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: raw,
		Metadata: map[string]any{
			"action":      ActionReadDocContent,
			"document_id": documentID,
		},
	}, nil
}

func executeListDocBlocks(ctx context.Context, client *larkclient.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	documentID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "document_id")
	if errResult != nil {
		return errResult, nil
	}

	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	if pageSize <= 0 {
		pageSize = 50
	}
	pageToken := strings.TrimSpace(shared.StringArg(call.Arguments, "page_token"))

	blocks, nextToken, hasMore, err := client.Docx().ListDocumentBlocks(ctx, documentID, pageSize, pageToken)
	if err != nil {
		return shared.ToolError(call.ID, "list_doc_blocks failed: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("文档 %s 共 %d 个 block:\n", documentID, len(blocks)))
	for _, b := range blocks {
		sb.WriteString(fmt.Sprintf("- block_id=%s type=%d parent=%s children=%d\n",
			b.BlockID, b.BlockType, b.ParentID, len(b.Children)))
	}
	if hasMore {
		sb.WriteString(fmt.Sprintf("\nhas_more=true next_page_token=%s", nextToken))
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
		Metadata: map[string]any{
			"action":      ActionListDocBlocks,
			"document_id": documentID,
			"block_count": len(blocks),
			"has_more":    hasMore,
		},
	}, nil
}

func executeUpdateDocBlock(ctx context.Context, client *larkclient.Client, call ports.ToolCall) (*ports.ToolResult, error) {
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

	result, err := client.Docx().UpdateDocumentBlockText(ctx, larkclient.UpdateDocumentBlockTextRequest{
		DocumentID: documentID,
		BlockID:    blockID,
		Content:    content,
	})
	if err != nil {
		return shared.ToolError(call.ID, "update_doc_block failed: %v", err)
	}

	out := fmt.Sprintf("Block 更新成功\nblock_id: %s\nblock_type: %d\ndocument_revision_id: %d",
		result.Block.BlockID, result.Block.BlockType, result.DocumentRevisionID)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: out,
		Metadata: map[string]any{
			"action":               ActionUpdateDocBlock,
			"document_id":          documentID,
			"block_id":             result.Block.BlockID,
			"block_type":           result.Block.BlockType,
			"document_revision_id": result.DocumentRevisionID,
		},
	}, nil
}

func executeWriteDocMarkdown(ctx context.Context, client *larkclient.Client, call ports.ToolCall) (*ports.ToolResult, error) {
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

	if err := client.Docx().WriteMarkdown(ctx, documentID, blockID, content); err != nil {
		return shared.ToolError(call.ID, "write_doc_markdown failed: %v", err)
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Markdown 写入成功\ndocument_id: %s\nparent_block_id: %s", documentID, blockID),
		Metadata: map[string]any{
			"action":      ActionWriteDocMarkdown,
			"document_id": documentID,
			"block_id":    blockID,
		},
	}, nil
}
