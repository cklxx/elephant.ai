package sandbox

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	materialports "alex/internal/materials/ports"
	"alex/internal/sandbox"
	"alex/internal/tools/builtin/shared"
)

type sandboxCodeExecuteTool struct {
	client   *sandbox.Client
	uploader materialports.Migrator
}

func NewSandboxCodeExecute(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxCodeExecuteTool{
		client:   newSandboxClient(cfg),
		uploader: cfg.AttachmentUploader,
	}
}

func (t *sandboxCodeExecuteTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "sandbox_code_execute",
		Version:  "0.1.0",
		Category: "sandbox_shell",
		Tags:     []string{"sandbox", "code", "execute"},
	}
}

func (t *sandboxCodeExecuteTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "sandbox_code_execute",
		Description: `Execute code inside the sandbox using the available runtimes.

Supported languages: python, go, javascript/js, bash.
Provide inline code or reference an existing sandbox file via code_path.
Optionally fetch output files from the sandbox as attachments.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"language": {
					Type:        "string",
					Description: "Programming language to execute",
					Enum:        []any{"python", "go", "javascript", "js", "bash"},
				},
				"code": {
					Type:        "string",
					Description: "Inline code to execute (ignored if code_path is provided).",
				},
				"code_path": {
					Type:        "string",
					Description: "Absolute path to a code file already present in the sandbox.",
				},
				"exec_dir": {
					Type:        "string",
					Description: "Absolute working directory inside the sandbox.",
				},
				"timeout": {
					Type:        "number",
					Description: "Timeout in seconds.",
				},
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
			Required: []string{"language"},
		},
	}
}

func (t *sandboxCodeExecuteTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	language := strings.ToLower(strings.TrimSpace(shared.StringArg(call.Arguments, "language")))
	if language == "js" {
		language = "javascript"
	}
	if language == "" {
		err := errors.New("language is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	codePath := strings.TrimSpace(shared.StringArg(call.Arguments, "code_path"))
	code := shared.StringArg(call.Arguments, "code")
	if codePath == "" && strings.TrimSpace(code) == "" {
		err := errors.New("code or code_path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if codePath != "" && !strings.HasPrefix(codePath, "/") {
		err := errors.New("code_path must be absolute")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	execPath := codePath
	if execPath == "" {
		path, err := writeSandboxCode(ctx, t.client, call.SessionID, language, code)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		execPath = path
	}

	command, err := buildSandboxCodeCommand(language, execPath)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload := map[string]any{
		"command": command,
	}
	if execDir := strings.TrimSpace(shared.StringArg(call.Arguments, "exec_dir")); execDir != "" {
		payload["exec_dir"] = execDir
	}
	if timeout, ok := floatArgOptional(call.Arguments, "timeout"); ok {
		payload["timeout"] = timeout
	}

	var response sandbox.Response[sandbox.ShellCommandResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/shell/exec", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox code execute failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox code execute returned empty payload")
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
		"language":   language,
		"command":    command,
		"session_id": response.Data.SessionID,
		"status":     response.Data.Status,
	}
	if response.Data.ExitCode != nil {
		metadata["exit_code"] = *response.Data.ExitCode
	}
	if response.Data.Output != nil {
		metadata["output"] = *response.Data.Output
	}
	if codePath == "" {
		metadata["code_path"] = execPath
		metadata["code_provenance"] = "inline"
	} else {
		metadata["code_path"] = codePath
		metadata["code_provenance"] = "sandbox_file"
	}

	specs, err := parseSandboxAttachmentSpecs(call.Arguments)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	attachments, attachmentErrs := downloadSandboxAttachments(ctx, t.client, call.SessionID, specs, "sandbox_code_execute")
	if len(attachments) > 0 {
		attachments = normalizeSandboxAttachments(ctx, attachments, t.uploader, "sandbox_code_execute")
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

func writeSandboxCode(ctx context.Context, client *sandbox.Client, sessionID, language, code string) (string, error) {
	if client == nil {
		return "", errors.New("sandbox client not configured")
	}

	code = strings.TrimRight(code, "\n")
	if language == "bash" && !strings.HasPrefix(code, "#!") {
		code = "#!/bin/bash\n" + code
	}

	filename := fmt.Sprintf("alex-sandbox-%d%s", time.Now().UnixNano(), codeFileExtension(language))
	path := filepath.ToSlash(filepath.Join("/tmp", filename))

	request := map[string]any{
		"file":    path,
		"content": code,
	}

	var response sandbox.Response[sandbox.FileWriteResult]
	if err := client.DoJSON(ctx, httpMethodPost, "/v1/file/write", request, sessionID, &response); err != nil {
		return "", err
	}
	if !response.Success {
		return "", fmt.Errorf("sandbox code write failed: %s", response.Message)
	}
	if response.Data == nil {
		return "", errors.New("sandbox code write returned empty payload")
	}

	return path, nil
}

func buildSandboxCodeCommand(language, path string) (string, error) {
	switch language {
	case "python":
		return fmt.Sprintf("python3 %s", path), nil
	case "go":
		return fmt.Sprintf("go run %s", path), nil
	case "javascript":
		return fmt.Sprintf("node %s", path), nil
	case "bash":
		return fmt.Sprintf("bash %s", path), nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func codeFileExtension(language string) string {
	switch language {
	case "python":
		return ".py"
	case "go":
		return ".go"
	case "javascript":
		return ".js"
	case "bash":
		return ".sh"
	default:
		return ".txt"
	}
}
