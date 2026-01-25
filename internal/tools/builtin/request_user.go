package builtin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
)

type uiRequestUser struct{}

func NewRequestUser() tools.ToolExecutor {
	return &uiRequestUser{}
}

func (t *uiRequestUser) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "request_user",
		Version:  "1.0.0",
		Category: "ui",
		Tags:     []string{"ui", "user", "request"},
	}
}

func (t *uiRequestUser) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "request_user",
		Description: "UI tool: request the user to perform an action (e.g., login) and pause execution." +
			" Always use when an external login, 2FA, or CAPTCHA is required.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"message": {
					Type:        "string",
					Description: "Instruction to the user describing the required action.",
				},
				"title": {
					Type:        "string",
					Description: "Optional short title for the request.",
				},
				"reason": {
					Type:        "string",
					Description: "Optional reason or context for the request.",
				},
			},
			Required: []string{"message"},
		},
	}
}

func (t *uiRequestUser) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	message, ok := call.Arguments["message"].(string)
	if !ok {
		err := fmt.Errorf("missing 'message'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	message = strings.TrimSpace(message)
	if message == "" {
		err := errors.New("message cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	title := ""
	if raw, exists := call.Arguments["title"]; exists {
		value, ok := raw.(string)
		if !ok {
			err := fmt.Errorf("title must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		title = strings.TrimSpace(value)
	}

	reason := ""
	if raw, exists := call.Arguments["reason"]; exists {
		value, ok := raw.(string)
		if !ok {
			err := fmt.Errorf("reason must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		reason = strings.TrimSpace(value)
	}

	for key := range call.Arguments {
		switch key {
		case "message", "title", "reason":
		default:
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	content := message
	if title != "" {
		content = title + "\n" + message
	}

	metadata := map[string]any{
		"message":          message,
		"needs_user_input": true,
	}
	if title != "" {
		metadata["title"] = title
	}
	if reason != "" {
		metadata["reason"] = reason
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}, nil
}
