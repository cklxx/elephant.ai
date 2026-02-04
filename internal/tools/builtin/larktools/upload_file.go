package larktools

import (
	"context"
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
)

type larkUploadFile struct {
	shared.BaseTool
}

// NewLarkUploadFile constructs a tool that uploads a file and sends it to the
// current Lark chat as a "file" message.
func NewLarkUploadFile() tools.ToolExecutor {
	return &larkUploadFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_upload_file",
				Description: "Upload a file (from local path or task attachment) and send it to the current Lark chat as a file message. Only available inside a Lark chat context.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path": {
							Type:        "string",
							Description: "Local file path (must stay within the working directory). Provide exactly one of path or attachment_name.",
						},
						"attachment_name": {
							Type:        "string",
							Description: "Attachment name from the current task context. Provide exactly one of path or attachment_name.",
						},
						"file_name": {
							Type:        "string",
							Description: "Optional override for the uploaded file name.",
						},
						"reply_to_message_id": {
							Type:        "string",
							Description: "Optional message ID to reply to (threaded reply).",
						},
						"max_bytes": {
							Type:        "integer",
							Description: "Maximum upload size in bytes (default 20MiB).",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "lark_upload_file",
				Version:  "0.1.0",
				Category: "lark",
				Tags:     []string{"lark", "chat", "upload", "file"},
			},
		),
	}
}

func (t *larkUploadFile) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "lark_upload_file: not implemented",
		Error:   fmt.Errorf("lark_upload_file: not implemented"),
	}, nil
}
