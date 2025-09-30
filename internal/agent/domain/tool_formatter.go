package domain

import (
	"fmt"
	"strings"
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
	if len(args) == 0 {
		return fmt.Sprintf("%s %s()", tf.colorDot, name)
	}

	// Tool-specific formatting
	switch name {
	case "code_execute":
		return tf.formatCodeExecuteCall(args)
	case "bash":
		return tf.formatBashCall(args)
	case "file_read":
		return tf.formatFileReadCall(args)
	case "file_edit":
		return tf.formatFileEditCall(args)
	case "file_write":
		return tf.formatFileWriteCall(args)
	case "grep", "ripgrep":
		return tf.formatGrepCall(args)
	case "find":
		return tf.formatFindCall(args)
	case "web_search":
		return tf.formatWebSearchCall(args)
	case "web_fetch":
		return tf.formatWebFetchCall(args)
	case "think":
		return tf.formatThinkCall(args)
	case "todo_update":
		return fmt.Sprintf("%s %s", tf.colorDot, name)
	case "todo_read", "list_files":
		// Simple path display
		if path, ok := args["path"].(string); ok && path != "" {
			return fmt.Sprintf("%s %s(%s)", tf.colorDot, name, path)
		}
		return fmt.Sprintf("%s %s", tf.colorDot, name)
	case "subagent":
		return tf.formatSubagentCall(args)
	default:
		return tf.formatDefaultCall(name, args)
	}
}

// formatCodeExecuteCall shows FULL code - user needs to see what's being executed
func (tf *ToolFormatter) formatCodeExecuteCall(args map[string]any) string {
	lang := tf.getStringArg(args, "language", "code")
	code := tf.getStringArg(args, "code", "")

	if code == "" {
		return fmt.Sprintf("%s code_execute(language=%s)", tf.colorDot, lang)
	}

	// Show FULL code with language label
	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s code_execute(language=%s):\n", tf.colorDot, lang))

	// Add code block with indentation
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		output.WriteString("  " + line + "\n")
	}

	return strings.TrimRight(output.String(), "\n")
}

// formatBashCall shows FULL command - user needs to see exact command
func (tf *ToolFormatter) formatBashCall(args map[string]any) string {
	cmd := tf.getStringArg(args, "command", "")
	if cmd == "" {
		return fmt.Sprintf("%s bash", tf.colorDot)
	}

	// Show full command without truncation
	return fmt.Sprintf("%s bash: %s", tf.colorDot, cmd)
}

// formatFileReadCall shows file path
func (tf *ToolFormatter) formatFileReadCall(args map[string]any) string {
	path := tf.getStringArg(args, "file_path", "")
	offset := tf.getIntArg(args, "offset", 0)
	limit := tf.getIntArg(args, "limit", 0)

	if offset > 0 || limit > 0 {
		return fmt.Sprintf("%s file_read(%s, lines %d-%d)", tf.colorDot, path, offset, offset+limit)
	}
	return fmt.Sprintf("%s file_read(%s)", tf.colorDot, path)
}

// formatFileEditCall shows file and FULL edit - user needs to see what's changing
func (tf *ToolFormatter) formatFileEditCall(args map[string]any) string {
	path := tf.getStringArg(args, "file_path", "")
	oldStr := tf.getStringArg(args, "old_string", "")
	newStr := tf.getStringArg(args, "new_string", "")

	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s file_edit(%s):\n", tf.colorDot, path))

	// Show old -> new with clear separation
	output.WriteString("  - Old:\n")
	for _, line := range strings.Split(oldStr, "\n") {
		output.WriteString(fmt.Sprintf("    %s\n", line))
	}

	output.WriteString("  + New:\n")
	for _, line := range strings.Split(newStr, "\n") {
		output.WriteString(fmt.Sprintf("    %s\n", line))
	}

	return strings.TrimRight(output.String(), "\n")
}

// formatFileWriteCall shows file path
func (tf *ToolFormatter) formatFileWriteCall(args map[string]any) string {
	path := tf.getStringArg(args, "file_path", "")
	content := tf.getStringArg(args, "content", "")

	lines := strings.Count(content, "\n") + 1
	return fmt.Sprintf("%s file_write(%s, %d lines)", tf.colorDot, path, lines)
}

