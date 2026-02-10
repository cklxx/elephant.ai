package aliases

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

type shellExec struct {
	shared.BaseTool
}

func NewShellExec(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &shellExec{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "shell_exec",
				Description: "Execute shell/CLI commands in the environment (process checks, logs, grep, runtime snapshots, git/test runs). Use this for terminal evidence collection and command-driven diagnostics. For deterministic code snippets/calculations/recalculations use execute_code.",
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
							Description: "Optional list of file paths or attachment specs to fetch after execution.",
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
			},
			ports.ToolMetadata{
				Name:     "shell_exec",
				Version:  "0.1.0",
				Category: "shell",
				Tags:     []string{"shell", "exec", "command", "cli", "terminal", "process", "logs", "runtime", "git", "test"},
			},
		),
	}
}

func (t *shellExec) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if !localExecEnabled {
		err := errors.New("local shell execution is disabled in this build")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	command := strings.TrimSpace(shared.StringArg(call.Arguments, "command"))
	if command == "" {
		err := errors.New("command is required")
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

	timeoutSeconds, _ := floatArgOptional(call.Arguments, "timeout")
	timeout := time.Duration(0)
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds * float64(time.Second))
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	workingDir := execDir
	if workingDir == "" {
		resolver := pathutil.GetPathResolverFromContext(ctx)
		workingDir = resolver.ResolvePath(".")
	}

	script, err := os.CreateTemp("", "alex-bash-*.sh")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	defer func() { _ = os.Remove(script.Name()) }()

	if _, err := script.WriteString(command); err != nil {
		_ = script.Close()
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if err := script.Close(); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if err := os.Chmod(script.Name(), 0o755); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	cmd := exec.CommandContext(runCtx, "bash", script.Name())
	if workingDir != "" {
		cmd.Dir = workingDir
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
		"session_id": call.SessionID,
		"status":     status,
		"exit_code":  exitCode,
		"output":     output,
		"command":    command,
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
