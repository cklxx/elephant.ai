package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os/exec"
)

type bash struct{}

func NewBash() ports.ToolExecutor {
	return &bash{}
}

func (t *bash) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	command, ok := call.Arguments["command"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'command'")}, nil
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: string(output), Error: err}, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: string(output)}, nil
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
