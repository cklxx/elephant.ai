package fileops

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
	"context"
	"fmt"
	"os"
)

type fileRead struct {
	shared.BaseTool
}

func NewFileRead(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &fileRead{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "file_read",
				Description: "Read file contents",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path": {Type: "string", Description: "File path"},
					},
					Required: []string{"path"},
				},
			},
			ports.ToolMetadata{
				Name: "file_read", Version: "1.0.0", Category: "file_operations",
			},
		),
	}
}

func (t *fileRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'path'")}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: string(content)}, nil
}

