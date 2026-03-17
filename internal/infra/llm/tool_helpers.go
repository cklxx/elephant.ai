package llm

import (
	"regexp"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
	"alex/internal/shared/utils"
)

var validToolNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func isValidToolName(name string) bool {
	return validToolNamePattern.MatchString(strings.TrimSpace(name))
}

func normalizeToolSchema(schema ports.ParameterSchema) ports.ParameterSchema {
	normalized := schema
	if utils.IsBlank(normalized.Type) {
		normalized.Type = "object"
	}
	if normalized.Properties == nil {
		normalized.Properties = map[string]ports.Property{}
	}
	for name, prop := range normalized.Properties {
		// Ensure array properties always have items — providers like Codex reject
		// array schemas without an items field.
		if prop.Type == "array" && prop.Items == nil {
			prop.Items = &ports.Property{Type: "string"}
		}
		// Sanitize items: Anthropic rejects empty description strings and
		// object items without a properties field.
		if prop.Items != nil {
			sanitizePropertyItems(prop.Items)
		}
		normalized.Properties[name] = prop
	}
	return normalized
}

// sanitizePropertyItems cleans up a nested Property to satisfy strict
// provider schema validation (e.g. Anthropic rejects empty descriptions
// and object items without properties).
func sanitizePropertyItems(p *ports.Property) {
	if p == nil {
		return
	}
	// Empty description causes Anthropic invalid_request_error.
	if p.Description == "" && p.Type != "" {
		p.Description = "item"
	}
	// Object items must declare properties; fall back to string when none
	// are available (Property struct cannot express nested properties).
	if p.Type == "object" {
		p.Type = "string"
		if p.Description == "item" {
			p.Description = "JSON object as string"
		}
	}
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

// toolFormat controls whether tool definitions are nested under a "function"
// key (OpenAI chat format) or flattened (Codex/Responses format).
type toolFormat int

const (
	toolFormatChat      toolFormat = iota // {"type":"function","function":{...}}
	toolFormatCodex                       // {"type":"function","name":...,"parameters":...}
	toolFormatAnthropic                   // {"name":...,"description":...,"input_schema":...}
)

func convertToolsWithFormat(tools []ports.ToolDefinition, format toolFormat) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if !isValidToolName(tool.Name) {
			continue
		}
		schema := normalizeToolSchema(tool.Parameters)
		var entry map[string]any
		switch format {
		case toolFormatCodex:
			entry = map[string]any{
				"type":        "function",
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  schema,
			}
		case toolFormatAnthropic:
			entry = map[string]any{
				"name":         tool.Name,
				"description":  tool.Description,
				"input_schema": schema,
			}
		default:
			entry = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  schema,
				},
			}
		}
		result = append(result, entry)
	}
	return result
}

func convertTools(tools []ports.ToolDefinition) []map[string]any {
	return convertToolsWithFormat(tools, toolFormatChat)
}

func convertCodexTools(tools []ports.ToolDefinition) []map[string]any {
	return convertToolsWithFormat(tools, toolFormatCodex)
}

func convertAnthropicToolsDef(tools []ports.ToolDefinition) []map[string]any {
	return convertToolsWithFormat(tools, toolFormatAnthropic)
}
