package llm

import (
	"regexp"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
)

var validToolNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func isValidToolName(name string) bool {
	return validToolNamePattern.MatchString(strings.TrimSpace(name))
}

func normalizeToolSchema(schema ports.ParameterSchema) ports.ParameterSchema {
	normalized := schema
	if strings.TrimSpace(normalized.Type) == "" {
		normalized.Type = "object"
	}
	if normalized.Properties == nil {
		normalized.Properties = map[string]ports.Property{}
	}
	return normalized
}

func buildToolCallHistory(calls []ports.ToolCall) []map[string]any {
	result := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		if !isValidToolName(call.Name) {
			continue
		}
		args := "{}"
		if len(call.Arguments) > 0 {
			if data, err := jsonx.Marshal(call.Arguments); err == nil {
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
		schema := normalizeToolSchema(tool.Parameters)
		// alex_material_capabilities is for renderers; it is not an OpenAI tool field.
		entry := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  schema,
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
		schema := normalizeToolSchema(tool.Parameters)
		entry := map[string]any{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  schema,
		}
		result = append(result, entry)
	}
	return result
}
