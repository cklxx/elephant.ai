package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"alex/internal/agent/types"

	"github.com/charmbracelet/lipgloss"
)

// formatToolOutput formats tool output based on tool category and hierarchy
func (r *CLIRenderer) formatToolOutput(_ *types.OutputContext, toolName, result string, indent string) string {
	// Use brighter gray (#808080) that works on both light and dark backgrounds
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	normalizedTool := strings.TrimSpace(toolName)
	if strings.HasPrefix(normalizedTool, "sandbox_file_") {
		return r.formatSandboxFileOutput(normalizedTool, result, indent, grayStyle)
	}
	category := CategorizeToolName(normalizedTool)

	switch category {
	case types.CategoryFile:
		return r.formatFileOutput(normalizedTool, result, indent, grayStyle)
	case types.CategorySearch:
		return r.formatSearchOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryShell, types.CategoryExecution:
		return r.formatExecutionOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryWeb:
		return r.formatWebOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryTask:
		return r.formatTaskOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryReasoning:
		return r.formatReasoningOutput(result, indent, grayStyle)
	default:
		cleaned := filterSystemReminders(result)
		preview := truncateWithEllipsis(cleaned, 80)
		return fmt.Sprintf("%s  %s\n", indent, grayStyle.Render("→ "+preview))
	}
}

// Category-specific formatters

func (r *CLIRenderer) formatFileOutput(toolName, result, indent string, style lipgloss.Style) string {
	// Clean system reminders
	cleaned := filterSystemReminders(result)

	switch toolName {
	case "file_read":
		lines := countLines(cleaned)
		return fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d lines read", lines)))
	case "file_write", "file_edit":
		if summary, ok := summarizeFileOperation(cleaned); ok {
			return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+summary))
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	case "list_files":
		return r.formatListFiles(cleaned, indent, style)
	default:
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	}
}

func (r *CLIRenderer) formatSearchOutput(_, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	summary := parseSearchSummary(cleaned)
	matchCount := summary.Total
	lines := summary.Matches
	if summary.NoMatches {
		matchCount = 0
	}

	var output strings.Builder
	if summary.NoMatches {
		output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ no matches")))
		return output.String()
	}
	summaryLine := fmt.Sprintf("→ %d matches", matchCount)
	if summary.Truncated {
		summaryLine += " (truncated)"
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(summaryLine)))

	// In verbose mode, show first few matches
	if r.verbose && matchCount > 0 {
		preview := lines
		if len(preview) > 5 {
			preview = preview[:5]
		}
		for _, line := range preview {
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
		if len(lines) > len(preview) {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", len(lines)-len(preview)))))
		}
		if summary.Warning != "" {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(summary.Warning)))
		}
	}

	return output.String()
}

func (r *CLIRenderer) formatExecutionOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)

	if toolName == "bash" {
		if formatted, ok := r.formatBashExecutionOutput(cleaned, indent, style); ok {
			return formatted
		}
	}

	// Show execution output with proper indentation
	if r.verbose {
		// In verbose mode, show more output
		lines := strings.Split(strings.TrimSpace(cleaned), "\n")
		var output strings.Builder
		output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ execution output:")))
		for i, line := range lines {
			if i >= 10 {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... (%d more lines)", len(lines)-10))))
				break
			}
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
		return output.String()
	}

	// Concise mode: just show summary
	preview := truncateWithEllipsis(cleaned, 100)
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
}

