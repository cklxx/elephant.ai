package search

import (
	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type find struct {
	shared.BaseTool
}

func NewFind(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &find{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "find",
				Description: "Find file/directory paths by name or pattern. Use for path discovery/filtering before opening content; not for inside-file text search.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"name": {
							Type:        "string",
							Description: "The name pattern to search for (supports wildcards like *.go)",
						},
						"path": {
							Type:        "string",
							Description: "Path to search in (default: current directory)",
						},
						"type": {
							Type:        "string",
							Description: "Type of files to find: 'f' for files, 'd' for directories",
							Enum:        []any{"f", "d"},
						},
						"max_depth": {
							Type:        "number",
							Description: "Maximum depth to search (default: 10)",
						},
					},
					Required: []string{"name"},
				},
			},
			ports.ToolMetadata{
				Name:        "find",
				Version:     "1.0.0",
				Category:    "search",
				Tags:        []string{"filesystem", "search", "files", "path", "discovery", "directory"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
	}
}

func (t *find) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	name, ok := call.Arguments["name"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'name'")}, nil
	}

	path := "."
	if p, ok := call.Arguments["path"].(string); ok && p != "" {
		path = p
	}

	resolvedPath, err := pathutil.SanitizePathWithinBase(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	maxDepth := 10
	if md, ok := call.Arguments["max_depth"].(float64); ok {
		maxDepth = int(md)
	}
	if maxDepth < 0 {
		maxDepth = 0
	}

	fileType, _ := call.Arguments["type"].(string)
	results, err := t.walkMatches(resolvedPath, name, fileType, maxDepth)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if len(results) == 0 {
		return t.noMatchesResult(call, name, path, maxDepth)
	}

	truncated := false
	if len(results) > 100 {
		results = results[:100]
		truncated = true
	}

	content := fmt.Sprintf("Found %d matches", len(results))
	if truncated {
		content += " (showing first 100)"
	}
	content += ":\n" + strings.Join(results, "\n")

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"pattern":       name,
			"path":          path,
			"resolved_path": resolvedPath,
			"matches":       len(results),
			"max_depth":     maxDepth,
			"truncated":     truncated,
			"results":       results,
		},
	}, nil
}

func (t *find) noMatchesResult(call ports.ToolCall, name, path string, maxDepth int) (*ports.ToolResult, error) {
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "No matches found",
		Metadata: map[string]any{
			"pattern":   name,
			"path":      path,
			"matches":   0,
			"max_depth": maxDepth,
		},
	}, nil
}

func (t *find) walkMatches(root, pattern, fileType string, maxDepth int) ([]string, error) {
	results := make([]string, 0, 32)
	walkFn := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		if depth > maxDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if rel == "." {
			return nil
		}
		switch fileType {
		case "f":
			if entry.IsDir() {
				return nil
			}
		case "d":
			if !entry.IsDir() {
				return nil
			}
		}

		match, matchErr := filepath.Match(pattern, entry.Name())
		if matchErr != nil {
			return matchErr
		}
		if match {
			results = append(results, rel)
		}
		return nil
	}

	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, err
	}
	return results, nil
}
