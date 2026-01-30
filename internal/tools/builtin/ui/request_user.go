package ui

import (
	"context"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
)

type uiRequestUser struct {
	shared.BaseTool
}

func NewRequestUser() tools.ToolExecutor {
	return &uiRequestUser{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
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
			},
			ports.ToolMetadata{
				Name:     "request_user",
				Version:  "1.0.0",
				Category: "ui",
				Tags:     []string{"ui", "user", "request"},
			},
		),
	}
}

func (t *uiRequestUser) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	message, errResult := shared.RequireStringArg(call.Arguments, call.ID, "message")
	if errResult != nil {
		return errResult, nil
	}

	title := ""
	if raw, exists := call.Arguments["title"]; exists {
		value, ok := raw.(string)
		if !ok {
			return shared.ToolError(call.ID, "title must be a string")
		}
		title = strings.TrimSpace(value)
	}

	reason := ""
	if raw, exists := call.Arguments["reason"]; exists {
		value, ok := raw.(string)
		if !ok {
			return shared.ToolError(call.ID, "reason must be a string")
		}
		reason = strings.TrimSpace(value)
	}

	for key := range call.Arguments {
		switch key {
		case "message", "title", "reason":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
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
