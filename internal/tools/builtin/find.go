package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type find struct{}

func NewFind() ports.ToolExecutor {
	return &find{}
}

func (t *find) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Extract parameters
	name, ok := call.Arguments["name"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'name'")}, nil
	}

	// Optional parameters with defaults
	path := "."
	if p, ok := call.Arguments["path"].(string); ok && p != "" {
		path = p
	}

	maxDepth := 10
	if md, ok := call.Arguments["max_depth"].(float64); ok {
		maxDepth = int(md)
	}

	// Build find command
	cmdArgs := []string{path}

	// Add max depth
	cmdArgs = append(cmdArgs, "-maxdepth", fmt.Sprintf("%d", maxDepth))

	// Add type filter if specified
	if fileType, ok := call.Arguments["type"].(string); ok && fileType != "" {
		cmdArgs = append(cmdArgs, "-type", fileType)
	}

	// Add name pattern
	cmdArgs = append(cmdArgs, "-name", name)

	// Execute find command
	cmd := exec.CommandContext(ctx, "find", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// find returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
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
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: string(output),
			Error:   fmt.Errorf("find command failed: %w", err),
		}, nil
	}

	// Process output
	lines := strings.Split(string(output), "\n")
	// Remove empty lines and convert to relative paths
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

	// Handle no matches case
	if len(results) == 0 {
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

	// Limit results to 100 matches
	truncated := false
	if len(results) > 100 {
		results = results[:100]
		truncated = true
	}

	// Build content message
	content := fmt.Sprintf("Found %d matches", len(results))
	if truncated {
		content += " (showing first 100)"
	}
	content += ":\n" + strings.Join(results, "\n")

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"pattern":   name,
			"path":      path,
			"matches":   len(results),
			"max_depth": maxDepth,
			"truncated": truncated,
			"results":   results,
		},
	}, nil
}

func (t *find) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "find",
		Description: "Find files and directories by name or pattern using the find command. Supports wildcards and limits results to maximum 100 matches.",
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
	}
}

func (t *find) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "find",
		Version:  "1.0.0",
		Category: "search",
		Tags:     []string{"filesystem", "search", "files"},
	}
}
