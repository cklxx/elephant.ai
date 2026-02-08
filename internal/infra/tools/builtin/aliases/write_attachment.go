package aliases

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/artifacts"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

type writeAttachment struct {
	shared.BaseTool
	httpClient *http.Client
}

func NewWriteAttachment(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &writeAttachment{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "write_attachment",
				Description: "Write an existing attachment to a local file path. " +
					"Accepts attachment placeholder/name, data URI, or HTTPS URL.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"attachment": {Type: "string", Description: "Attachment name or placeholder (e.g., deck.pptx or [deck.pptx])"},
						"path":       {Type: "string", Description: "Destination file path in the workspace"},
						"append":     {Type: "boolean", Description: "Append payload to existing file instead of overwrite"},
					},
					Required: []string{"attachment", "path"},
				},
			},
			ports.ToolMetadata{
				Name:     "write_attachment",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "attachment", "write"},
			},
		),
		httpClient: artifacts.NewAttachmentHTTPClient(artifacts.AttachmentFetchTimeout, "LocalAttachmentWrite"),
	}
}

func (t *writeAttachment) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	attachmentRef := strings.TrimSpace(shared.StringArg(call.Arguments, "attachment"))
	if attachmentRef == "" {
		err := errors.New("attachment is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	rawPath := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if rawPath == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, rawPath)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload, mediaType, err := artifacts.ResolveAttachmentBytes(ctx, attachmentRef, t.httpClient)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	appendMode, _ := boolArgOptional(call.Arguments, "append")
	bytesWritten := 0
	if appendMode {
		file, err := os.OpenFile(resolved, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		defer func() { _ = file.Close() }()
		n, err := file.Write(payload)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		bytesWritten = n
	} else {
		if err := os.WriteFile(resolved, payload, 0o644); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		bytesWritten = len(payload)
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote attachment to %s (%d bytes, %s)", resolved, bytesWritten, mediaType),
		Metadata: map[string]any{
			"path":           resolved,
			"bytes_written":  bytesWritten,
			"attachment_ref": attachmentRef,
			"media_type":     mediaType,
		},
	}, nil
}
