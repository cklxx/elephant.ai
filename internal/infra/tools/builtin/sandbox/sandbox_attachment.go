package sandbox

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/sandbox"
	"alex/internal/infra/tools/builtin/artifacts"
	"alex/internal/infra/tools/builtin/shared"
)

type sandboxWriteAttachmentTool struct {
	shared.BaseTool
	client     *sandbox.Client
	httpClient *http.Client
}

func NewSandboxWriteAttachment(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxWriteAttachmentTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "write_attachment",
				Description: "Materialize an existing attachment into local filesystem path for downloadable handoff/review package delivery. " +
					"Accepts attachment name/placeholder, data URI, or HTTPS URL. This does not create new artifact content (use artifacts_write).",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"attachment": {Type: "string", Description: "Attachment name or placeholder (e.g., deck.pptx or [deck.pptx])"},
						"path":       {Type: "string", Description: "Absolute target path"},
						"sudo":       {Type: "boolean", Description: "Use sudo privileges"},
					},
					Required: []string{"attachment", "path"},
				},
			},
			ports.ToolMetadata{
				Name:     "write_attachment",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "attachment", "materialize", "download", "handoff"},
			},
		),
		client:     newSandboxClient(cfg),
		httpClient: artifacts.NewAttachmentHTTPClient(artifacts.AttachmentFetchTimeout, "SandboxAttachmentWrite"),
	}
}

func (t *sandboxWriteAttachmentTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	attachmentRef := strings.TrimSpace(shared.StringArg(call.Arguments, "attachment"))
	if attachmentRef == "" {
		err := errors.New("attachment is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_write_attachment")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload, mimeType, err := artifacts.ResolveAttachmentBytes(ctx, attachmentRef, t.httpClient)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	request := map[string]any{
		"file":     path,
		"content":  base64.StdEncoding.EncodeToString(payload),
		"encoding": "base64",
	}
	if value, ok := boolArgOptional(call.Arguments, "sudo"); ok {
		request["sudo"] = value
	}

	var response sandbox.Response[sandbox.FileWriteResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/file/write", request, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox write attachment failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox write attachment returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	bytesWritten := len(payload)
	if response.Data.BytesWritten != nil {
		bytesWritten = *response.Data.BytesWritten
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote attachment to %s (%d bytes, %s)", response.Data.File, bytesWritten, mimeType),
		Metadata: map[string]any{
			"path":           response.Data.File,
			"bytes_written":  bytesWritten,
			"attachment_ref": attachmentRef,
			"media_type":     mimeType,
		},
	}, nil
}
