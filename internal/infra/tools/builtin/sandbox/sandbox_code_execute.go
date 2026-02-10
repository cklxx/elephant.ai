package sandbox

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	materialports "alex/internal/domain/materials/ports"
	"alex/internal/infra/sandbox"
	"alex/internal/infra/tools/builtin/shared"
)

type sandboxCodeExecuteTool struct {
	shared.BaseTool
	client   *sandbox.Client
	uploader materialports.Migrator
}

func NewSandboxCodeExecute(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxCodeExecuteTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "execute_code",
				Description: `Execute deterministic code snippets/scripts using local runtimes.

Supported languages: python, go, javascript/js, bash.
Provide inline code or reference an existing file via code_path.
Optionally fetch output files as attachments. Use this for deterministic computation/recalculation/invariant checks. For shell/process/log commands prefer shell_exec. Do not use for browser interaction or calendar querying.`,
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
							Description: "Absolute path to an existing code file.",
						},
						"exec_dir": {
							Type:        "string",
							Description: "Absolute working directory.",
						},
						"timeout": {
							Type:        "number",
							Description: "Timeout in seconds.",
						},
						"attachments": {
							Type:        "array",
							Description: "Optional list of file paths or attachment specs to fetch after execution.",
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
			},
			ports.ToolMetadata{
				Name:     "execute_code",
				Version:  "0.1.0",
				Category: "execution",
				Tags:     []string{"code", "execute", "run", "deterministic", "calculation", "recalculate", "metric", "formula", "invariant", "snippet"},
			},
		),
		client:   newSandboxClient(cfg),
		uploader: cfg.AttachmentUploader,
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

	data, errResult := doSandboxRequest[sandbox.ShellCommandResult](ctx, t.client, call.ID, call.SessionID, httpMethodPost, "/v1/shell/exec", payload, "sandbox code execute")
	if errResult != nil {
		return errResult, nil
	}

	content := fmt.Sprintf("Command status: %s", data.Status)
	if data.ExitCode != nil {
		content = fmt.Sprintf("%s (exit=%d)", content, *data.ExitCode)
	}
	if data.Output != nil && strings.TrimSpace(*data.Output) != "" {
		content = fmt.Sprintf("%s\n\n%s", content, strings.TrimSpace(*data.Output))
	}

	metadata := map[string]any{
		"language":   language,
		"command":    command,
		"session_id": data.SessionID,
		"status":     data.Status,
	}
	if data.ExitCode != nil {
		metadata["exit_code"] = *data.ExitCode
	}
	if data.Output != nil {
		metadata["output"] = *data.Output
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

	if _, err := doSandboxCall[sandbox.FileWriteResult](ctx, client, httpMethodPost, "/v1/file/write", request, sessionID, "sandbox code write"); err != nil {
		return "", err
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
