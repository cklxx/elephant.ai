// Package larktools implements the unified Lark "channel" tool that routes
// action-based requests to Feishu/Lark document APIs.
package larktools

import (
	"context"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	larkclient "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"
)

// Action constants for the channel tool.
const (
	ActionCreateDoc        = "create_doc"
	ActionReadDoc          = "read_doc"
	ActionReadDocContent   = "read_doc_content"
	ActionListDocBlocks    = "list_doc_blocks"
	ActionUpdateDocBlock   = "update_doc_block"
	ActionWriteDocMarkdown = "write_doc_markdown"
)

// actionSafety maps each action to its safety level.
var actionSafety = map[string]int{
	ActionCreateDoc:        ports.SafetyLevelHighImpact,
	ActionReadDoc:          ports.SafetyLevelReadOnly,
	ActionReadDocContent:   ports.SafetyLevelReadOnly,
	ActionListDocBlocks:    ports.SafetyLevelReadOnly,
	ActionUpdateDocBlock:   ports.SafetyLevelHighImpact,
	ActionWriteDocMarkdown: ports.SafetyLevelHighImpact,
}

// allActions is the ordered list of supported actions for the Enum.
var allActions = []any{
	ActionCreateDoc,
	ActionReadDoc,
	ActionReadDocContent,
	ActionListDocBlocks,
	ActionUpdateDocBlock,
	ActionWriteDocMarkdown,
}

type channelTool struct {
	shared.BaseTool
	client *larkclient.Client
}

// NewChannel creates a Lark channel tool backed by the given client.
func NewChannel(client *larkclient.Client) tools.ToolExecutor {
	return &channelTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "channel",
				Description: `Feishu/Lark document management tool. Use this tool to create, read, update, and write Feishu documents.

Actions:
- action="create_doc": Create a new document. Params: title (optional), folder_id (optional).
- action="read_doc": Get document metadata. Params: document_id (required).
- action="read_doc_content": Get document raw text content. Params: document_id (required).
- action="list_doc_blocks": List all blocks in a document. Params: document_id (required), page_size (optional), page_token (optional).
- action="update_doc_block": Update a text block's content. Params: document_id (required), block_id (required), content (required).
- action="write_doc_markdown": Write markdown content into a document. Params: document_id (required), block_id (required, parent block), content (required, markdown text).`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: "The document operation to perform.",
							Enum:        allActions,
						},
						"document_id": {
							Type:        "string",
							Description: "Document ID (required for read/update/list/write operations).",
						},
						"block_id": {
							Type:        "string",
							Description: "Block ID (required for update_doc_block and write_doc_markdown).",
						},
						"content": {
							Type:        "string",
							Description: "Text content for update_doc_block, or markdown content for write_doc_markdown.",
						},
						"title": {
							Type:        "string",
							Description: "Document title (used with create_doc).",
						},
						"folder_id": {
							Type:        "string",
							Description: "Target folder token (used with create_doc, optional).",
						},
						"page_size": {
							Type:        "integer",
							Description: "Page size for list_doc_blocks (default 50).",
						},
						"page_token": {
							Type:        "string",
							Description: "Pagination token for list_doc_blocks.",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:        "channel",
				Version:     "1.0.0",
				Category:    "lark",
				Tags:        []string{"lark", "feishu", "document", "docx", "channel"},
				SafetyLevel: ports.SafetyLevelHighImpact,
			},
		),
		client: client,
	}
}

// Execute dispatches to the appropriate action handler.
func (t *channelTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action := strings.TrimSpace(shared.StringArg(call.Arguments, "action"))
	if action == "" {
		return shared.ToolError(call.ID, "action is required")
	}

	if _, ok := actionSafety[action]; !ok {
		return shared.ToolError(call.ID, "unsupported action: %s", action)
	}

	switch action {
	case ActionCreateDoc:
		return executeCreateDoc(ctx, t.client, call)
	case ActionReadDoc:
		return executeReadDoc(ctx, t.client, call)
	case ActionReadDocContent:
		return executeReadDocContent(ctx, t.client, call)
	case ActionListDocBlocks:
		return executeListDocBlocks(ctx, t.client, call)
	case ActionUpdateDocBlock:
		return executeUpdateDocBlock(ctx, t.client, call)
	case ActionWriteDocMarkdown:
		return executeWriteDocMarkdown(ctx, t.client, call)
	default:
		return shared.ToolError(call.ID, "unsupported action: %s", action)
	}
}

// ActionSafetyLevel returns the safety level for a given action name.
func ActionSafetyLevel(action string) int {
	if level, ok := actionSafety[action]; ok {
		return level
	}
	return ports.SafetyLevelHighImpact
}
