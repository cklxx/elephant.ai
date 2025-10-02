package domain

import (
	"fmt"
	"strings"
)

// OutputFormatter formats tool results for different audiences
type OutputFormatter struct {
	verbose bool
}

// NewOutputFormatter creates a new output formatter
func NewOutputFormatter(verbose bool) *OutputFormatter {
	return &OutputFormatter{verbose: verbose}
}

// FormatForUser formats tool output for CLI display to user
// This is optimized for readability and conciseness
func (f *OutputFormatter) FormatForUser(toolName string, result string, metadata map[string]any) string {
	switch toolName {
	case "subagent":
		return f.formatSubagentForUser(result, metadata)
	case "file_read":
		return f.formatFileReadForUser(result, metadata)
	case "bash":
		return f.formatBashForUser(result, metadata)
	case "grep", "ripgrep":
		return f.formatSearchForUser(result, metadata)
	default:
		// Default: show full output for user
		return result
	}
}

// FormatForLLM formats tool output for LLM consumption
// This includes all details and structured information
func (f *OutputFormatter) FormatForLLM(toolName string, result string, metadata map[string]any) string {
	// LLM always gets the full, structured output
	// Metadata is already included in ToolResult
	return result
}

// formatSubagentForUser shows compact progress for subagent
func (f *OutputFormatter) formatSubagentForUser(result string, metadata map[string]any) string {
	if f.verbose {
		return result // Show everything in verbose mode
	}

	// Extract stats from metadata
	totalTasks, _ := metadata["total_tasks"].(int)
	successCount, _ := metadata["success_count"].(int)
	totalTokens, _ := metadata["total_tokens"].(int)
	totalToolCalls, _ := metadata["total_tool_calls"].(int)

	// Compact summary for user
	var output strings.Builder
	output.WriteString(fmt.Sprintf("🤖 Subagent: %d/%d tasks completed\n", successCount, totalTasks))
	output.WriteString(fmt.Sprintf("   Tokens: %d | Tool calls: %d\n", totalTokens, totalToolCalls))

	// Show brief results only if there are failures
	if failureCount, ok := metadata["failure_count"].(int); ok && failureCount > 0 {
		output.WriteString(fmt.Sprintf("   ⚠️  %d task(s) failed\n", failureCount))
	}

	return output.String()
}

// formatFileReadForUser shows summary for file read
func (f *OutputFormatter) formatFileReadForUser(result string, metadata map[string]any) string {
	if f.verbose {
		return result
	}

	// Show first 500 chars + line count
	lines := strings.Split(result, "\n")
	lineCount := len(lines)

	preview := result
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}

	return fmt.Sprintf("📄 Read %d lines\n%s", lineCount, preview)
}

// formatBashForUser shows bash output
func (f *OutputFormatter) formatBashForUser(result string, metadata map[string]any) string {
	// Always show full bash output to user (important for debugging)
	return result
}

// formatSearchForUser shows search results summary
func (f *OutputFormatter) formatSearchForUser(result string, metadata map[string]any) string {
	if f.verbose {
		return result
	}

	// Extract match count from metadata
	matchCount := 0
	if mc, ok := metadata["match_count"].(int); ok {
		matchCount = mc
	}

	lines := strings.Split(result, "\n")
	preview := strings.Join(lines[:min(10, len(lines))], "\n")

	return fmt.Sprintf("🔍 Found %d matches (showing first 10)\n%s", matchCount, preview)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
