package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// RipgrepTool implements ripgrep functionality
type RipgrepTool struct{}

func CreateRipgrepTool() *RipgrepTool {
	return &RipgrepTool{}
}

func (t *RipgrepTool) Name() string {
	return "ripgrep"
}

func (t *RipgrepTool) Description() string {
	return "Search for patterns in files using ripgrep (rg). Faster than grep. Limits results to maximum 100 matches."
}

func (t *RipgrepTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The pattern to search for",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to search in",
				"default":     ".",
			},
			"file_type": map[string]interface{}{
				"type":        "string",
				"description": "File type to search (e.g., 'go', 'js', 'py')",
			},
			"ignore_case": map[string]interface{}{
				"type":        "boolean",
				"description": "Ignore case",
				"default":     false,
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *RipgrepTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("pattern", "Pattern to search for").
		AddOptionalStringField("path", "Path to search in").
		AddOptionalStringField("file_type", "File type to search").
		AddOptionalBooleanField("ignore_case", "Ignore case")

	return validator.Validate(args)
}

func (t *RipgrepTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Check if ripgrep is available
	if !t.hasRipgrep() {
		return nil, fmt.Errorf("ripgrep (rg) is not installed. Install with: brew install ripgrep (macOS) or visit https://github.com/BurntSushi/ripgrep#installation")
	}

	// 参数已通过Validate验证，可以安全访问
	pattern := args["pattern"].(string)

	path := "."
	if p, ok := args["path"]; ok {
		path = p.(string)
	}

	ignoreCase := false
	if ic, ok := args["ignore_case"].(bool); ok {
		ignoreCase = ic
	}

	// Build ripgrep command
	cmdArgs := []string{}

	if ignoreCase {
		cmdArgs = append(cmdArgs, "-i")
	}

	cmdArgs = append(cmdArgs, "-n") // Always show line numbers

	if fileType, ok := args["file_type"].(string); ok && fileType != "" {
		cmdArgs = append(cmdArgs, "-t", fileType)
	}

	cmdArgs = append(cmdArgs, pattern, path)

	// Execute ripgrep command
	cmd := exec.CommandContext(ctx, "rg", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		// ripgrep returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &ToolResult{
				Content: "No matches found",
				Data: map[string]interface{}{
					"pattern":     pattern,
					"path":        path,
					"matches":     0,
					"ignore_case": ignoreCase,
					"file_type":   args["file_type"],
				},
			}, nil
		}
		return nil, fmt.Errorf("ripgrep command failed: %w", err)
	}

	// Process output
	lines := strings.Split(string(output), "\n")
	lines = lines[:len(lines)-1] // Remove last empty line

	// Limit results to 100 matches
	if len(lines) > 100 {
		lines = lines[:100]
	}

	return &ToolResult{
		Content: fmt.Sprintf("Found %d matches:\n%s", len(lines), strings.Join(lines, "\n")),
		Data: map[string]interface{}{
			"pattern":     pattern,
			"path":        path,
			"matches":     len(lines),
			"ignore_case": ignoreCase,
			"file_type":   args["file_type"],
			"results":     lines,
			"content":     fmt.Sprintf("Found %d matches:\n%s", len(lines), strings.Join(lines, "\n")),
		},
	}, nil
}

func (t *RipgrepTool) hasRipgrep() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}
