package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindTool implements find functionality
type FindTool struct{}

func CreateFindTool() *FindTool {
	return &FindTool{}
}

func (t *FindTool) Name() string {
	return "find"
}

func (t *FindTool) Description() string {
	return "Find files and directories by name or pattern using the find command. Limits results to maximum 100 matches."
}

func (t *FindTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name pattern to search for (supports wildcards)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to search in",
				"default":     ".",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Type of files to find: 'f' for files, 'd' for directories",
				"enum":        []string{"f", "d"},
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum depth to search",
				"default":     10,
			},
		},
		"required": []string{"name"},
	}
}

func (t *FindTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("name", "Name pattern to search for").
		AddOptionalStringField("path", "Path to search in").
		AddOptionalStringField("type", "File type filter").
		AddOptionalIntField("max_depth", "Maximum depth", 1, 20)

	return validator.Validate(args)
}

func (t *FindTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	name := args["name"].(string)

	path := "."
	if p, ok := args["path"]; ok {
		path = p.(string)
	}

	maxDepth := 10
	if md, ok := args["max_depth"]; ok {
		if mdFloat, ok := md.(float64); ok {
			maxDepth = int(mdFloat)
		}
	}

	// Build find command
	cmdArgs := []string{path}

	// Add max depth
	cmdArgs = append(cmdArgs, "-maxdepth", fmt.Sprintf("%d", maxDepth))

	// Add type filter if specified
	if fileType, ok := args["type"].(string); ok && fileType != "" {
		cmdArgs = append(cmdArgs, "-type", fileType)
	}

	// Add name pattern
	cmdArgs = append(cmdArgs, "-name", name)

	// Execute find command
	cmd := exec.CommandContext(ctx, "find", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		// find returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &ToolResult{
				Content: "No matches found",
				Data: map[string]interface{}{
					"pattern":   name,
					"path":      path,
					"matches":   0,
					"max_depth": maxDepth,
					"type":      args["type"],
				},
			}, nil
		}
		return nil, fmt.Errorf("find command failed: %w", err)
	}

	// Process output
	lines := strings.Split(string(output), "\n")
	// Remove empty lines
	var results []string
	for _, line := range lines {
		if line != "" {
			// Convert to relative path if possible
			if relPath, err := filepath.Rel(path, line); err == nil && !strings.HasPrefix(relPath, "..") {
				results = append(results, relPath)
			} else {
				results = append(results, line)
			}
		}
	}

	// Limit results to 100 matches
	if len(results) > 100 {
		results = results[:100]
	}

	return &ToolResult{
		Content: fmt.Sprintf("Found %d matches:\n%s", len(results), strings.Join(results, "\n")),
		Data: map[string]interface{}{
			"pattern":   name,
			"path":      path,
			"matches":   len(results),
			"max_depth": maxDepth,
			"type":      args["type"],
			"results":   results,
			"content":   fmt.Sprintf("Found %d matches:\n%s", len(results), strings.Join(results, "\n")),
		},
	}, nil
}
