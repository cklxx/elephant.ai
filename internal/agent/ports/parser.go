package ports

// FunctionCallParser extracts tool calls from LLM responses
type FunctionCallParser interface {
	// Parse extracts tool calls from content
	Parse(content string) ([]ToolCall, error)

	// Validate checks if tool calls are valid
	Validate(call ToolCall, definition ToolDefinition) error
}
