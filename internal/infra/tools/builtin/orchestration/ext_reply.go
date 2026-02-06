package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/tools/builtin/shared"
)

type extReply struct {
	shared.BaseTool
}

// NewExtReply creates the ext_reply tool for responding to external input requests.
func NewExtReply() *extReply {
	return &extReply{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "ext_reply",
				Description: `Reply to an input request from an external agent (Claude Code, Codex, etc).
Use this when an external background task requests permission or clarification.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"task_id": {
							Type:        "string",
							Description: "The background task ID.",
						},
						"request_id": {
							Type:        "string",
							Description: "The input request ID from the notification.",
						},
						"approved": {
							Type:        "boolean",
							Description: "Whether to approve (for permission requests).",
						},
						"option_id": {
							Type:        "string",
							Description: "Selected option ID (if applicable).",
						},
						"message": {
							Type:        "string",
							Description: "Free-form response text (for clarification requests).",
						},
					},
					Required: []string{"task_id", "request_id"},
				},
			},
			ports.ToolMetadata{
				Name:     "ext_reply",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "external"},
			},
		),
	}
}

func (t *extReply) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "task_id", "request_id", "approved", "option_id", "message":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	taskID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "task_id")
	if errResult != nil {
		return errResult, nil
	}
	requestID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "request_id")
	if errResult != nil {
		return errResult, nil
	}

	approved := false
	if raw, ok := call.Arguments["approved"]; ok {
		val, ok := raw.(bool)
		if !ok {
			return shared.ToolError(call.ID, "approved must be a boolean")
		}
		approved = val
	}

	optionID := ""
	if raw, ok := call.Arguments["option_id"]; ok {
		if str, ok := raw.(string); ok {
			optionID = strings.TrimSpace(str)
		} else {
			return shared.ToolError(call.ID, "option_id must be a string")
		}
	}

	message := ""
	if raw, ok := call.Arguments["message"]; ok {
		if str, ok := raw.(string); ok {
			message = strings.TrimSpace(str)
		} else {
			return shared.ToolError(call.ID, "message must be a string")
		}
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	responder, ok := dispatcher.(agent.ExternalInputResponder)
	if !ok {
		return shared.ToolError(call.ID, "external input responder is not available in this context")
	}

	if err := responder.ReplyExternalInput(ctx, agent.InputResponse{
		TaskID:    taskID,
		RequestID: requestID,
		Approved:  approved,
		OptionID:  optionID,
		Text:      message,
	}); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Reply sent for task %q request %q.", taskID, requestID),
	}, nil
}