func (r *CLIRenderer) formatBashExecutionOutput(result, indent string, style lipgloss.Style) (string, bool) {
	type bashPayload struct {
		Command  string `json:"command"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode *int   `json:"exit_code"`
	}

	var payload bashPayload
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return "", false
	}

	stdout := strings.TrimRight(payload.Stdout, "\n")
	stderr := strings.TrimRight(payload.Stderr, "\n")
	exitCode := 0
	if payload.ExitCode != nil {
		exitCode = *payload.ExitCode
	}

	var summaryParts []string
	summaryParts = append(summaryParts, fmt.Sprintf("exit %d", exitCode))

	trimmedStdout := strings.TrimSpace(stdout)
	if trimmedStdout != "" {
		stdoutLines := countLines(trimmedStdout)
		if stdoutLines == 1 && utf8.RuneCountInString(trimmedStdout) <= 80 {
			summaryParts = append(summaryParts, trimmedStdout)
		} else {
			summaryParts = append(summaryParts, fmt.Sprintf("stdout %d %s", stdoutLines, pluralize("line", stdoutLines)))
		}
	} else {
		summaryParts = append(summaryParts, "stdout empty")
	}

	trimmedStderr := strings.TrimSpace(stderr)
	if trimmedStderr != "" {
		stderrLines := countLines(trimmedStderr)
		if stderrLines == 1 && utf8.RuneCountInString(trimmedStderr) <= 80 {
			summaryParts = append(summaryParts, fmt.Sprintf("stderr: %s", trimmedStderr))
		} else {
			summaryParts = append(summaryParts, fmt.Sprintf("stderr %d %s", stderrLines, pluralize("line", stderrLines)))
		}
	}

	var output strings.Builder
	fmt.Fprintf(&output, "%s  %s\n", indent, style.Render("→ "+strings.Join(summaryParts, ", ")))

	if r.verbose {
		if stdout != "" {
			r.writeVerboseStream(&output, indent, style, "stdout", stdout)
		}
		if stderr != "" {
			r.writeVerboseStream(&output, indent, style, "stderr", stderr)
		}
	}

	return output.String(), true
}

func (r *CLIRenderer) writeVerboseStream(builder *strings.Builder, indent string, style lipgloss.Style, label string, content string) {
	fmt.Fprintf(builder, "%s    %s\n", indent, style.Render(label+":"))
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i >= 10 {
			fmt.Fprintf(builder, "%s      %s\n", indent, style.Render(fmt.Sprintf("... (%d more lines)", len(lines)-10)))
			break
		}
		fmt.Fprintf(builder, "%s      %s\n", indent, style.Render(line))
	}
}

func (r *CLIRenderer) formatWebOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	switch toolName {
	case "web_search":
		return r.formatWebSearchOutput(cleaned, indent, style)
	case "web_fetch":
		return r.formatWebFetchOutput(cleaned, indent, style)
	default:
		preview := truncateWithEllipsis(cleaned, 100)
		if preview == "" {
			preview = "ok"
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
	}
}

func (r *CLIRenderer) formatTaskOutput(toolName, result, indent string, style lipgloss.Style) string {
	// Clean system reminders from output
	cleaned := filterSystemReminders(result)

	// For todo tools, format the task list nicely
	if toolName == "todo_update" || toolName == "todo_read" {
		return r.formatTodoList(cleaned, indent, style)
	}

	// Other task tools: show cleaned result
	lines := strings.Split(strings.TrimSpace(cleaned), "\n")
	var output strings.Builder
	for _, line := range lines {
		if line != "" {
			output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(line)))
		}
	}
	return output.String()
}

func (r *CLIRenderer) formatReasoningOutput(result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	preview := truncateWithEllipsis(cleaned, 100)
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
}

// formatTodoList formats todo list output with proper indentation
func (r *CLIRenderer) formatTodoList(content, indent string, style lipgloss.Style) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var output strings.Builder

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Add double indent for all lines (tool output should be indented)
		trimmed := strings.TrimSpace(line)
		output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(trimmed)))
	}

	return output.String()
}

// formatListFiles formats file list with count summary and optional preview
func (r *CLIRenderer) formatListFiles(content, indent string, style lipgloss.Style) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	summary := parseListFilesSummary(lines)
	totalCount := summary.Total
	if strings.TrimSpace(content) == "" {
		totalCount = 0
		summary = listFilesSummary{}
	}

	var output strings.Builder

	// Show count summary
	summaryParts := []string{}
	summaryParts = append(summaryParts, fmt.Sprintf("%d entries", totalCount))
	if summary.Dirs > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s", summary.Dirs, pluralize("dir", summary.Dirs)))
	}
	if summary.Files > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s", summary.Files, pluralize("file", summary.Files)))
	}
	if summary.TotalBytes > 0 {
		summaryParts = append(summaryParts, formatBytes(summary.TotalBytes))
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+strings.Join(summaryParts, ", "))))

	// In verbose mode, show first few files
	if r.verbose && totalCount > 0 && totalCount <= 10 {
		for _, line := range lines {
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
	} else if r.verbose && totalCount > 10 {
		for i := 0; i < 5; i++ {
			if lines[i] != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(lines[i])))
			}
		}
		output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", totalCount-5))))
	}

	return output.String()
}

func (r *CLIRenderer) formatWebSearchOutput(content, indent string, style lipgloss.Style) string {
	summary := parseWebSearchContent(content)
	var parts []string
	query := strings.TrimSpace(summary.Query)
	if query != "" {
		parts = append(parts, fmt.Sprintf("search %q", truncateInlinePreview(query, 48)))
	} else {
		parts = append(parts, "search")
	}
	if summary.ResultCount > 0 {
		parts = append(parts, fmt.Sprintf("%d results", summary.ResultCount))
	}
	if strings.TrimSpace(summary.Summary) != "" {
		parts = append(parts, "summary available")
	}

	line := strings.Join(parts, ", ")
	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+line)))

	if r.verbose {
		if summaryText := strings.TrimSpace(summary.Summary); summaryText != "" {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render("summary: "+truncateWithEllipsis(summaryText, 200))))
		}
		for i, item := range summary.Results {
			if i >= 3 {
				break
			}
			title := strings.TrimSpace(item.Title)
			if title == "" {
				continue
			}
			host := hostFromURL(item.URL)
			line := title
			if host != "" {
				line = fmt.Sprintf("%s (%s)", title, host)
			}
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
		}
	}

	return output.String()
}

func (r *CLIRenderer) formatWebFetchOutput(content, indent string, style lipgloss.Style) string {
	summary := parseWebFetchContent(content)
	host := hostFromURL(summary.URL)
	if host == "" {
		host = strings.TrimSpace(summary.URL)
	}
	body := summary.Content
	action := "fetched"
	if strings.TrimSpace(summary.Analysis) != "" || strings.TrimSpace(summary.Question) != "" {
		action = "analyzed"
		if summary.Analysis != "" {
			body = summary.Analysis
		}
	}
	lineCount := countLines(strings.TrimSpace(body))
	var parts []string
	if host != "" {
		parts = append(parts, fmt.Sprintf("%s %s", action, host))
	} else {
		parts = append(parts, action)
	}
	if summary.Cached {
		parts = append(parts, "cached")
	}
	if lineCount > 0 {
		parts = append(parts, fmt.Sprintf("%d lines", lineCount))
	}
	line := strings.Join(parts, ", ")

	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+line)))
	if r.verbose {
		if question := strings.TrimSpace(summary.Question); question != "" {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render("question: "+truncateWithEllipsis(question, 160))))
		}
		label := "content"
		if strings.TrimSpace(summary.Analysis) != "" {
			label = "analysis"
			body = summary.Analysis
		}
		preview := takePreviewLines(body, 3)
		if len(preview) > 0 {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(label+":")))
			for _, line := range preview {
				output.WriteString(fmt.Sprintf("%s      %s\n", indent, style.Render(truncateWithEllipsis(line, 200))))
			}
		}
	}
	return output.String()
}

func (r *CLIRenderer) formatSandboxFileOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	switch toolName {
	case "sandbox_file_read":
		lines := countLines(cleaned)
		return fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d lines read", lines)))
	case "sandbox_file_write", "sandbox_file_replace":
		if summary, ok := summarizeFileOperation(cleaned); ok {
			return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+summary))
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	case "sandbox_file_list":
		if summary, ok := parseSandboxFileListSummary(cleaned); ok {
			return r.renderSandboxFileList(summary, indent, style)
		}
		preview := truncateWithEllipsis(cleaned, 100)
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
	case "sandbox_file_search":
		if summary, ok := parseSandboxFileSearchSummary(cleaned); ok {
			return r.renderSandboxFileSearch(summary, indent, style)
		}
		preview := truncateWithEllipsis(cleaned, 100)
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
	default:
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	}
}

func (r *CLIRenderer) renderSandboxFileList(summary sandboxFileListSummary, indent string, style lipgloss.Style) string {
	var output strings.Builder
	parts := []string{fmt.Sprintf("%d entries", summary.Total)}
	if summary.Dirs > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", summary.Dirs, pluralize("dir", summary.Dirs)))
	}
	if summary.Files > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", summary.Files, pluralize("file", summary.Files)))
	}
	if summary.TotalBytes > 0 {
		parts = append(parts, formatBytes(summary.TotalBytes))
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+strings.Join(parts, ", "))))
	if r.verbose && len(summary.Entries) > 0 {
		preview := summary.Entries
		if len(preview) > 5 {
			preview = preview[:5]
		}
		for _, entry := range preview {
			line := entry.Path
			if entry.IsDir {
				line = "[DIR] " + line
			} else {
				line = "[FILE] " + line
				if entry.Size != nil {
					line = fmt.Sprintf("%s (%s)", line, formatBytes(*entry.Size))
				}
			}
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
		}
		if len(summary.Entries) > len(preview) {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", len(summary.Entries)-len(preview)))))
		}
	}
	return output.String()
}

func (r *CLIRenderer) renderSandboxFileSearch(summary sandboxFileSearchSummary, indent string, style lipgloss.Style) string {
	var output strings.Builder
	matchCount := len(summary.Matches)
	header := fmt.Sprintf("→ %d matches", matchCount)
	if summary.File != "" {
		header += fmt.Sprintf(" in %s", summary.File)
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(header)))
	if r.verbose && matchCount > 0 {
		preview := summary.Matches
		if len(preview) > 5 {
			preview = preview[:5]
		}
		for i, match := range preview {
			line := match
			if i < len(summary.Lines) && summary.Lines[i] > 0 {
				line = fmt.Sprintf("%d: %s", summary.Lines[i], match)
			}
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(truncateWithEllipsis(line, 200))))
		}
		if len(summary.Matches) > len(preview) {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", len(summary.Matches)-len(preview)))))
		}
	}
	return output.String()
}
