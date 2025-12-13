package builtin

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
)

type explore struct {
	subagent ports.ToolExecutor
}

// NewExplore creates an explore tool that wraps the subagent executor.
func NewExplore(subagent ports.ToolExecutor) ports.ToolExecutor {
	return &explore{subagent: subagent}
}

func (e *explore) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
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
				},
				"web_scope": {
					Type:        "array",
					Description: "Web research focus areas.",
				},
				"custom_tasks": {
					Type:        "array",
					Description: "Additional custom subtasks to run.",
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
	}
}

func (e *explore) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "explore",
		Version:  "1.0.0",
		Category: "orchestration",
		Tags:     []string{"planning", "delegation", "discovery"},
	}
}

func (e *explore) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if e.subagent == nil {
		err := fmt.Errorf("explore tool is unavailable: subagent not registered")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	objectiveVal, ok := call.Arguments["objective"].(string)
	if !ok || strings.TrimSpace(objectiveVal) == "" {
		err := fmt.Errorf("objective must be a non-empty string")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	objective := strings.TrimSpace(objectiveVal)

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
			err := fmt.Errorf("notes must be a string when provided")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		notes = strings.TrimSpace(noteStr)
	}

	prompt := buildExplorePrompt(objective, localScope, webScope, customTasks, notes)

	delegationCall := ports.ToolCall{
		ID:        call.ID + ":explore",
		Name:      "subagent",
		Arguments: map[string]any{"prompt": prompt},
	}

	delegateResult, execErr := e.subagent.Execute(ctx, delegationCall)
	if execErr != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("subagent execution failed: %v", execErr), Error: execErr}, nil
	}

	delegationMetadata := cloneMetadata(delegateResult.Metadata)
	delegationDetails := map[string]any{
		"call": map[string]any{
			"tool":      delegationCall.Name,
			"arguments": map[string]any{"prompt": prompt},
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

func parseStringList(args map[string]any, key string) ([]string, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return result, nil
	case []any:
		result := make([]string, 0, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be a string", key, i)
			}
			if trimmed := strings.TrimSpace(str); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings when provided", key)
	}
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

func buildExplorePrompt(objective string, localScope, webScope, customTasks []string, notes string) string {
	subtasks := buildExploreSubtasks(objective, localScope, webScope, customTasks, notes)
	if len(subtasks) == 0 {
		base := fmt.Sprintf("[CUSTOM] %s", objective)
		subtasks = []string{appendNotes(base, notes)}
	}

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
