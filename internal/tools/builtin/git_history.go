package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type gitHistory struct{}

// NewGitHistory creates a new git history search tool
func NewGitHistory() ports.ToolExecutor {
	return &gitHistory{}
}

func (t *gitHistory) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Check if we're in a git repository
	if err := t.validateGitRepo(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	// Get search parameters
	query, _ := call.Arguments["query"].(string)
	searchType, _ := call.Arguments["type"].(string)
	file, _ := call.Arguments["file"].(string)
	limit, _ := call.Arguments["limit"].(float64)

	if limit == 0 {
		limit = 20 // Default limit
	}

	var result string
	var err error

	switch searchType {
	case "message", "":
		// Search commit messages (default)
		if query == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("query is required for message search")}, nil
		}
		result, err = t.searchCommitMessages(ctx, query, int(limit))

	case "code":
		// Search code changes
		if query == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("query is required for code search")}, nil
		}
		result, err = t.searchCodeChanges(ctx, query, int(limit))

	case "file":
		// Search file history
		if file == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("file parameter is required for file search")}, nil
		}
		result, err = t.searchFileHistory(ctx, file, int(limit))

	case "author":
		// Search by author
		if query == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("query is required for author search")}, nil
		}
		result, err = t.searchByAuthor(ctx, query, int(limit))

	case "date":
		// Search by date range
		if query == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("query is required for date search (format: YYYY-MM-DD or 'last week')")}, nil
		}
		result, err = t.searchByDate(ctx, query, int(limit))

	default:
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("invalid search type: %s. Valid types: message, code, file, author, date", searchType),
		}, nil
	}

	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("search failed: %w", err)}, nil
	}

	if result == "" {
		result = "No results found."
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: result,
		Metadata: map[string]any{
			"search_type": searchType,
			"query":       query,
			"file":        file,
			"limit":       int(limit),
		},
	}, nil
}

func (t *gitHistory) validateGitRepo(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository")
	}
	return nil
}

func (t *gitHistory) searchCommitMessages(ctx context.Context, query string, limit int) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("--grep=%s", query),
		"-i", // Case insensitive
		fmt.Sprintf("-%d", limit),
		"--pretty=format:%h - %s (%an, %ar)",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", nil
	}

	return fmt.Sprintf("Commits matching '%s':\n\n%s", query, result), nil
}

func (t *gitHistory) searchCodeChanges(ctx context.Context, query string, limit int) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("-S%s", query), // Pickaxe search
		fmt.Sprintf("-%d", limit),
		"--pretty=format:%h - %s (%an, %ar)",
		"--all",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", nil
	}

	// Get detailed changes for first few commits
	lines := strings.Split(result, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}

	detailed := fmt.Sprintf("Commits where code changed (search: '%s'):\n\n%s", query, strings.Join(lines, "\n"))

	// Add note about what changed
	if len(lines) > 0 {
		detailed += "\n\nUse 'git show <commit-hash>' to see detailed changes."
	}

	return detailed, nil
}

func (t *gitHistory) searchFileHistory(ctx context.Context, file string, limit int) (string, error) {
	// First check if file exists or ever existed
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("-%d", limit),
		"--pretty=format:%h - %s (%an, %ar)",
		"--follow", // Follow file renames
		"--", file,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("No commit history found for file: %s", file), nil
	}

	// Get file stats
	statsCmd := exec.CommandContext(ctx, "git", "log",
		"--numstat",
		"--pretty=format:",
		"--", file,
	)

	statsOutput, _ := statsCmd.CombinedOutput()
	stats := strings.TrimSpace(string(statsOutput))

	totalAdditions := 0
	totalDeletions := 0
	if stats != "" {
		lines := strings.Split(stats, "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var add, del int
				_, _ = fmt.Sscanf(fields[0], "%d", &add)
				_, _ = fmt.Sscanf(fields[1], "%d", &del)
				totalAdditions += add
				totalDeletions += del
			}
		}
	}

	output_str := fmt.Sprintf("History for file '%s':\n\n%s", file, result)
	if totalAdditions > 0 || totalDeletions > 0 {
		output_str += fmt.Sprintf("\n\nTotal changes: +%d/-%d lines", totalAdditions, totalDeletions)
	}

	return output_str, nil
}

func (t *gitHistory) searchByAuthor(ctx context.Context, author string, limit int) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("--author=%s", author),
		fmt.Sprintf("-%d", limit),
		"--pretty=format:%h - %s (%ar)",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", nil
	}

	// Get commit count
	countCmd := exec.CommandContext(ctx, "git", "rev-list",
		"--count",
		fmt.Sprintf("--author=%s", author),
		"HEAD",
	)

	countOutput, _ := countCmd.CombinedOutput()
	count := strings.TrimSpace(string(countOutput))

	return fmt.Sprintf("Commits by '%s' (showing %d of %s total):\n\n%s", author, limit, count, result), nil
}

func (t *gitHistory) searchByDate(ctx context.Context, dateQuery string, limit int) (string, error) {
	// Support various date formats
	var gitDateArg string

	// Check if it's a relative date like "last week", "yesterday", etc.
	if strings.Contains(dateQuery, "last") || strings.Contains(dateQuery, "ago") {
		gitDateArg = fmt.Sprintf("--since=%s", dateQuery)
	} else if strings.Contains(dateQuery, "..") {
		// Date range like "2024-01-01..2024-12-31"
		parts := strings.Split(dateQuery, "..")
		if len(parts) == 2 {
			cmd := exec.CommandContext(ctx, "git", "log",
				fmt.Sprintf("--since=%s", strings.TrimSpace(parts[0])),
				fmt.Sprintf("--until=%s", strings.TrimSpace(parts[1])),
				fmt.Sprintf("-%d", limit),
				"--pretty=format:%h - %s (%an, %ar)",
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return "", err
			}
			result := strings.TrimSpace(string(output))
			if result == "" {
				return "", nil
			}
			return fmt.Sprintf("Commits in date range '%s':\n\n%s", dateQuery, result), nil
		}
	} else {
		// Assume it's a specific date
		gitDateArg = fmt.Sprintf("--since=%s", dateQuery)
	}

	cmd := exec.CommandContext(ctx, "git", "log",
		gitDateArg,
		fmt.Sprintf("-%d", limit),
		"--pretty=format:%h - %s (%an, %ar)",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", nil
	}

	return fmt.Sprintf("Commits since '%s':\n\n%s", dateQuery, result), nil
}

func (t *gitHistory) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "git_history",
		Description: "Search git commit history by message, code changes, file, author, or date. Useful for understanding when and why code changed.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"query": {
					Type:        "string",
					Description: "Search query (commit message text, author name, or date)",
				},
				"type": {
					Type:        "string",
					Description: "Search type: 'message' (default), 'code', 'file', 'author', 'date'",
					Enum:        []any{"message", "code", "file", "author", "date"},
				},
				"file": {
					Type:        "string",
					Description: "File path (required when type='file')",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of results (default: 20)",
				},
			},
			Required: []string{},
		},
	}
}

func (t *gitHistory) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "git_history",
		Version:  "1.0.0",
		Category: "git",
		Tags:     []string{"git", "history", "search", "log"},
	}
}
