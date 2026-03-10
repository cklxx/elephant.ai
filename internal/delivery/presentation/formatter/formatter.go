package formatter

import (
	"fmt"
	"sort"
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

// ToolArgsPresentation captures how arguments should be displayed in downstream renderers.
type ToolArgsPresentation struct {
	ShouldDisplay bool
	InlinePreview string
	Args          map[string]string
}

// FormatToolCall formats a single tool call for display
func (tf *ToolFormatter) FormatToolCall(name string, args map[string]any) string {
	presentation := tf.PrepareArgs(name, args)
	return tf.renderToolCall(name, presentation)
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
	case "bash", "shell_exec":
		return tf.bashArgs(args), 160
	case "file_read", "read_file":
		return tf.fileReadArgs(args), 120
	case "file_edit", "replace_in_file":
		return tf.fileEditArgs(args), 120
	case "file_write", "write_file":
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
	case "todo_read", "list_files", "list_dir":
		return tf.simplePathArgs(args), 80
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

// FormatToolResult formats a tool execution result with smart preview
func (tf *ToolFormatter) FormatToolResult(name string, content string, success bool) string {
	if !success {
		return "  ✗ failed"
	}

	// Smart formatting based on tool type
	switch name {
	case "bash", "shell_exec":
		return tf.formatBashResult(content)
	case "file_read", "read_file":
		return tf.formatFileReadResult(content)
	case "file_write", "write_file":
		return tf.formatFileWriteResult(content)
	case "file_edit", "replace_in_file":
		return tf.formatFileEditResult(content)
	case "grep", "ripgrep":
		return tf.formatGrepResult(content)
	case "find":
		return tf.formatFindResult(content)
	case "list_files", "list_dir":
		return tf.formatListFilesResult(content)
	case "web_search":
		return tf.formatWebSearchResult(content)
	case "web_fetch":
		return tf.formatWebFetchResult(content)
	case "todo_update", "todo_read":
		return tf.formatTodoResult(content)
	case "final":
		return tf.formatFinalResult(content)
	default:
		return tf.formatDefaultResult(content)
	}
}
