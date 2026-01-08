package builtin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/sandbox"
)

type sandboxWriteAttachmentTool struct {
	client *sandbox.Client
}

func NewSandboxWriteAttachment(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxWriteAttachmentTool{client: newSandboxClient(cfg)}
}

func (t *sandboxWriteAttachmentTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "sandbox_write_attachment",
		Version:  "0.1.0",
		Category: "sandbox_files",
		Tags:     []string{"sandbox", "file", "attachment"},
	}
}

func (t *sandboxWriteAttachmentTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "sandbox_write_attachment",
		Description: "Write an existing attachment into the sandbox filesystem. " +
			"Use this to materialize generated artifacts (e.g., PPTX) inside the sandbox.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"attachment": {Type: "string", Description: "Attachment name or placeholder (e.g., deck.pptx or [deck.pptx])"},
				"path":       {Type: "string", Description: "Absolute target path in the sandbox"},
				"sudo":       {Type: "boolean", Description: "Use sudo privileges"},
			},
			Required: []string{"attachment", "path"},
		},
	}
}

func (t *sandboxWriteAttachmentTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	attachmentRef := strings.TrimSpace(stringArg(call.Arguments, "attachment"))
	if attachmentRef == "" {
		err := errors.New("attachment is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	path := strings.TrimSpace(stringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_write_attachment")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload, mimeType, err := resolveAttachmentPayload(ctx, attachmentRef)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	request := map[string]any{
		"file":     path,
		"content":  payload,
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

	bytesWritten := base64DecodedLen(payload)
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

func resolveAttachmentPayload(ctx context.Context, ref string) (string, string, error) {
	attachments, _ := ports.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return "", "", errors.New("no attachments available in current context")
	}

	name := normalizePlaceholder(ref)
	if name == "" {
		name = ref
	}

	att, ok := lookupAttachmentCaseInsensitive(attachments, name)
	if !ok {
		return "", "", fmt.Errorf("attachment not found: %s", name)
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	if data := strings.TrimSpace(att.Data); data != "" {
		return data, mediaType, nil
	}

	if dataURI := strings.TrimSpace(att.URI); strings.HasPrefix(strings.ToLower(dataURI), "data:") {
		payload, mimeType, err := decodeDataURI(dataURI)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(mimeType) == "" {
			mimeType = mediaType
		}
		return base64.StdEncoding.EncodeToString(payload), mimeType, nil
	}

	return "", "", errors.New("attachment has no inline data; regenerate with inline data to write to sandbox")
}

func base64DecodedLen(payload string) int {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return 0
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return len(decoded)
	}
	return len(trimmed)
}
