package search

import (
	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
	"context"
	"fmt"
	"regexp"
	"strings"
)

type ripgrep struct {
	shared.BaseTool
}

func NewRipgrep(cfg shared.ShellToolConfig) tools.ToolExecutor {
	_ = cfg
	return &ripgrep{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "ripgrep",
				Description: "Search for patterns in files using ripgrep (rg). Faster than grep. Limits results to maximum 100 matches, with each match line truncated to 200 characters. Exceeding limits will show truncated results with warnings.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"pattern": {
							Type:        "string",
							Description: "The pattern to search for",
						},
						"path": {
							Type:        "string",
							Description: "Path to search in (default: .)",
						},
						"file_type": {
							Type:        "string",
							Description: "File type to search (e.g., 'go', 'js', 'py')",
						},
						"ignore_case": {
							Type:        "boolean",
							Description: "Ignore case (default: false)",
						},
						"max_results": {
							Type:        "number",
							Description: "Maximum number of results to return (default: 100)",
						},
					},
					Required: []string{"pattern"},
				},
			},
			ports.ToolMetadata{
				Name:        "ripgrep",
				Version:     "1.0.0",
				Category:    "search",
				Tags:        []string{"search", "files", "pattern"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
	}
}

func (t *ripgrep) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	pattern, ok := call.Arguments["pattern"].(string)
	if !ok || pattern == "" {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("missing required 'pattern' parameter"),
		}, nil
	}

	path := "."
	if p, ok := call.Arguments["path"].(string); ok && p != "" {
		path = p
	}

	resolvedPath, err := pathutil.SanitizePathWithinBase(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	ignoreCase := false
	if ic, ok := call.Arguments["ignore_case"].(bool); ok {
		ignoreCase = ic
	}

	maxResults := 100
	if mr, ok := call.Arguments["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	re, err := compileSearchPattern(pattern, ignoreCase)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	matches, total, err := searchTextMatches(resolvedPath, re, fileTypeFilter(shared.StringArg(call.Arguments, "file_type")), maxResults)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if total == 0 {
		return t.noMatchesResult(call, pattern, path, ignoreCase)
	}

	return t.processMatches(call, matches, total, pattern, path, resolvedPath, ignoreCase, maxResults)
}

func (t *ripgrep) processMatches(call ports.ToolCall, lines []string, originalMatchCount int, pattern, path, resolvedPath string, ignoreCase bool, maxResults int) (*ports.ToolResult, error) {
	const maxLineChars = 200
	truncatedByMatches := false
	linesTruncated := 0

	if maxResults > 0 && len(lines) > maxResults {
		lines = lines[:maxResults]
		truncatedByMatches = true
	}

	for i, line := range lines {
		if len(line) > maxLineChars {
			lines[i] = line[:maxLineChars] + "..."
			linesTruncated++
		}
	}

	var warnings []string
	if truncatedByMatches {
		warnings = append(warnings, fmt.Sprintf("Results truncated: showing %d of %d total matches (limit: %d matches)", len(lines), originalMatchCount, maxResults))
	}
	if linesTruncated > 0 {
		warnings = append(warnings, fmt.Sprintf("%d match lines were truncated to %d characters", linesTruncated, maxLineChars))
	}

	warningMsg := ""
	if len(warnings) > 0 {
		warningMsg = "\n\n[TRUNCATED] " + strings.Join(warnings, ". ")
	}

	finalContent := fmt.Sprintf("Found %d matches:\n%s%s", len(lines), strings.Join(lines, "\n"), warningMsg)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: finalContent,
		Metadata: map[string]any{
			"pattern":              pattern,
			"path":                 path,
			"resolved_path":        resolvedPath,
			"matches":              len(lines),
			"original_matches":     originalMatchCount,
			"ignore_case":          ignoreCase,
			"file_type":            call.Arguments["file_type"],
			"results":              lines,
			"truncated_by_matches": truncatedByMatches,
			"lines_truncated":      linesTruncated,
			"max_line_chars":       maxLineChars,
		},
	}, nil
}

func (t *ripgrep) noMatchesResult(call ports.ToolCall, pattern, path string, ignoreCase bool) (*ports.ToolResult, error) {
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "No matches found",
		Metadata: map[string]any{
			"pattern":     pattern,
			"path":        path,
			"matches":     0,
			"ignore_case": ignoreCase,
		},
	}, nil
}

func compileSearchPattern(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}
