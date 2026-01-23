package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"alex/internal/agent/ports"
	materialports "alex/internal/materials/ports"
	"alex/internal/sandbox"
)

type sandboxShellExecTool struct {
	client   *sandbox.Client
	uploader materialports.Migrator
}

func NewSandboxShellExec(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxShellExecTool{client: newSandboxClient(cfg), uploader: cfg.AttachmentUploader}
}

func (t *sandboxShellExecTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "sandbox_shell_exec",
		Version:  "0.1.0",
		Category: "sandbox_shell",
		Tags:     []string{"sandbox", "shell", "exec"},
	}
}

func (t *sandboxShellExecTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "sandbox_shell_exec",
		Description: "Execute a shell command inside the sandbox (isolated environment). Optionally fetch sandbox files as attachments.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"command":    {Type: "string", Description: "Shell command to execute"},
				"exec_dir":   {Type: "string", Description: "Absolute working directory"},
				"timeout":    {Type: "number", Description: "Timeout in seconds"},
				"async_mode": {Type: "boolean", Description: "Run asynchronously"},
				"session_id": {Type: "string", Description: "Optional shell session id"},
				"attachments": {
					Type:        "array",
					Description: "Optional list of sandbox file paths or attachment specs to fetch after execution.",
					Items:       &ports.Property{Type: "object"},
				},
				"output_files": {
					Type:        "array",
					Description: "Deprecated alias for attachments (array of absolute file paths).",
					Items:       &ports.Property{Type: "string"},
				},
			},
			Required: []string{"command"},
		},
	}
}

func (t *sandboxShellExecTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	command := strings.TrimSpace(stringArg(call.Arguments, "command"))
	if command == "" {
		err := errors.New("command is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload := map[string]any{
		"command": command,
	}
	if execDir := strings.TrimSpace(stringArg(call.Arguments, "exec_dir")); execDir != "" {
		payload["exec_dir"] = execDir
	}
	if value, ok := boolArgOptional(call.Arguments, "async_mode"); ok {
		payload["async_mode"] = value
	}
	if timeout, ok := floatArgOptional(call.Arguments, "timeout"); ok {
		payload["timeout"] = timeout
	}
	if sessionID := strings.TrimSpace(stringArg(call.Arguments, "session_id")); sessionID != "" {
		payload["id"] = sessionID
	}

	var response sandbox.Response[sandbox.ShellCommandResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/shell/exec", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox shell exec failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox shell exec returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content := fmt.Sprintf("Command status: %s", response.Data.Status)
	if response.Data.ExitCode != nil {
		content = fmt.Sprintf("%s (exit=%d)", content, *response.Data.ExitCode)
	}
	if response.Data.Output != nil && strings.TrimSpace(*response.Data.Output) != "" {
		content = fmt.Sprintf("%s\n\n%s", content, strings.TrimSpace(*response.Data.Output))
	}

	metadata := map[string]any{
		"session_id": response.Data.SessionID,
		"status":     response.Data.Status,
	}
	if response.Data.ExitCode != nil {
		metadata["exit_code"] = *response.Data.ExitCode
	}
	if response.Data.Output != nil {
		metadata["output"] = *response.Data.Output
	}
	if len(response.Data.Console) > 0 {
		payloadJSON, err := json.Marshal(response.Data.Console)
		if err == nil {
			metadata["console"] = json.RawMessage(payloadJSON)
		}
	}

	specs, err := parseSandboxAttachmentSpecs(call.Arguments)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	attachments, attachmentErrs := downloadSandboxAttachments(ctx, t.client, call.SessionID, specs, "sandbox_shell_exec")
	if len(attachments) > 0 {
		attachments = normalizeSandboxAttachments(ctx, attachments, t.uploader, "sandbox_shell_exec")
		content = fmt.Sprintf("%s\n\nAttachments: %s", content, formatAttachmentList(attachments))
	}
	if len(attachmentErrs) > 0 {
		metadata["attachment_errors"] = attachmentErrs
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func floatArgOptional(args map[string]any, key string) (float64, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
