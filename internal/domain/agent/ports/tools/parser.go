package tools

import "alex/internal/agent/ports"

// FunctionCallParser extracts tool calls from LLM responses
type FunctionCallParser interface {
	// Parse extracts tool calls from content
	Parse(content string) ([]ports.ToolCall, error)

	// Validate checks if tool calls are valid
	Validate(call ports.ToolCall, definition ports.ToolDefinition) error
}
