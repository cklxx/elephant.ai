package formatter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"alex/internal/domain/agent"
)

// ToolFormatter formats tool calls for display output
type ToolFormatter struct {
	colorDot string
}

// NewToolFormatter creates a new formatter
func NewToolFormatter() *ToolFormatter {
	return &ToolFormatter{
		colorDot: "\033[32m⏺\033[0m", // Green dot
	}
}

// FormatToolCall formats a single tool call for display
func (tf *ToolFormatter) FormatToolCall(name string, args map[string]any) string {
	presentation := tf.PrepareArgs(name, args)
	return tf.renderToolCall(name, presentation)
}

// ToolArgsPresentation captures how arguments should be displayed in downstream renderers.
type ToolArgsPresentation struct {
	ShouldDisplay bool
	InlinePreview string
	Args          map[string]string
}

// PrepareArgs analyses tool call arguments and decides whether they should be surfaced.
func (tf *ToolFormatter) PrepareArgs(name string, args map[string]any) ToolArgsPresentation {
	normalized, inlineLimit := tf.visibleArgs(name, args)
	if len(normalized) == 0 {
		return ToolArgsPresentation{ShouldDisplay: false, Args: map[string]string{}}
	}

	inline := tf.joinArgs(normalized, inlineLimit)
	return ToolArgsPresentation{
		ShouldDisplay: inline != "",
		InlinePreview: inline,
		Args:          normalized,
	}
}

func (tf *ToolFormatter) renderToolCall(name string, presentation ToolArgsPresentation) string {
	if presentation.ShouldDisplay && presentation.InlinePreview != "" {
		return fmt.Sprintf("%s %s(%s)", tf.colorDot, name, presentation.InlinePreview)
	}

	return fmt.Sprintf("%s %s", tf.colorDot, name)
}

func (tf *ToolFormatter) visibleArgs(name string, args map[string]any) (map[string]string, int) {
	if len(args) == 0 {
		return map[string]string{}, 0
	}

	switch name {
	case "code_execute", "execute_code":
		return tf.codeExecuteArgs(args), 80
	case "bash", "shell_exec":
		return tf.bashArgs(args), 160
	case "file_read":
		return tf.fileReadArgs(args), 120
	case "file_edit":
		return tf.fileEditArgs(args), 120
	case "file_write":
		return tf.fileWriteArgs(args), 120
	case "grep", "ripgrep", "code_search":
		return tf.searchArgs(args), 140
	case "find":
		return tf.findArgs(args), 140
	case "web_search":
		return tf.webSearchArgs(args), 140
	case "web_fetch":
		return tf.webFetchArgs(args), 160
	case "todo_update":
		return map[string]string{}, 0
	case "todo_read", "list_files":
		return tf.simplePathArgs(args), 80
	case "subagent":
		return tf.subagentArgs(args), 120
	case "final":
		return tf.finalArgs(args), 160
	default:
		return tf.genericArgs(args), 120
	}
}

func (tf *ToolFormatter) joinArgs(args map[string]string, maxLen int) string {
	if len(args) == 0 {
		return ""
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		value := strings.TrimSpace(args[key])
		if value == "" {
			continue
		}

		if builder.Len() > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("%s=%s", key, value))

		if maxLen > 0 && builder.Len() > maxLen {
			preview := builder.String()
			if len(preview) > maxLen {
				return preview[:maxLen] + "…"
			}
			return preview
		}
	}

	return builder.String()
}

func (tf *ToolFormatter) codeExecuteArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if lang := tf.getStringArg(args, "language", ""); lang != "" {
		result["language"] = lang
	}
	if path := tf.getStringArg(args, "code_path", ""); path != "" {
		result["code_path"] = path
	}
	if code := tf.getStringArg(args, "code", ""); code != "" {
		result["lines"] = strconv.Itoa(countLines(code))
		result["chars"] = strconv.Itoa(utf8.RuneCountInString(code))
	}
	return result
}

