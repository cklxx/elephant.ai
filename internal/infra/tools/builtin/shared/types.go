package shared

import "context"

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Content  string                 `json:"content"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Files    []string               `json:"files,omitempty"` // Files that were modified/created
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Tool interface that all builtin tools must implement
type Tool interface {
	// Name returns the unique name of the tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// Parameters returns the JSON schema for the tool's parameters
	Parameters() map[string]interface{}

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)

	// Validate checks if the provided arguments are valid for this tool
	Validate(args map[string]interface{}) error
}
