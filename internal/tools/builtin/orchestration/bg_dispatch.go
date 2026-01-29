package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
)

type bgDispatch struct{}

// NewBGDispatch creates the bg_dispatch tool for launching background tasks.
func NewBGDispatch() *bgDispatch {
	return &bgDispatch{}
}

func (t *bgDispatch) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "bg_dispatch",
		Description: `Dispatch a task to run in the background. The task executes asynchronously while you continue working. Use bg_status to check progress and bg_collect to retrieve results.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"task_id": {
					Type:        "string",
					Description: "A unique identifier for this background task.",
				},
				"description": {
					Type:        "string",
					Description: "A short human-readable description of the task.",
				},
				"prompt": {
					Type:        "string",
					Description: "The full task prompt to execute in the background.",
				},
				"agent_type": {
					Type:        "string",
					Description: `Agent type to use. "internal" (default) uses the built-in subagent. Future types: "claude_code", "cursor", etc.`,
				},
			},
			Required: []string{"task_id", "description", "prompt"},
		},
	}
}

func (t *bgDispatch) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "bg_dispatch",
		Version:  "1.0.0",
		Category: "agent",
		Tags:     []string{"background", "orchestration", "async"},
	}
}

func (t *bgDispatch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Validate parameters.
	for key := range call.Arguments {
		switch key {
		case "task_id", "description", "prompt", "agent_type":
		default:
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	taskID, err := requireString(call.Arguments, "task_id")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	description, err := requireString(call.Arguments, "description")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	prompt, err := requireString(call.Arguments, "prompt")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	agentType := "internal"
	if raw, ok := call.Arguments["agent_type"]; ok {
		if str, ok := raw.(string); ok && strings.TrimSpace(str) != "" {
			agentType = strings.TrimSpace(str)
		}
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		err := fmt.Errorf("background task dispatch is not available in this context")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if err := dispatcher.Dispatch(ctx, taskID, description, prompt, agentType, call.ID); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Background task %q dispatched successfully. Use bg_status(task_ids=[\"%s\"]) to check progress.", taskID, taskID),
	}, nil
}

func requireString(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	trimmed := strings.TrimSpace(str)
	if trimmed == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}
	return trimmed, nil
}