func (tf *ToolFormatter) bashArgs(args map[string]any) map[string]string {
	command := tf.getStringArg(args, "command", "")
	if command == "" {
		return map[string]string{}
	}
	return map[string]string{
		"command": tf.summarizeString(command, 160),
	}
}

func (tf *ToolFormatter) fileReadArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if path := tf.getStringArg(args, "file_path", ""); path != "" {
		result["path"] = path
	}
	if offset := tf.getIntArg(args, "offset", 0); offset > 0 {
		result["offset"] = strconv.Itoa(offset)
	}
	if limit := tf.getIntArg(args, "limit", 0); limit > 0 {
		result["limit"] = strconv.Itoa(limit)
	}
	return result
}

func (tf *ToolFormatter) fileEditArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if path := tf.getStringArg(args, "file_path", ""); path != "" {
		result["path"] = path
	}
	if oldStr := tf.getStringArg(args, "old_string", ""); oldStr != "" {
		result["old_lines"] = strconv.Itoa(countLines(oldStr))
	}
	if newStr := tf.getStringArg(args, "new_string", ""); newStr != "" {
		result["new_lines"] = strconv.Itoa(countLines(newStr))
	}
	if oldStr := tf.getStringArg(args, "old_string", ""); oldStr != "" {
		if newStr := tf.getStringArg(args, "new_string", ""); newStr != "" {
			delta := utf8.RuneCountInString(newStr) - utf8.RuneCountInString(oldStr)
			result["delta_chars"] = strconv.Itoa(delta)
		}
	}
	return result
}

func (tf *ToolFormatter) fileWriteArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if path := tf.getStringArg(args, "file_path", ""); path != "" {
		result["path"] = path
	}
	if content := tf.getStringArg(args, "content", ""); content != "" {
		result["lines"] = strconv.Itoa(countLines(content))
		result["chars"] = strconv.Itoa(utf8.RuneCountInString(content))
	}
	return result
}

func (tf *ToolFormatter) searchArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if pattern := tf.getStringArg(args, "pattern", ""); pattern != "" {
		result["pattern"] = tf.summarizeString(pattern, 80)
	}
	if path := tf.getStringArg(args, "path", ""); path != "" && path != "." {
		result["path"] = path
	}
	return result
}

func (tf *ToolFormatter) findArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if pattern := tf.getStringArg(args, "pattern", ""); pattern != "" {
		result["pattern"] = tf.summarizeString(pattern, 60)
	}
	if path := tf.getStringArg(args, "path", ""); path != "" && path != "." {
		result["path"] = path
	}
	return result
}

func (tf *ToolFormatter) webSearchArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if query := tf.getStringArg(args, "query", ""); query != "" {
		result["query"] = tf.summarizeString(query, 80)
	}
	if maxResults := tf.getIntArg(args, "max_results", 0); maxResults > 0 {
		result["max_results"] = strconv.Itoa(maxResults)
	}
	return result
}

func (tf *ToolFormatter) webFetchArgs(args map[string]any) map[string]string {
	if url := tf.getStringArg(args, "url", ""); url != "" {
		return map[string]string{"url": tf.summarizeString(url, 120)}
	}
	return map[string]string{}
}

func (tf *ToolFormatter) simplePathArgs(args map[string]any) map[string]string {
	if path := tf.getStringArg(args, "path", ""); path != "" {
		return map[string]string{"path": path}
	}
	return map[string]string{}
}

func (tf *ToolFormatter) subagentArgs(args map[string]any) map[string]string {
	if prompt := tf.getStringArg(args, "prompt", ""); prompt != "" {
		return map[string]string{
			"prompt": tf.summarizeString(prompt, 120),
		}
	}
	return map[string]string{}
}

func (tf *ToolFormatter) finalArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if answer := tf.getStringArg(args, "answer", ""); answer != "" {
		result["answer"] = tf.summarizeString(answer, 120)
	}
	if highlights := tf.getStringSliceArg(args, "highlights"); len(highlights) > 0 {
		summary := strings.Join(highlights, " | ")
		result["highlights"] = tf.summarizeString(summary, 120)
	}
	return result
}

