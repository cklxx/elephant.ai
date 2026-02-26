package ui

import (
	"context"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
	"alex/internal/shared/utils"
)

type uiAskUser struct {
	shared.BaseTool
}

func NewAskUser() tools.ToolExecutor {
	return &uiAskUser{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "ask_user",
				Description: `UI tool: interact with the user — ask clarification questions OR request a decision/action and pause execution.

Actions:
- action="clarify": emit a task sub-header and ask targeted clarification questions when requirements are truly missing/contradictory.
  Do not use when the user already gave an explicit actionable operation with clear parameters.
- action="request": explicitly request a user decision/action and pause execution.
  Use for approval/consent/manual gate events (login, 2FA, CAPTCHA, external confirmation, release approval, go/no-go).

When needs_user_input=true, provide question_to_user and the orchestrator will pause for user input.
When waiting for user input, provide options to let channels render a selection UI.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: `"clarify" for intent clarification / task sub-header; "request" for user decision/action gate. Defaults to "clarify".`,
							Enum:        []any{"clarify", "request"},
						},
						"branch_id": {
							Type:        "string",
							Description: "Optional branch identifier.",
						},
						"task_id": {
							Type:        "string",
							Description: "Optional task identifier within the run. Auto-generated if omitted.",
						},
						"task_goal_ui": {
							Type:        "string",
							Description: "User-facing task header (used with action=clarify).",
						},
						"success_criteria": {
							Type:        "array",
							Description: "Optional string array of success criteria (UI hint, used with action=clarify).",
							Items:       &ports.Property{Type: "string"},
						},
						"message": {
							Type:        "string",
							Description: "Instruction to the user describing the required action (used with action=request).",
						},
						"title": {
							Type:        "string",
							Description: "Optional short title for the request.",
						},
						"reason": {
							Type:        "string",
							Description: "Optional reason or context for the request.",
						},
						"needs_user_input": {
							Type:        "boolean",
							Description: "Set true to pause and ask the user a question.",
						},
						"question_to_user": {
							Type:        "string",
							Description: "Question shown to the user when needs_user_input=true.",
						},
						"options": {
							Type:        "array",
							Description: "Optional selectable options shown to the user.",
							Items:       &ports.Property{Type: "string"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "ask_user",
				Version:  "2.0.0",
				Category: "ui",
				Tags:     []string{"ui", "orchestration", "clarification", "user", "request", "approval", "consent", "manual-gate"},
			},
		),
	}
}

func (t *uiAskUser) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action := strings.TrimSpace(shared.StringArg(call.Arguments, "action"))
	if action == "" {
		action = "clarify"
	}
	if action != "clarify" && action != "request" {
		return shared.ToolError(call.ID, "action must be \"clarify\" or \"request\"")
	}

	switch action {
	case "request":
		return t.executeRequest(call)
	default:
		return t.executeClarify(ctx, call)
	}
}

func (t *uiAskUser) executeClarify(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	runID := strings.TrimSpace(id.RunIDFromContext(ctx))

	taskID := shared.StringArg(call.Arguments, "task_id")
	if utils.IsBlank(taskID) {
		taskID = "task-" + call.ID
	}

	taskGoalUI := strings.TrimSpace(shared.StringArg(call.Arguments, "task_goal_ui"))
	if taskGoalUI == "" {
		return shared.ToolError(call.ID, "task_goal_ui is required for action=clarify")
	}

	branchID := ""
	if raw, exists := call.Arguments["branch_id"]; exists {
		if value, ok := raw.(string); ok {
			branchID = strings.TrimSpace(value)
		} else if raw != nil {
			return shared.ToolError(call.ID, "branch_id must be a string")
		}
	}

	var successCriteria []string
	if raw, exists := call.Arguments["success_criteria"]; exists {
		arr, ok := raw.([]any)
		if !ok {
			return shared.ToolError(call.ID, "success_criteria must be an array of strings")
		}
		for _, item := range arr {
			text, ok := item.(string)
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(text)
			if trimmed == "" {
				continue
			}
			successCriteria = append(successCriteria, trimmed)
		}
	}

	needsUserInput := false
	if raw, exists := call.Arguments["needs_user_input"]; exists {
		value, ok := raw.(bool)
		if !ok {
			return shared.ToolError(call.ID, "needs_user_input must be a boolean")
		}
		needsUserInput = value
	}

	questionToUser := ""
	if raw, exists := call.Arguments["question_to_user"]; exists {
		if value, ok := raw.(string); ok {
			questionToUser = strings.TrimSpace(value)
		} else if raw != nil {
			return shared.ToolError(call.ID, "question_to_user must be a string")
		}
	}
	if needsUserInput && questionToUser == "" {
		return shared.ToolError(call.ID, "question_to_user is required when needs_user_input=true")
	}
	options, errResult := parseOptionsArg(call)
	if errResult != nil {
		return errResult, nil
	}
	if len(options) > 0 && !needsUserInput {
		return shared.ToolError(call.ID, "options requires needs_user_input=true")
	}

	metadata := map[string]any{
		"action":       "clarify",
		"run_id":       runID,
		"branch_id":    branchID,
		"task_id":      taskID,
		"task_goal_ui": taskGoalUI,
	}
	if len(successCriteria) > 0 {
		metadata["success_criteria"] = append([]string(nil), successCriteria...)
	}
	if needsUserInput {
		metadata["needs_user_input"] = true
		metadata["question_to_user"] = questionToUser
		if len(options) > 0 {
			metadata["options"] = append([]string(nil), options...)
		}
	}

	content := taskGoalUI
	if needsUserInput {
		content = content + "\n" + questionToUser
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}, nil
}

func (t *uiAskUser) executeRequest(call ports.ToolCall) (*ports.ToolResult, error) {
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

	content := message
	if title != "" {
		content = title + "\n" + message
	}

	metadata := map[string]any{
		"action":           "request",
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
