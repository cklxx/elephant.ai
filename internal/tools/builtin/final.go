package builtin

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
)

// finalTool captures the agent's synthesized answer and ends the run.
type finalTool struct{}

// NewFinal creates a tool that reports the final answer back to the coordinator.
func NewFinal() ports.ToolExecutor {
	return &finalTool{}
}

func (t *finalTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawAnswer, _ := call.Arguments["answer"].(string)
	answer := strings.TrimSpace(rawAnswer)
	if answer == "" {
		return nil, fmt.Errorf("final tool requires non-empty 'answer' content")
	}

	metadata := map[string]any{"final_tool": true}
	if highlights, ok := call.Arguments["highlights"]; ok {
		metadata["highlights"] = highlights
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  answer,
		Metadata: metadata,
	}, nil
}

func (t *finalTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "final",
		Description: "Use this tool to report the final response to the user and end the run.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"answer": {
					Type:        "string",
					Description: "Complete response that should be delivered to the user.",
				},
				"highlights": {
					Type:        "array",
					Description: "Optional key-value or bullet strings worth surfacing in the summary.",
				},
			},
			Required: []string{"answer"},
		},
	}
}

func (t *finalTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "final",
		Version:  "1.0.0",
		Category: "reasoning",
		Tags:     []string{"summary", "handoff"},
	}
}