func (tf *ToolFormatter) genericArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	for key, value := range args {
		result[key] = tf.summarizeString(tf.formatValue(value), 80)
	}
	return result
}

func (tf *ToolFormatter) summarizeString(value string, limit int) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if cleaned == "" {
		return ""
	}

	runes := []rune(cleaned)
	if len(runes) <= limit {
		return cleaned
	}

	truncated := string(runes[:limit])
	remaining := len(runes) - limit
	if remaining <= 0 {
		return truncated
	}

	return fmt.Sprintf("%s… (+%d chars)", truncated, remaining)
}

func countLines(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// Helper functions to extract typed arguments
func (tf *ToolFormatter) getStringArg(args map[string]any, key, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
}

func (tf *ToolFormatter) getStringSliceArg(args map[string]any, key string) []string {
	if raw, ok := args[key]; ok {
		switch typed := raw.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			var values []string
			for _, item := range typed {
				if str, ok := item.(string); ok {
					trimmed := strings.TrimSpace(str)
					if trimmed != "" {
						values = append(values, trimmed)
					}
				}
			}
			return values
		}
	}
	return nil
}

func (tf *ToolFormatter) getIntArg(args map[string]any, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	if val, ok := args[key].(int); ok {
		return val
	}
	return defaultVal
}

// formatValue safely converts any value to string
func (tf *ToolFormatter) formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// FormatToolResult formats a tool execution result with smart preview
func (tf *ToolFormatter) FormatToolResult(name string, content string, success bool) string {
	if !success {
		return "  ✗ failed"
	}

	// Smart formatting based on tool type
	switch name {
	case "code_execute":
		return tf.formatCodeExecuteResult(content)
	case "bash":
		return tf.formatBashResult(content)
	case "file_read":
		return tf.formatFileReadResult(content)
	case "file_write":
		return tf.formatFileWriteResult(content)
	case "file_edit":
		return tf.formatFileEditResult(content)
	case "grep", "ripgrep":
		return tf.formatGrepResult(content)
	case "find":
		return tf.formatFindResult(content)
	case "list_files":
		return tf.formatListFilesResult(content)
	case "web_search":
		return tf.formatWebSearchResult(content)
	case "web_fetch":
		return tf.formatWebFetchResult(content)
	case "todo_update", "todo_read":
		return tf.formatTodoResult(content)
	case "subagent":
		return tf.formatSubagentResult(content)
	case "final":
		return tf.formatFinalResult(content)
	default:
		return tf.formatDefaultResult(content)
	}
}

// formatCodeExecuteResult shows execution time and output preview
func (tf *ToolFormatter) formatCodeExecuteResult(content string) string {
	lines := strings.Split(content, "\n")

	// Try to find execution time info
	var timeInfo string
	var outputLines []string

	for _, line := range lines {
		if strings.Contains(line, "Execution time:") || strings.Contains(line, "ms") {
			timeInfo = strings.TrimSpace(line)
		} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "Code executed") {
			outputLines = append(outputLines, strings.TrimSpace(line))
		}
	}

	// Build result
	var result string
	if timeInfo != "" {
		result = fmt.Sprintf("  → Success in %s", strings.TrimPrefix(timeInfo, "Execution time: "))
	} else {
		result = "  → Success"
	}

	// Add output preview (first 80 chars)
	if len(outputLines) > 0 {
		output := strings.Join(outputLines, " ")
		if len(output) > 80 {
			output = output[:80] + "..."
		}
		result += ": " + output
	}

	return result
}

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
	if strings.Contains(content, "replacement") {
		return "  → Replacements made"
	}
	return "  → File edited successfully"
}