// formatGrepCall shows pattern and path
func (tf *ToolFormatter) formatGrepCall(args map[string]any) string {
	pattern := tf.getStringArg(args, "pattern", "")
	path := tf.getStringArg(args, "path", ".")

	if len(pattern) > 40 {
		pattern = pattern[:40] + "..."
	}

	if path != "." {
		return fmt.Sprintf("%s grep(\"%s\", %s)", tf.colorDot, pattern, path)
	}
	return fmt.Sprintf("%s grep(\"%s\")", tf.colorDot, pattern)
}

// formatFindCall shows pattern and path
func (tf *ToolFormatter) formatFindCall(args map[string]any) string {
	pattern := tf.getStringArg(args, "pattern", "")
	path := tf.getStringArg(args, "path", ".")

	if path != "." {
		return fmt.Sprintf("%s find(\"%s\", %s)", tf.colorDot, pattern, path)
	}
	return fmt.Sprintf("%s find(\"%s\")", tf.colorDot, pattern)
}

// formatWebSearchCall shows query
func (tf *ToolFormatter) formatWebSearchCall(args map[string]any) string {
	query := tf.getStringArg(args, "query", "")
	maxResults := tf.getIntArg(args, "max_results", 5)

	if len(query) > 60 {
		query = query[:60] + "..."
	}

	return fmt.Sprintf("%s web_search(max_results=%d, query=%s)", tf.colorDot, maxResults, query)
}

// formatWebFetchCall shows URL
func (tf *ToolFormatter) formatWebFetchCall(args map[string]any) string {
	url := tf.getStringArg(args, "url", "")

	if len(url) > 60 {
		url = url[:60] + "..."
	}

	return fmt.Sprintf("%s web_fetch(%s)", tf.colorDot, url)
}

// formatThinkCall shows FULL thought - user needs to see agent's reasoning
func (tf *ToolFormatter) formatThinkCall(args map[string]any) string {
	thought := tf.getStringArg(args, "thought", "")

	if thought == "" {
		return fmt.Sprintf("%s think", tf.colorDot)
	}

	// Show full thought with clear formatting
	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s think:\n", tf.colorDot))

	// Add thought with indentation
	lines := strings.Split(thought, "\n")
	for _, line := range lines {
		output.WriteString("  " + line + "\n")
	}

	return strings.TrimRight(output.String(), "\n")
}

// formatSubagentCall shows subtask count
func (tf *ToolFormatter) formatSubagentCall(args map[string]any) string {
	subtasks, ok := args["subtasks"].([]any)
	if !ok || len(subtasks) == 0 {
		return fmt.Sprintf("%s subagent", tf.colorDot)
	}

	mode := tf.getStringArg(args, "mode", "parallel")
	return fmt.Sprintf("%s subagent(%d tasks, %s)", tf.colorDot, len(subtasks), mode)
}

// formatDefaultCall handles unknown tools
func (tf *ToolFormatter) formatDefaultCall(name string, args map[string]any) string {
	var argsStr []string
	for k, v := range args {
		valueStr := tf.formatValue(v)
		if len(valueStr) > 30 {
			valueStr = valueStr[:30] + "..."
		}
		if len(args) > 1 {
			argsStr = append(argsStr, fmt.Sprintf("%s=%s", k, valueStr))
		} else {
			argsStr = append(argsStr, valueStr)
		}
	}
	return fmt.Sprintf("%s %s(%s)", tf.colorDot, name, strings.Join(argsStr, ", "))
}

// Helper functions to extract typed arguments
func (tf *ToolFormatter) getStringArg(args map[string]any, key, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
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
	case "think":
		return tf.formatThinkResult(content)
	case "todo_update", "todo_read":
		return tf.formatTodoResult(content)
	case "subagent":
		return tf.formatSubagentResult(content)
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

// formatThinkResult shows thought summary
func (tf *ToolFormatter) formatThinkResult(content string) string {
	preview := content
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	return fmt.Sprintf("  → %s", preview)
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

// extractTaskName extracts task description from todo line
func extractTaskName(line string) string {
	// Remove status indicators
	line = strings.TrimPrefix(line, "-")
	line = strings.TrimPrefix(line, "*")
	line = strings.TrimSpace(line)

	// Remove status words
	for _, status := range []string{"in progress", "in_progress", "pending", "completed"} {
		line = strings.ReplaceAll(line, status, "")
		line = strings.ReplaceAll(line, strings.ToUpper(status), "")
	}

	line = strings.TrimSpace(line)
	if len(line) > 50 {
		line = line[:50] + "..."
	}
	return line
}
