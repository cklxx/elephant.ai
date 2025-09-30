package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GrepTool implements grep functionality
type GrepTool struct{}

func CreateGrepTool() *GrepTool {
	return &GrepTool{}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return `Search for text patterns in files using grep with line number output.

Usage:
- Searches for regular expression patterns in files
- Returns matches with filename and line numbers
- Supports recursive directory search
- Case-sensitive by default, can be made case-insensitive
- Limits results to maximum 100 matches, with each match line truncated to 200 characters
- Exceeding limits will show truncated results with warnings

Parameters:
- pattern: Regular expression pattern to search for (required)
- path: Directory or file to search in (defaults to current directory)
- recursive: Search subdirectories recursively (default: false)
- ignore_case: Perform case-insensitive search (default: false)

Example patterns:
- "function.*main" - Find lines containing "function" followed by "main"
- "TODO|FIXME" - Find lines with TODO or FIXME comments
- "^import" - Find lines starting with "import"

Output format:
filename:linenum:matched_line_content

Notes:
- Uses system grep command
- Returns "No matches found" if pattern not found
- Truncated output will show warnings with total counts
- Exit code 1 (grep no matches) is handled as normal result`
}

func (t *GrepTool) Parameters() map[string]interface{} {
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
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Search recursively",
				"default":     false,
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

func (t *GrepTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("pattern", "Pattern to search for").
		AddOptionalStringField("path", "Path to search in").
		AddOptionalBooleanField("recursive", "Search recursively").
		AddOptionalBooleanField("ignore_case", "Ignore case")

	return validator.Validate(args)
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// 防御性检查：确保参数存在且有效（通常已通过Validate验证）
	if args == nil {
		return nil, fmt.Errorf("arguments cannot be nil")
	}

	patternValue, exists := args["pattern"]
	if !exists {
		return nil, fmt.Errorf("pattern parameter is required")
	}

	pattern, ok := patternValue.(string)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}

	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	path := "."
	if p, ok := args["path"]; ok {
		path = p.(string)
	}

	recursive := false
	if r, ok := args["recursive"].(bool); ok {
		recursive = r
	}

	ignoreCase := false
	if ic, ok := args["ignore_case"].(bool); ok {
		ignoreCase = ic
	}

	// Build grep command
	cmdArgs := []string{}

	if ignoreCase {
		cmdArgs = append(cmdArgs, "-i")
	}

	cmdArgs = append(cmdArgs, "-n") // Always show line numbers

	if recursive {
		cmdArgs = append(cmdArgs, "-r")
	}

	cmdArgs = append(cmdArgs, pattern, path)

	// Execute grep command
	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		// grep returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &ToolResult{
				Content: "No matches found",
				Data: map[string]interface{}{
					"pattern":     pattern,
					"path":        path,
					"matches":     0,
					"recursive":   recursive,
					"ignore_case": ignoreCase,
					"content":     "No matches found",
				},
			}, nil
		}
		return nil, fmt.Errorf("grep command failed: %w", err)
	}

	// Process output
	lines := strings.Split(string(output), "\n")
	lines = lines[:len(lines)-1] // Remove last empty line

	const maxMatches = 100
	const maxLineChars = 200
	originalMatchCount := len(lines)
	truncatedByMatches := false
	linesToruncated := 0

	// Limit results to 100 matches
	if len(lines) > maxMatches {
		lines = lines[:maxMatches]
		truncatedByMatches = true
	}

	// Format output based on search context
	lines = t.formatOutput(path, recursive, lines)

	// Truncate each line to maxLineChars
	for i, line := range lines {
		if len(line) > maxLineChars {
			lines[i] = line[:maxLineChars] + "..."
			linesToruncated++
		}
	}

	// Build warning message
	var warnings []string
	if truncatedByMatches {
		warnings = append(warnings, fmt.Sprintf("Results truncated: showing %d of %d total matches (limit: %d matches)", len(lines), originalMatchCount, maxMatches))
	}
	if linesToruncated > 0 {
		warnings = append(warnings, fmt.Sprintf("%d match lines were truncated to %d characters", linesToruncated, maxLineChars))
	}

	warningMsg := ""
	if len(warnings) > 0 {
		warningMsg = "\n\n[TRUNCATED] " + strings.Join(warnings, ". ")
	}

	finalContent := fmt.Sprintf("Found %d matches:\n%s%s", len(lines), strings.Join(lines, "\n"), warningMsg)

	return &ToolResult{
		Content: finalContent,
		Data: map[string]interface{}{
			"pattern":              pattern,
			"path":                 path,
			"matches":              len(lines),
			"original_matches":     originalMatchCount,
			"recursive":            recursive,
			"ignore_case":          ignoreCase,
			"results":              lines,
			"truncated_by_matches": truncatedByMatches,
			"lines_truncated":      linesToruncated,
			"max_line_chars":       maxLineChars,
			"content":              finalContent,
		},
	}, nil
}

// formatOutput formats grep output based on search context
// For single file searches or current directory searches, removes filename prefix for better readability
func (t *GrepTool) formatOutput(path string, recursive bool, lines []string) []string {
	// If searching recursively or in complex paths, keep original format
	if recursive || (path != "." && strings.Contains(path, "/")) {
		return lines
	}

	// For current directory or single file searches, optimize format
	formatted := make([]string, len(lines))
	for i, line := range lines {
		// Check if line has the format "filename:linenum:content"
		colonIndex := strings.Index(line, ":")
		if colonIndex > 0 {
			// Find the second colon (after line number)
			remaining := line[colonIndex+1:]
			secondColonIndex := strings.Index(remaining, ":")
			if secondColonIndex > 0 {
				// Remove filename prefix, keep "linenum:content" format
				formatted[i] = remaining
			} else {
				// Keep original if format is unexpected
				formatted[i] = line
			}
		} else {
			// Keep original if no colon found
			formatted[i] = line
		}
	}

	return formatted
}