// formatGrepResult shows match count
func (tf *ToolFormatter) formatGrepResult(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	matchCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "Search") {
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
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "Found") {
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

// formatSubagentResult shows task completion summary
func (tf *ToolFormatter) formatSubagentResult(content string) string {
	// Parse summary line
	if strings.Contains(content, "Success:") && strings.Contains(content, "Failed:") {
		// Extract numbers
		successIdx := strings.Index(content, "Success: ")
		failedIdx := strings.Index(content, "Failed: ")

		if successIdx > 0 && failedIdx > successIdx {
			return "  → " + content[successIdx:failedIdx+10]
		}
	}

	return "  → Subtasks completed"
}

func (tf *ToolFormatter) formatFinalResult(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "  ✓ Final response prepared"
	}
	return fmt.Sprintf("  ✓ %s", tf.summarizeString(trimmed, domain.ToolResultPreviewRunes))
}

// formatTodoResult shows FULL todo list - user needs complete task overview
func (tf *ToolFormatter) formatTodoResult(content string) string {
	lines := strings.Split(content, "\n")
	var output []string
	currentSection := ""

	// Add header
	output = append(output, "  Todo List:")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect section headers (both text and markdown format)
		if strings.HasPrefix(line, "In Progress:") || strings.HasPrefix(line, "## In Progress") {
			currentSection = "progress"
			output = append(output, "")
			output = append(output, "  In Progress:")
			continue
		} else if strings.HasPrefix(line, "Pending:") || strings.HasPrefix(line, "## Pending") {
			currentSection = "pending"
			output = append(output, "")
			output = append(output, "  Pending:")
			continue
		} else if strings.Contains(line, "Completed:") || strings.HasPrefix(line, "## Completed") {
			currentSection = "completed"
			output = append(output, "")
			output = append(output, "  Completed:")
			continue
		} else if strings.HasPrefix(line, "Updated:") || strings.HasPrefix(line, "Todo List:") {
			// Skip summary/header lines
			continue
		}

		// Format task items - show ALL tasks (no limits)
		if strings.HasPrefix(line, "- [▶]") || strings.HasPrefix(line, "- [ ]") ||
			strings.HasPrefix(line, "- [✓]") || strings.HasPrefix(line, "- [x]") ||
			strings.HasPrefix(line, "- ") {
			// Extract task text
			task := line
			task = strings.TrimPrefix(task, "- [▶] ")
			task = strings.TrimPrefix(task, "- [ ] ")
			task = strings.TrimPrefix(task, "- [✓] ")
			task = strings.TrimPrefix(task, "- [x] ")
			task = strings.TrimPrefix(task, "- ")
			task = strings.TrimSpace(task)

			if task == "" {
				continue
			}

			// Add task with appropriate marker
			switch currentSection {
			case "progress":
				output = append(output, "    [▶] "+task)
			case "pending":
				output = append(output, "    [ ] "+task)
			case "completed":
				output = append(output, "    [✓] "+task)
			}
		}
	}

	if len(output) <= 1 { // Only header
		return "  → No tasks"
	}

	return strings.Join(output, "\n")
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

// formatListFilesResult shows file count and key files
func (tf *ToolFormatter) formatListFilesResult(content string) string {
	lines := strings.Split(content, "\n")
	var dirs, files []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[DIR]") {
			dirName := strings.TrimPrefix(line, "[DIR]")
			dirName = strings.TrimSpace(dirName)
			if dirName != "" {
				dirs = append(dirs, dirName)
			}
		} else if strings.HasPrefix(line, "[FILE]") {
			fileName := strings.TrimPrefix(line, "[FILE]")
			// Extract just filename (before size info)
			parts := strings.Fields(fileName)
			if len(parts) > 0 {
				files = append(files, parts[0])
			}
		}
	}

	var parts []string
	if len(dirs) > 0 {
		if len(dirs) <= 3 {
			parts = append(parts, fmt.Sprintf("dirs: %s", strings.Join(dirs, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%d dirs: %s...", len(dirs), strings.Join(dirs[:3], ", ")))
		}
	}
	if len(files) > 0 {
		if len(files) <= 3 {
			parts = append(parts, fmt.Sprintf("files: %s", strings.Join(files, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%d files: %s...", len(files), strings.Join(files[:3], ", ")))
		}
	}

	if len(parts) > 0 {
		return "  → " + strings.Join(parts, " | ")
	}
	return "  → empty"
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
