//go:build local_exec

package builtin

import (
	"bytes"
	"os"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"context"
	"fmt"
	"os/exec"
)

type bash struct {
}

func NewBash(cfg ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &bash{}
}

func (t *bash) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	command, ok := call.Arguments["command"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'command'")}, nil
	}

	resolver := GetPathResolverFromContext(ctx)
	workingDir := resolver.ResolvePath(".")
	script, err := os.CreateTemp("", "alex-bash-*.sh")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to create temp script: %w", err)}, nil
	}
	defer func() { _ = os.Remove(script.Name()) }()

	if _, err := script.WriteString(command); err != nil {
		_ = script.Close()
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to write command script: %w", err)}, nil
	}
	if err := script.Close(); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to close command script: %w", err)}, nil
	}
	if err := os.Chmod(script.Name(), 0o755); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to chmod command script: %w", err)}, nil
	}

	cmd := exec.CommandContext(ctx, "bash", script.Name())
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

func countLines(output string) int {
	if output == "" {
		return 0
	}
	return strings.Count(output, "\n") + 1
}
