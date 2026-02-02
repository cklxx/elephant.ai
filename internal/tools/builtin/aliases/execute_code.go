package aliases

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/execution"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
)

type executeCode struct {
	shared.BaseTool
}

func NewExecuteCode(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &executeCode{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "execute_code",
				Description: `Execute code using the available local runtimes.

Supported languages: python, go, javascript/js, bash.
Provide inline code or reference an existing file via code_path.
Optionally fetch output files as attachments.`,
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
				Tags:     []string{"code", "execute", "run"},
			},
		),
	}
}

func (t *executeCode) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if !execution.LocalExecEnabled {
		err := errors.New("local code execution is disabled in this build")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

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

	execDir := strings.TrimSpace(shared.StringArg(call.Arguments, "exec_dir"))
	if execDir != "" {
		resolved, err := pathutil.ResolveLocalPath(ctx, execDir)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		execDir = resolved
	}
	if execDir == "" {
		resolver := pathutil.GetPathResolverFromContext(ctx)
		execDir = resolver.ResolvePath(".")
	}

	timeoutSeconds, _ := floatArgOptional(call.Arguments, "timeout")
	timeout := time.Duration(0)
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds * float64(time.Second))
	}

	provenance := "inline"
	execPath := codePath
	if execPath != "" {
		resolved, err := pathutil.ResolveLocalPath(ctx, execPath)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		execPath = resolved
		provenance = "file"
	} else {
		tempDir := execDir
		if tempDir == "" {
			tempDir = os.TempDir()
		}
		tmpDir, err := os.MkdirTemp(tempDir, "alex-exec-*")
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		filename := "code" + codeFileExtension(language)
		execPath = filepath.Join(tmpDir, filename)
		if language == "bash" && !strings.HasPrefix(strings.TrimSpace(code), "#!") {
			code = "#!/bin/bash\n" + strings.TrimRight(code, "\n")
		}
		if err := os.WriteFile(execPath, []byte(code), 0o644); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	command, err := buildCodeCommand(language, execPath)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, "bash", "-c", command)
	if execDir != "" {
		cmd.Dir = execDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	output := strings.TrimSpace(stdout)
	if output == "" {
		output = strings.TrimSpace(stderr)
	} else if strings.TrimSpace(stderr) != "" {
		output = output + "\n" + strings.TrimSpace(stderr)
	}

	status := "completed"
	if runErr != nil {
		status = "failed"
	}

	content := fmt.Sprintf("Command status: %s", status)
	content = fmt.Sprintf("%s (exit=%d)", content, exitCode)
	if output != "" {
		content = fmt.Sprintf("%s\n\n%s", content, output)
	}

	metadata := map[string]any{
		"language":        language,
		"command":         command,
		"session_id":      call.SessionID,
		"status":          status,
		"exit_code":       exitCode,
		"output":          output,
		"code_path":       execPath,
		"code_provenance": provenance,
	}

	specs, err := parseAttachmentSpecs(call.Arguments)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	uploadCfg := shared.GetAutoUploadConfig(ctx)
	uploadCfg.Enabled = true
	attachments, attachmentErrs := buildAttachmentsFromSpecs(ctx, specs, uploadCfg)
	if len(attachments) > 0 {
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
		Error:       runErr,
	}, nil
}

func buildCodeCommand(language, path string) (string, error) {
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
