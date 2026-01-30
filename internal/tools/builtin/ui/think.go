package ui

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
	"context"
	"fmt"
)

type think struct {
	shared.BaseTool
}

func NewThink() tools.ToolExecutor {
	return &think{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "think",
				Description: "Internal reasoning tool",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"thought": {Type: "string", Description: "Reasoning step"},
					},
					Required: []string{"thought"},
				},
			},
			ports.ToolMetadata{
				Name: "think", Version: "1.0.0", Category: "reasoning",
			},
		),
	}
}

func (t *think) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	thought, ok := call.Arguments["thought"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'thought'")}, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Thinking: %s", thought)}, nil
}

