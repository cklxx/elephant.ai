package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/tools/builtin/shared"
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
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	taskID, err := requireString(call.Arguments, "task_id")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	requestID, err := requireString(call.Arguments, "request_id")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	approved := false
	if raw, ok := call.Arguments["approved"]; ok {
		val, ok := raw.(bool)
		if !ok {
			err := fmt.Errorf("approved must be a boolean")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		approved = val
	}

	optionID := ""
	if raw, ok := call.Arguments["option_id"]; ok {
		if str, ok := raw.(string); ok {
			optionID = strings.TrimSpace(str)
		} else {
			err := fmt.Errorf("option_id must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	message := ""
	if raw, ok := call.Arguments["message"]; ok {
		if str, ok := raw.(string); ok {
			message = strings.TrimSpace(str)
		} else {
			err := fmt.Errorf("message must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		err := fmt.Errorf("background task dispatch is not available in this context")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	responder, ok := dispatcher.(agent.ExternalInputResponder)
	if !ok {
		err := fmt.Errorf("external input responder is not available in this context")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
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
