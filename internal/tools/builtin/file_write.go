package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os"
)

type fileWrite struct{}

func NewFileWrite() ports.ToolExecutor {
	return &fileWrite{}
}

func (t *fileWrite) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'path'")}, nil
	}

	content, ok := call.Arguments["content"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'content'")}, nil
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
	}, nil
}

func (t *fileWrite) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "file_write",
		Description: "Write content to file",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path":    {Type: "string", Description: "File path"},
				"content": {Type: "string", Description: "Content to write"},
			},
			Required: []string{"path", "content"},
		},
	}
}

func (t *fileWrite) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "file_write", Version: "1.0.0", Category: "file_operations", Dangerous: true,
	}
}
