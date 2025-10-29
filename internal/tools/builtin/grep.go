package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/tools"
	"context"
	"fmt"
	"os/exec"
)

type grep struct {
        mode    tools.ExecutionMode
        sandbox *tools.SandboxManager
}

func NewGrep(cfg ShellToolConfig) ports.ToolExecutor {
        mode := cfg.Mode
        if mode == tools.ExecutionModeUnknown {
                mode = tools.ExecutionModeLocal
        }
        return &grep{mode: mode, sandbox: cfg.SandboxManager}
}

func (t *grep) Mode() tools.ExecutionMode {
        return t.mode
}

func (t *grep) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	pattern, ok := call.Arguments["pattern"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'pattern'")}, nil
	}

	path, ok := call.Arguments["path"].(string)
	if !ok {
		path = "."
	}

	if t.mode == tools.ExecutionModeSandbox {
		command := fmt.Sprintf("grep -r -n %s %s", shellQuote(pattern), shellQuote(path))
		return executeSandboxCommand(ctx, t.sandbox, call, command)
	}

	cmd := exec.CommandContext(ctx, "grep", "-r", "-n", pattern, path)
	output, err := cmd.CombinedOutput()

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: string(output),
		Error:   err,
	}, nil
}

func (t *grep) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "grep",
		Description: "Search for pattern in files",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"pattern": {Type: "string", Description: "Search pattern"},
				"path":    {Type: "string", Description: "Path to search (default: .)"},
			},
			Required: []string{"pattern"},
		},
	}
}

func (t *grep) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "grep", Version: "1.0.0", Category: "search",
	}
}
