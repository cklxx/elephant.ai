package builtin

import (
	"bytes"
	"os"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"fmt"
	"os/exec"
)

type bash struct {
	mode    tools.ExecutionMode
	sandbox *tools.SandboxManager
}

func NewBash(cfg ShellToolConfig) ports.ToolExecutor {
	mode := cfg.Mode
	if mode == tools.ExecutionModeUnknown {
		mode = tools.ExecutionModeLocal
	}
	return &bash{mode: mode, sandbox: cfg.SandboxManager}
}

func (t *bash) Mode() tools.ExecutionMode {
	return t.mode
}

func (t *bash) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	command, ok := call.Arguments["command"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'command'")}, nil
	}

	if t.mode == tools.ExecutionModeSandbox {
		return t.executeSandbox(ctx, call, command)
	}
	return t.executeLocal(ctx, call, command)
}

func (t *bash) executeLocal(ctx context.Context, call ports.ToolCall, command string) (*ports.ToolResult, error) {
	// Auto-prepend working directory if command doesn't start with 'cd'
	// This ensures commands execute in the current working directory
	if !strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		// Get current working directory
		pwd, err := os.Getwd()
		if err == nil && pwd != "" {
			command = fmt.Sprintf("cd %q && %s", pwd, command)
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)

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

	// Build combined text output prioritizing stdout
	text := strings.TrimSpace(stdout)
	if text == "" {
		text = strings.TrimSpace(stderr)
	} else if stderr != "" {
		text = text + "\n" + strings.TrimSpace(stderr)
	}
	if text == "" {
		if runErr != nil {
			text = fmt.Sprintf("exit code %d (no output)", exitCode)
		} else {
			text = "command completed with no output"
		}
	}

	resultPayload := map[string]any{
		"command":   command,
		"exit_code": exitCode,
		"stdout":    stdout,
		"stderr":    stderr,
		"text":      text,
	}

	metadata := map[string]any{
		"command":      command,
		"exit_code":    exitCode,
		"stdout":       stdout,
		"stderr":       stderr,
		"text":         text,
		"stdout_bytes": stdoutBuf.Len(),
		"stderr_bytes": stderrBuf.Len(),
		"stdout_lines": countLines(stdoutBuf.String()),
		"stderr_lines": countLines(stderrBuf.String()),
		"success":      runErr == nil,
		"payload":      resultPayload,
	}
	if runErr != nil {
		metadata["error"] = runErr.Error()
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  text,
		Error:    runErr,
		Metadata: metadata,
	}, nil
}

func (t *bash) executeSandbox(ctx context.Context, call ports.ToolCall, command string) (*ports.ToolResult, error) {
	return executeSandboxCommand(ctx, t.sandbox, call, command)
}

func (t *bash) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "bash",
		Description: "Execute bash command",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"command": {Type: "string", Description: "Shell command"},
			},
			Required: []string{"command"},
		},
	}
}

func (t *bash) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "bash", Version: "1.0.0", Category: "execution", Dangerous: true,
	}
}
