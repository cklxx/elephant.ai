package ui

import (
	"context"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type uiRequestUser struct {
	shared.BaseTool
}

func NewRequestUser() tools.ToolExecutor {
	return &uiRequestUser{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "request_user",
				Description: "UI tool: explicitly request a user decision/action and pause execution." +
					" Use for approval/consent/manual gate events (login, 2FA, CAPTCHA, external confirmation, release approval, go/no-go)." +
					" Do not use for internal planning headers; use clarify/plan for that.",
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
						"options": {
							Type:        "array",
							Description: "Optional selectable options shown to the user.",
							Items:       &ports.Property{Type: "string"},
						},
					},
					Required: []string{"message"},
				},
			},
			ports.ToolMetadata{
				Name:     "request_user",
				Version:  "1.0.0",
				Category: "ui",
				Tags:     []string{"ui", "user", "request", "approval", "consent", "signoff", "manual-gate", "human_confirmation", "pause_for_user"},
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
	options, errResult := parseOptionsArg(call)
	if errResult != nil {
		return errResult, nil
	}

	for key := range call.Arguments {
		switch key {
		case "message", "title", "reason", "options":
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
	if len(options) > 0 {
		metadata["options"] = append([]string(nil), options...)
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}, nil
}
