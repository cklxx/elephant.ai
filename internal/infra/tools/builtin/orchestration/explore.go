package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type explore struct {
	shared.BaseTool
	subagent tools.ToolExecutor
}

// NewExplore creates an explore tool that wraps the subagent executor.
func NewExplore(subagent tools.ToolExecutor) tools.ToolExecutor {
	return &explore{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "explore",
				Description: "Plan and delegate multi-scope investigations while orchestrating the platform's complete exploration toolset. Automatically prepares local, web, and custom subtasks and synthesizes a concise summary of findings.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"objective": {
							Type:        "string",
							Description: "High-level goal to investigate.",
						},
						"local_scope": {
							Type:        "array",
							Description: "Specific local/codebase areas to inspect.",
							Items:       &ports.Property{Type: "string"},
						},
						"web_scope": {
							Type:        "array",
							Description: "Web research focus areas.",
							Items:       &ports.Property{Type: "string"},
						},
						"custom_tasks": {
							Type:        "array",
							Description: "Additional custom subtasks to run.",
							Items:       &ports.Property{Type: "string"},
						},
						"notes": {
							Type:        "string",
							Description: "Context or constraints shared with every subtask.",
						},
						"mode": {
							Type:        "string",
							Description: "Delegation mode forwarded to subagent (parallel or serial).",
							Enum:        []any{"parallel", "serial"},
						},
					},
					Required: []string{"objective"},
				},
			},
			ports.ToolMetadata{
				Name:     "explore",
				Version:  "1.0.0",
				Category: "orchestration",
				Tags:     []string{"planning", "delegation", "discovery"},
			},
		),
		subagent: subagent,
	}
}

func (e *explore) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if e.subagent == nil {
		return shared.ToolError(call.ID, "explore tool is unavailable: subagent not registered")
	}

	objective, errResult := shared.RequireStringArg(call.Arguments, call.ID, "objective")
	if errResult != nil {
		return errResult, nil
	}

	localScope, err := parseStringList(call.Arguments, "local_scope")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	webScope, err := parseStringList(call.Arguments, "web_scope")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	customTasks, err := parseStringList(call.Arguments, "custom_tasks")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	notes := ""
	if raw, exists := call.Arguments["notes"]; exists {
		noteStr, ok := raw.(string)
		if !ok {
			return shared.ToolError(call.ID, "notes must be a string when provided")
		}
		notes = strings.TrimSpace(noteStr)
	}

	subtasks := buildExploreSubtasks(objective, localScope, webScope, customTasks, notes)
	if len(subtasks) == 0 {
		base := fmt.Sprintf("[CUSTOM] %s", objective)
		subtasks = []string{appendNotes(base, notes)}
	}

	prompt := buildExplorePrompt(objective, subtasks)

	mode, err := parseSubagentMode(call.Arguments, len(subtasks))
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	delegationCall := ports.ToolCall{
		ID:   call.ID + ":explore",
		Name: "subagent",
		Arguments: func() map[string]any {
			args := map[string]any{"tasks": subtasks}
			if mode != "" {
				args["mode"] = mode
			}
			return args
		}(),
	}

	delegateResult, execErr := e.subagent.Execute(ctx, delegationCall)
	if execErr != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("subagent execution failed: %v", execErr), Error: execErr}, nil
	}

	delegationMetadata := cloneMetadata(delegateResult.Metadata)
	delegationDetails := map[string]any{
		"call": map[string]any{
			"tool":      delegationCall.Name,
			"arguments": delegationCall.Arguments,
		},
		"result_metadata": delegationMetadata,
		"result_content":  delegateResult.Content,
	}
	content := fmt.Sprintf("Delegated objective \"%s\" to subagent.\n\nPrompt:\n%s\n\nSubagent output:\n%s", objective, prompt, delegateResult.Content)

	resultMetadata := map[string]any{
		"objective":    objective,
		"local_scope":  localScope,
		"web_scope":    webScope,
		"custom_tasks": customTasks,
		"notes":        notes,
		"prompt":       prompt,
		"delegation":   delegationDetails,
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: resultMetadata,
	}, nil
}

func buildExploreSubtasks(objective string, localScope, webScope, customTasks []string, notes string) []string {
	var subtasks []string

	for _, focus := range localScope {
		base := fmt.Sprintf("[LOCAL] %s — Focus on %s.", objective, focus)
		subtasks = append(subtasks, appendNotes(base, notes))
	}
	for _, focus := range webScope {
		base := fmt.Sprintf("[WEB] %s — Research %s.", objective, focus)
		subtasks = append(subtasks, appendNotes(base, notes))
	}
	for _, task := range customTasks {
		base := fmt.Sprintf("[CUSTOM] %s", task)
		subtasks = append(subtasks, appendNotes(base, notes))
	}

	if len(subtasks) == 0 {
		return subtasks
	}

	return subtasks
}

func buildExplorePrompt(objective string, subtasks []string) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(objective))
	builder.WriteString("\n\nTasks:\n")
	for i, task := range subtasks {
		builder.WriteString(fmt.Sprintf("%d) %s\n", i+1, task))
	}

	return strings.TrimSpace(builder.String())
}

func appendNotes(base, notes string) string {
	if strings.TrimSpace(notes) == "" {
		return strings.TrimSpace(base)
	}

	trimmed := strings.TrimSpace(base)
	if strings.HasSuffix(trimmed, ".") {
		return fmt.Sprintf("%s Notes: %s", trimmed, notes)
	}
	return fmt.Sprintf("%s. Notes: %s", trimmed, notes)
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for k, v := range metadata {
		cloned[k] = v
	}
	return cloned
}
