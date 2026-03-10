package mocks

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// ToolScenario represents a complete tool calling scenario for testing
type ToolScenario struct {
	Name        string
	Description string
	LLM         *MockLLMClient
	Registry    *MockToolRegistry
}

func scenarioToolRegistry(inner func(name string) (tools.ToolExecutor, error)) *MockToolRegistry {
	return &MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			switch name {
			case "plan":
				return newUIPlanExecutor(), nil
			case "ask_user":
				return newUIAskUserExecutor(), nil
			default:
				if inner == nil {
					return nil, fmt.Errorf("tool not found: %s", name)
				}
				return inner(name)
			}
		},
	}
}

func newUIPlanExecutor() tools.ToolExecutor {
	return &MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			runID, _ := call.Arguments["run_id"].(string)
			runID = strings.TrimSpace(runID)
			if runID == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "run_id cannot be empty", Error: fmt.Errorf("run_id cannot be empty")}, nil
			}

			goal, _ := call.Arguments["overall_goal_ui"].(string)
			goal = strings.TrimSpace(goal)
			if goal == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "overall_goal_ui cannot be empty", Error: fmt.Errorf("invalid overall_goal_ui")}, nil
			}

			complexity, _ := call.Arguments["complexity"].(string)
			complexity = strings.ToLower(strings.TrimSpace(complexity))
			if complexity != "simple" && complexity != "complex" {
				return &ports.ToolResult{CallID: call.ID, Content: "complexity must be simple or complex", Error: fmt.Errorf("invalid complexity")}, nil
			}
			if complexity == "simple" && strings.ContainsAny(goal, "\r\n") {
				return &ports.ToolResult{CallID: call.ID, Content: "overall_goal_ui must be single-line when complexity=simple", Error: fmt.Errorf("invalid overall_goal_ui")}, nil
			}

			metadata := map[string]any{
				"run_id":          runID,
				"overall_goal_ui": goal,
				"complexity":      complexity,
			}
			if internalPlan, ok := call.Arguments["internal_plan"]; ok {
				metadata["internal_plan"] = internalPlan
			}

			return &ports.ToolResult{CallID: call.ID, Content: goal, Metadata: metadata}, nil
		},
	}
}

func newUIAskUserExecutor() tools.ToolExecutor {
	return &MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			runID, _ := call.Arguments["run_id"].(string)
			runID = strings.TrimSpace(runID)
			if runID == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "run_id cannot be empty", Error: fmt.Errorf("run_id cannot be empty")}, nil
			}

			taskID, _ := call.Arguments["task_id"].(string)
			taskID = strings.TrimSpace(taskID)
			if taskID == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "task_id cannot be empty", Error: fmt.Errorf("task_id cannot be empty")}, nil
			}

			taskGoalUI, _ := call.Arguments["task_goal_ui"].(string)
			taskGoalUI = strings.TrimSpace(taskGoalUI)
			if taskGoalUI == "" {
				return &ports.ToolResult{CallID: call.ID, Content: "task_goal_ui cannot be empty", Error: fmt.Errorf("invalid task_goal_ui")}, nil
			}

			metadata := map[string]any{
				"run_id":       runID,
				"task_id":      taskID,
				"task_goal_ui": taskGoalUI,
			}

			if raw, ok := call.Arguments["success_criteria"]; ok {
				if arr, ok := raw.([]any); ok {
					var criteria []string
					for _, item := range arr {
						if text, ok := item.(string); ok {
							if trimmed := strings.TrimSpace(text); trimmed != "" {
								criteria = append(criteria, trimmed)
							}
						}
					}
					if len(criteria) > 0 {
						metadata["success_criteria"] = criteria
					}
				}
			}

			needsUserInput := false
			if raw, ok := call.Arguments["needs_user_input"]; ok {
				if value, ok := raw.(bool); ok {
					needsUserInput = value
				}
			}
			questionToUser := ""
			if raw, ok := call.Arguments["question_to_user"]; ok {
				if value, ok := raw.(string); ok {
					questionToUser = strings.TrimSpace(value)
				}
			}
			if needsUserInput {
				if questionToUser == "" {
					return &ports.ToolResult{CallID: call.ID, Content: "question_to_user is required when needs_user_input=true", Error: fmt.Errorf("missing question_to_user")}, nil
				}
				metadata["needs_user_input"] = true
				metadata["question_to_user"] = questionToUser
			}

			content := taskGoalUI
			if needsUserInput {
				content = content + "\n" + questionToUser
			}

			return &ports.ToolResult{CallID: call.ID, Content: strings.TrimSpace(content), Metadata: metadata}, nil
		},
	}
}

func newScriptedLLMClient(responses ...ports.CompletionResponse) *MockLLMClient {
	if len(responses) == 0 {
		panic("newScriptedLLMClient requires at least one completion response")
	}

	callCount := 0
	return &MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++

			idx := callCount - 1
			if idx >= len(responses) {
				idx = len(responses) - 1
			}

			return cloneCompletionResponse(responses[idx]), nil
		},
	}
}

func cloneCompletionResponse(template ports.CompletionResponse) *ports.CompletionResponse {
	clone := template

	if template.ToolCalls != nil {
		clone.ToolCalls = make([]ports.ToolCall, len(template.ToolCalls))
		for i, call := range template.ToolCalls {
			clone.ToolCalls[i] = cloneToolCall(call)
		}
	}

	if template.Metadata != nil {
		clone.Metadata = ports.CloneAnyMap(template.Metadata)
	}

	return &clone
}

func cloneToolCall(template ports.ToolCall) ports.ToolCall {
	clone := template
	if template.Arguments != nil {
		clone.Arguments = ports.CloneAnyMap(template.Arguments)
	}
	return clone
}

const (
	scenarioRunID  = "test-run"
	scenarioTaskID = "task-1"
)

func newToolCallCompletion(content string, call ports.ToolCall, usage ports.TokenUsage) ports.CompletionResponse {
	return ports.CompletionResponse{
		Content:    content,
		ToolCalls:  []ports.ToolCall{call},
		StopReason: "tool_calls",
		Usage:      usage,
	}
}

func newPlanCompletion(
	content string,
	overallGoalUI string,
	complexity string,
	usage ports.TokenUsage,
	internalPlan map[string]any,
) ports.CompletionResponse {
	args := map[string]any{
		"run_id":          scenarioRunID,
		"overall_goal_ui": overallGoalUI,
		"complexity":      complexity,
	}
	if internalPlan != nil {
		args["internal_plan"] = internalPlan
	}

	return newToolCallCompletion(content, ports.ToolCall{
		ID:        "call_plan",
		Name:      "plan",
		Arguments: args,
	}, usage)
}

func newAskUserCompletion(
	content string,
	taskGoalUI string,
	usage ports.TokenUsage,
	extraArgs map[string]any,
) ports.CompletionResponse {
	args := map[string]any{
		"run_id":       scenarioRunID,
		"task_id":      scenarioTaskID,
		"task_goal_ui": taskGoalUI,
	}
	for key, value := range extraArgs {
		args[key] = value
	}

	return newToolCallCompletion(content, ports.ToolCall{
		ID:        "call_ask_user",
		Name:      "ask_user",
		Arguments: args,
	}, usage)
}

func newFinalCompletion(content string, usage ports.TokenUsage) ports.CompletionResponse {
	return ports.CompletionResponse{
		Content:    content,
		StopReason: "stop",
		Usage:      usage,
	}
}
