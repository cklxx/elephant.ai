package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/tools"

	api "github.com/agent-infra/sandbox-sdk-go"
)

func runSandboxCommandRaw(ctx context.Context, sandbox *tools.SandboxManager, command string) (string, int, error) {
	if sandbox == nil {
		return "", 0, fmt.Errorf("sandbox manager is required")
	}
	if err := sandbox.Initialize(ctx); err != nil {
		return "", 0, tools.FormatSandboxError(err)
	}

	resp, err := sandbox.Shell().ExecCommand(ctx, &api.ShellExecRequest{Command: command})
	if err != nil {
		return "", 0, tools.FormatSandboxError(err)
	}

	data := resp.GetData()
	var output string
	var exitCode int
	if data != nil {
		if out := data.GetOutput(); out != nil {
			output = *out
		}
		if code := data.GetExitCode(); code != nil {
			exitCode = *code
		}
	}

	return output, exitCode, nil
}

func executeSandboxCommand(ctx context.Context, sandbox *tools.SandboxManager, call ports.ToolCall, command string) (*ports.ToolResult, error) {
	output, exitCode, err := runSandboxCommandRaw(ctx, sandbox, command)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	runErr := error(nil)
	if exitCode != 0 {
		runErr = fmt.Errorf("sandbox command exited with code %d", exitCode)
	}

	payload := map[string]any{
		"command":   command,
		"stdout":    output,
		"stderr":    "",
		"exit_code": exitCode,
	}

	contentBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		plain := strings.TrimSpace(output)
		if plain == "" {
			plain = fmt.Sprintf("command %q completed", command)
		}
		return &ports.ToolResult{CallID: call.ID, Content: plain, Error: runErr}, nil
	}

	metadata := map[string]any{
		"command":      command,
		"exit_code":    exitCode,
		"stdout_bytes": len(output),
		"stderr_bytes": 0,
		"stdout_lines": countLines(output),
		"stderr_lines": 0,
		"success":      runErr == nil,
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(contentBytes),
		Error:    runErr,
		Metadata: metadata,
	}, nil
}

func countLines(output string) int {
	if output == "" {
		return 0
	}
	return strings.Count(output, "\n") + 1
}
