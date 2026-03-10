package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	domain "alex/internal/domain/agent"
	"alex/internal/shared/utils"
)

// formatBashResult shows command output summary
func (tf *ToolFormatter) formatBashResult(content string) string {
	type bashPayload struct {
		Command  string `json:"command"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode *int   `json:"exit_code"`
	}

	var payload bashPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		// Fallback to legacy behaviour when payload is plain text
		lines := strings.Split(strings.TrimSpace(content), "\n")
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			return "  → Success (no output)"
		}
		if len(lines) == 1 {
			output := lines[0]
			if len(output) > 100 {
				output = output[:100] + "..."
			}
			return fmt.Sprintf("  → %s", output)
		}
		return fmt.Sprintf("  → %d lines output", len(lines))
	}

	stdout := strings.TrimSpace(payload.Stdout)
	stderr := strings.TrimSpace(payload.Stderr)
	exitCode := 0
	hasExit := false
	if payload.ExitCode != nil {
		exitCode = *payload.ExitCode
		hasExit = true
	}

	var parts []string

	if hasExit && exitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit %d", exitCode))
	}

	if stdout != "" {
		stdoutLines := countLines(stdout)
		if stdoutLines == 1 && utf8.RuneCountInString(stdout) <= 100 {
			parts = append(parts, stdout)
		} else {
			parts = append(parts, fmt.Sprintf("stdout %d %s", stdoutLines, pluralize("line", stdoutLines)))
		}
	}

	if stderr != "" {
		stderrLines := countLines(stderr)
		if stderrLines == 1 && utf8.RuneCountInString(stderr) <= 80 {
			parts = append(parts, fmt.Sprintf("stderr: %s", stderr))
		} else {
			parts = append(parts, fmt.Sprintf("stderr %d %s", stderrLines, pluralize("line", stderrLines)))
		}
	}

	if len(parts) == 0 {
		if hasExit {
			if exitCode == 0 {
				return "  → Success (no output)"
			}
			return fmt.Sprintf("  → exit %d", exitCode)
		}
		return "  → Success (no output)"
	}

	return fmt.Sprintf("  → %s", strings.Join(parts, ", "))
}

// formatFileReadResult shows file path + first 2-3 lines
func (tf *ToolFormatter) formatFileReadResult(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "  → empty file"
	}

	// Extract up to 3 meaningful lines
	var preview []string
	for i := 0; i < len(lines) && len(preview) < 3; i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
			if len(line) > 80 {
				line = line[:80] + "..."
			}
			preview = append(preview, "    "+line)
		}
	}

	if len(preview) == 0 {
		return "  → " + fmt.Sprintf("%d lines", len(lines))
	}

	return "  → file preview:\n" + strings.Join(preview, "\n")
}

// formatFileWriteResult shows success
func (tf *ToolFormatter) formatFileWriteResult(content string) string {
	if strings.Contains(content, "created") {
		return "  → File created successfully"
	}
	return "  → File written successfully"
}

// formatFileEditResult shows success
func (tf *ToolFormatter) formatFileEditResult(content string) string {
	if strings.Contains(content, "1 replacement") {
		return "  → 1 replacement made"
	}
	// Try to extract replacement count
	if strings.Contains(content, "replacement") || strings.Contains(content, "Replaced ") {
		return "  → Replacements made"
	}
	return "  → File edited successfully"
}

// formatGrepResult shows match count
func (tf *ToolFormatter) formatGrepResult(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	matchCount := 0

	for _, line := range lines {
		if utils.HasContent(line) && !strings.HasPrefix(line, "Search") {
			matchCount++
		}
	}

	if matchCount == 0 {
		return "  → No matches found"
	}

	if matchCount == 1 {
		return "  → 1 match found"
	}

	return fmt.Sprintf("  → %d matches found", matchCount)
}

// formatFindResult shows file count
func (tf *ToolFormatter) formatFindResult(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	fileCount := 0

	for _, line := range lines {
		if utils.HasContent(line) && !strings.HasPrefix(line, "Found") {
			fileCount++
		}
	}

	if fileCount == 0 {
		return "  → No files found"
	}

	if fileCount == 1 {
		return "  → 1 file found"
	}

	return fmt.Sprintf("  → %d files found", fileCount)
}

// formatWebSearchResult shows result count
func (tf *ToolFormatter) formatWebSearchResult(content string) string {
	lines := strings.Split(content, "\n")
	resultCount := 0

	for _, line := range lines {
		if strings.Contains(line, "http") || strings.Contains(line, "Title:") {
			resultCount++
		}
	}

	if resultCount == 0 {
		return "  → No results found"
	}

	return fmt.Sprintf("  → %d search results", resultCount/2) // Title + URL = 1 result
}

// formatWebFetchResult shows content length
func (tf *ToolFormatter) formatWebFetchResult(content string) string {
	lines := len(strings.Split(content, "\n"))
	chars := len(content)

	if chars < 1000 {
		return fmt.Sprintf("  → Fetched %d bytes", chars)
	}

	return fmt.Sprintf("  → Fetched %d lines (%dKB)", lines, chars/1024)
}

func (tf *ToolFormatter) formatFinalResult(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "  ✓ Final response prepared"
	}
	return fmt.Sprintf("  ✓ %s", tf.summarizeString(trimmed, domain.ToolResultPreviewRunes))
}

// formatDefaultResult shows compact preview
func (tf *ToolFormatter) formatDefaultResult(content string) string {
	preview := content
	if len(preview) > 60 {
		preview = preview[:60] + "..."
	}
	preview = strings.ReplaceAll(preview, "\n", " ")
	return "  → " + preview
}
