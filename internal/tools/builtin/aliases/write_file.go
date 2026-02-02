package aliases

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
)

type writeFile struct {
	shared.BaseTool
}

func NewWriteFile(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &writeFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "write_file",
				Description: "Write content to a file (absolute paths only). Use encoding=base64 for binary data.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path":             {Type: "string", Description: "Absolute file path"},
						"content":          {Type: "string", Description: "Text content or base64 payload"},
						"encoding":         {Type: "string", Description: "Content encoding: utf-8 or base64"},
						"append":           {Type: "boolean", Description: "Append to the file instead of overwriting"},
						"leading_newline":  {Type: "boolean", Description: "Add a leading newline (text only)"},
						"trailing_newline": {Type: "boolean", Description: "Add a trailing newline (text only)"},
						"sudo":             {Type: "boolean", Description: "Use sudo privileges"},
					},
					Required: []string{"path", "content"},
				},
			},
			ports.ToolMetadata{
				Name:     "write_file",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "write"},
			},
		),
	}
}

func (t *writeFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content := shared.StringArg(call.Arguments, "content")
	if content == "" {
		err := errors.New("content is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	encoding := strings.TrimSpace(shared.StringArg(call.Arguments, "encoding"))
	appendMode, _ := boolArgOptional(call.Arguments, "append")
	leadingNewline, _ := boolArgOptional(call.Arguments, "leading_newline")
	trailingNewline, _ := boolArgOptional(call.Arguments, "trailing_newline")

	payload := []byte(content)
	if strings.EqualFold(encoding, "base64") {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		payload = decoded
	} else {
		text := content
		if leadingNewline {
			text = "\n" + text
		}
		if trailingNewline {
			text = text + "\n"
		}
		payload = []byte(text)
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

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

	result := &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote %d bytes to %s", bytesWritten, resolved),
		Metadata: map[string]any{
			"path":          resolved,
			"bytes_written": bytesWritten,
		},
	}

	attachments, errs := autoUploadFile(ctx, resolved)
	if len(attachments) > 0 {
		result.Attachments = attachments
	}
	if len(errs) > 0 {
		result.Metadata["attachment_errors"] = errs
	}

	return result, nil
}

func autoUploadFile(ctx context.Context, path string) (map[string]ports.Attachment, []string) {
	cfg := shared.GetAutoUploadConfig(ctx)
	if !cfg.Enabled {
		return nil, nil
	}
	spec := attachmentSpec{Path: path}
	attachments, errs := buildAttachmentsFromSpecs(ctx, []attachmentSpec{spec}, cfg)
	return attachments, errs
}
