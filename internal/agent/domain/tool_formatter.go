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

	// Special handling for todo_update - don't show parameters
	if name == "todo_update" {
		return fmt.Sprintf("%s %s", tf.colorDot, name)
	}

	// Format arguments
	var argsStr []string
	for k, v := range args {
		// Convert value to string safely
		valueStr := tf.formatValue(v)

		// Truncate if too long
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
	case "todo_update", "todo_read":
		return tf.formatTodoResult(content)
	case "file_read":
		return tf.formatFileReadResult(content)
	case "list_files":
		return tf.formatListFilesResult(content)
	default:
		return tf.formatDefaultResult(content)
	}
}

// formatTodoResult shows detailed todo list
func (tf *ToolFormatter) formatTodoResult(content string) string {
	lines := strings.Split(content, "\n")
	var output []string
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect section headers (both text and markdown format)
		if strings.HasPrefix(line, "In Progress:") || strings.HasPrefix(line, "## In Progress") {
			currentSection = "progress"
			continue
		} else if strings.HasPrefix(line, "Pending:") || strings.HasPrefix(line, "## Pending") {
			currentSection = "pending"
			continue
		} else if strings.Contains(line, "Completed:") || strings.HasPrefix(line, "## Completed") {
			currentSection = "completed"
			continue
		} else if strings.HasPrefix(line, "Updated:") || strings.HasPrefix(line, "Todo List:") {
			// Skip summary/header lines
			continue
		}

		// Format task items based on section (handle both markdown and plain text)
		if strings.HasPrefix(line, "- [▶]") || strings.HasPrefix(line, "- ") {
			// Extract task text
			task := line
			task = strings.TrimPrefix(task, "- [▶] ")
			task = strings.TrimPrefix(task, "- [ ] ")
			task = strings.TrimPrefix(task, "- [✓] ")
			task = strings.TrimPrefix(task, "- ")
			task = strings.TrimSpace(task)

			if task == "" {
				continue
			}

			// Add markdown format based on section
			switch currentSection {
			case "progress":
				output = append(output, "  - [▶] "+task)
			case "pending":
				output = append(output, "  - [ ] "+task)
			case "completed":
				if len(output) < 8 { // Limit completed tasks shown
					output = append(output, "  - [x] "+task)
				}
			}
		}
	}

	if len(output) > 0 {
		return strings.Join(output, "\n")
	}
	return "  → no tasks"
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
