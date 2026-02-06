package fileops

import (
	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
	"context"
	"fmt"
	"os"
	"strings"
)

type listFiles struct {
	shared.BaseTool
}

func NewListFiles(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &listFiles{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "list_files",
				Description: "List files and directories",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path": {Type: "string", Description: "Directory path (default: .)"},
					},
				},
			},
			ports.ToolMetadata{
				Name: "list_files", Version: "1.0.0", Category: "file_operations", SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
	}
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
