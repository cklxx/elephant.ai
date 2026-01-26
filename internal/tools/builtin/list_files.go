package builtin

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"context"
	"fmt"
	"os"
	"strings"
	"alex/internal/tools/builtin/shared"
	"alex/internal/tools/builtin/pathutil"
)

type listFiles struct {
}

func NewListFiles(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &listFiles{}
}

func (t *listFiles) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		path = "."
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	var result strings.Builder
	for _, entry := range entries {
		info, _ := entry.Info()
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s\n", entry.Name()))
		} else {
			result.WriteString(fmt.Sprintf("[FILE] %s (%d bytes)\n", entry.Name(), info.Size()))
		}
	}
	return &ports.ToolResult{CallID: call.ID, Content: result.String()}, nil
}

func (t *listFiles) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "list_files",
		Description: "List files and directories",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path": {Type: "string", Description: "Directory path (default: .)"},
			},
		},
	}
}

func (t *listFiles) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "list_files", Version: "1.0.0", Category: "file_operations",
	}
}
