package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileRead struct {
}

func NewFileRead(cfg FileToolConfig) ports.ToolExecutor {
	_ = cfg
	return &fileRead{}
}

func (t *fileRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, ok := call.Arguments["path"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'path'")}, nil
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

	content, err := os.ReadFile(safe)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: string(content)}, nil
}

func (t *fileRead) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "file_read",
		Description: "Read file contents",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path": {Type: "string", Description: "File path"},
			},
			Required: []string{"path"},
		},
	}
}

func (t *fileRead) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "file_read", Version: "1.0.0", Category: "file_operations",
	}
}
