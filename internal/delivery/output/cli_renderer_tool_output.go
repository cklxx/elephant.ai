package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"alex/internal/domain/agent/types"

	"github.com/charmbracelet/lipgloss"
)

// formatToolOutput formats tool output based on tool category and hierarchy
func (r *CLIRenderer) formatToolOutput(_ *types.OutputContext, toolName, result string, indent string) string {
	// Use brighter gray (#808080) that works on both light and dark backgrounds
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	normalizedTool := strings.TrimSpace(toolName)
	if normalizedTool == "read_file" || normalizedTool == "write_file" || normalizedTool == "replace_in_file" {
		return r.formatSandboxFileOutput(normalizedTool, result, indent, grayStyle)
	}
	category := CategorizeToolName(normalizedTool)

	switch category {
	case types.CategoryShell, types.CategoryExecution:
		return r.formatExecutionOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryWeb:
		return r.formatWebOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryTask:
		return r.formatTaskOutput(normalizedTool, result, indent, grayStyle)
	default:
		cleaned := filterSystemReminders(result)
		preview := truncateWithEllipsis(cleaned, 80)
		return fmt.Sprintf("%s  %s\n", indent, grayStyle.Render("→ "+preview))
	}
}

// Category-specific formatters

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
	if toolName == "web_search" {
		return r.formatWebSearchOutput(cleaned, indent, style)
	}
	preview := truncateWithEllipsis(cleaned, 100)
	if preview == "" {
		preview = "ok"
	}
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
}

func (r *CLIRenderer) formatTaskOutput(_, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	lines := strings.Split(strings.TrimSpace(cleaned), "\n")
	var output strings.Builder
	for _, line := range lines {
		if line != "" {
			output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(line)))
		}
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

func (r *CLIRenderer) formatSandboxFileOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	switch toolName {
	case "read_file":
		lines := countLines(cleaned)
		return fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d lines read", lines)))
	case "write_file", "replace_in_file":
		if summary, ok := summarizeFileOperation(cleaned); ok {
			return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+summary))
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	default:
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	}
}

