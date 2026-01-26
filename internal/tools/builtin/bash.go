//go:build no_local_exec

package builtin

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"context"
	"fmt"
	"alex/internal/tools/builtin/shared"
)

type bash struct {
}

func NewBash(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &bash{}
}

func (t *bash) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	err := fmt.Errorf("local bash execution is disabled in this build; rebuild without -tags=no_local_exec to enable")
	return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
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
