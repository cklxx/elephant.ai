package ui

import (
	"context"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

type uiClarify struct {
	shared.BaseTool
}

func NewClarify() tools.ToolExecutor {
	return &uiClarify{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "clarify",
				Description: `Optional UI tool: emit a task sub-header. Use needs_user_input=true to pause and ask the user a question.

Rules:
- Use to declare a sub-task header before starting a unit of work.
- When needs_user_input=true, provide question_to_user and the orchestrator will pause for user input.
- When waiting for user input, provide options to let channels render a selection UI.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
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
							Description: "User-facing task header.",
						},
						"success_criteria": {
							Type:        "array",
							Description: "Optional string array of success criteria (UI hint).",
							Items:       &ports.Property{Type: "string"},
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
							Description: "Optional selectable options shown to the user for question_to_user.",
							Items:       &ports.Property{Type: "string"},
						},
					},
					Required: []string{"task_goal_ui"},
				},
			},
			ports.ToolMetadata{
				Name:     "clarify",
				Version:  "1.0.0",
				Category: "ui",
				Tags:     []string{"ui", "orchestration"},
			},
		),
	}
}

func (t *uiClarify) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Source run_id from context; ignore any LLM-supplied value.
	runID := strings.TrimSpace(id.RunIDFromContext(ctx))

	// task_id is optional; auto-generate from call.ID if not provided.
	taskID := shared.StringArg(call.Arguments, "task_id")
	if strings.TrimSpace(taskID) == "" {
		taskID = "task-" + call.ID
	}

	taskGoalUI, errResult := shared.RequireStringArg(call.Arguments, call.ID, "task_goal_ui")
	if errResult != nil {
		return errResult, nil
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

	// Reject unexpected parameters to keep the protocol strict.
	for key := range call.Arguments {
		switch key {
		case "run_id", "branch_id", "task_id", "task_goal_ui", "success_criteria", "needs_user_input", "question_to_user", "options":
			// run_id accepted but ignored (sourced from context).
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	metadata := map[string]any{
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
