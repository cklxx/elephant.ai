package search

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
	"context"
	"fmt"
	"regexp"
	"strings"
)

type grep struct {
}

func NewGrep(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &grep{}
}

func (t *grep) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	pattern, ok := call.Arguments["pattern"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'pattern'")}, nil
	}

	path, ok := call.Arguments["path"].(string)
	if !ok {
		path = "."
	}

	resolvedPath, err := pathutil.SanitizePathWithinBase(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	matches, total, err := searchTextMatches(resolvedPath, re, nil, 0)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: formatSearchContent(matches, total),
		Metadata: map[string]any{
			"path":          path,
			"resolved_path": resolvedPath,
			"matches":       total,
		},
	}, nil
}

func (t *grep) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "grep",
		Description: "Search for pattern in files",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"pattern": {Type: "string", Description: "Search pattern"},
				"path":    {Type: "string", Description: "Path to search (default: .)"},
			},
			Required: []string{"pattern"},
		},
	}
}

func (t *grep) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name: "grep", Version: "1.0.0", Category: "search",
	}
}

func formatSearchContent(lines []string, total int) string {
	if total == 0 {
		return "No matches found"
	}
	return strings.Join(lines, "\n")
}
