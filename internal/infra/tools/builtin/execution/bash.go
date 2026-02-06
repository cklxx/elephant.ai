//go:build no_local_exec

package execution

import (
	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	"context"
	"fmt"
)

type bash struct {
	shared.BaseTool
}

func NewBash(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &bash{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "bash",
				Description: "Execute bash command",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"command": {Type: "string", Description: "Shell command"},
					},
					Required: []string{"command"},
				},
			},
			ports.ToolMetadata{
				Name: "bash", Version: "1.0.0", Category: "execution", Dangerous: true,
			},
		),
	}
}

func (t *bash) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	err := fmt.Errorf("local bash execution is disabled in this build; rebuild without -tags=no_local_exec to enable")
	return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
}
