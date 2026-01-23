package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type listFiles struct {
}

func NewListFiles(cfg FileToolConfig) ports.ToolExecutor {
	_ = cfg
	return &listFiles{}
}

func (t *listFiles) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		path = "."
	}

	resolver := GetPathResolverFromContext(ctx)
	base := resolver.ResolvePath(".")
	baseAbs, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to resolve base path: %w", err)}, nil
	}
	candidate := resolver.ResolvePath(path)
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to resolve path: %w", err)}, nil
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to resolve path within base: %w", err)}, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("path must stay within the working directory")}, nil
	}
	safe := filepath.Join(baseAbs, rel)
	if !pathWithinBase(baseAbs, safe) {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("path must stay within the working directory")}, nil
	}

	entries, err := os.ReadDir(safe)
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
