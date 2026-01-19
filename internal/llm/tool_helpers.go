package llm

import (
	"encoding/json"
	"regexp"
	"strings"

	"alex/internal/agent/ports"
)

var validToolNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func isValidToolName(name string) bool {
	return validToolNamePattern.MatchString(strings.TrimSpace(name))
}

func buildToolCallHistory(calls []ports.ToolCall) []map[string]any {
	result := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		if !isValidToolName(call.Name) {
			continue
		}
		args := "{}"
		if len(call.Arguments) > 0 {
			if data, err := json.Marshal(call.Arguments); err == nil {
				args = string(data)
			}
		}

		result = append(result, map[string]any{
			"id":   call.ID,
			"type": "function",
			"function": map[string]any{
				"name":      call.Name,
				"arguments": args,
			},
		})
	}
	return result
}

func convertTools(tools []ports.ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if !isValidToolName(tool.Name) {
			continue
		}
		// alex_material_capabilities is for renderers; it is not an OpenAI tool field.
		entry := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		}
		result = append(result, entry)
	}
	return result
}

func convertCodexTools(tools []ports.ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if !isValidToolName(tool.Name) {
			continue
		}
		entry := map[string]any{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		}
		result = append(result, entry)
	}
	return result
}
