package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	id "alex/internal/utils/id"
)

type uiClarify struct{}

func NewClarify() tools.ToolExecutor {
	return &uiClarify{}
}

func (t *uiClarify) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "clarify",
		Version:  "1.0.0",
		Category: "ui",
		Tags:     []string{"ui", "orchestration"},
	}
}

func (t *uiClarify) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "clarify",
		Description: `UI tool: emit Level 2 task header before starting a unit of work.

Rules:
- Required when plan(complexity="complex"): must be called once per task (task_id) before the task's first action tool call.
- Optional when plan(complexity="simple"): only call it if you need to pause and ask the user for missing input.
- When needs_user_input=true, provide question_to_user and the orchestrator should pause.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"run_id": {
					Type:        "string",
					Description: "Run identifier (must match the current run_id provided by the system).",
				},
				"branch_id": {
					Type:        "string",
					Description: "Optional branch identifier.",
				},
				"task_id": {
					Type:        "string",
					Description: "Task identifier within the run.",
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
			},
			Required: []string{"run_id", "task_id", "task_goal_ui"},
		},
	}
}

func (t *uiClarify) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	runID, ok := call.Arguments["run_id"].(string)
	if !ok {
		err := fmt.Errorf("missing 'run_id'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		err := fmt.Errorf("run_id cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	expected := strings.TrimSpace(id.TaskIDFromContext(ctx))
	if expected != "" && runID != expected {
		err := errors.New("run_id does not match the active task")
		return &ports.ToolResult{
			CallID: call.ID,
			Content: "Request does not match the active task. Please retry " +
				"from the latest conversation turn.",
			Error: err,
		}, nil
	}

	taskID, ok := call.Arguments["task_id"].(string)
	if !ok {
		err := fmt.Errorf("missing 'task_id'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		err := fmt.Errorf("task_id cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	taskGoalUI, ok := call.Arguments["task_goal_ui"].(string)
	if !ok {
		err := fmt.Errorf("missing 'task_goal_ui'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	taskGoalUI = strings.TrimSpace(taskGoalUI)
	if taskGoalUI == "" {
		err := fmt.Errorf("task_goal_ui cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	branchID := ""
	if raw, exists := call.Arguments["branch_id"]; exists {
		if value, ok := raw.(string); ok {
			branchID = strings.TrimSpace(value)
		} else if raw != nil {
			err := fmt.Errorf("branch_id must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	var successCriteria []string
	if raw, exists := call.Arguments["success_criteria"]; exists {
		arr, ok := raw.([]any)
		if !ok {
			err := fmt.Errorf("success_criteria must be an array of strings")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
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
			err := fmt.Errorf("needs_user_input must be a boolean")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		needsUserInput = value
	}

	questionToUser := ""
	if raw, exists := call.Arguments["question_to_user"]; exists {
		if value, ok := raw.(string); ok {
			questionToUser = strings.TrimSpace(value)
		} else if raw != nil {
			err := fmt.Errorf("question_to_user must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}
	if needsUserInput && questionToUser == "" {
		err := fmt.Errorf("question_to_user is required when needs_user_input=true")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	// Reject unexpected parameters to keep the protocol strict.
	for key := range call.Arguments {
		switch key {
		case "run_id", "branch_id", "task_id", "task_goal_ui", "success_criteria", "needs_user_input", "question_to_user":
		default:
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
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
