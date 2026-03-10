package server

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"alex/internal/shared/utils"
	"alex/internal/shared/uxphrases"
)

// formatHookEvent formats a hook payload into a friendly Chinese Lark message.
func (h *HooksBridge) formatHookEvent(p hookPayload) string {
	switch p.Event {
	case "PostToolUse":
		return formatPostToolUse(p)
	case "Stop":
		return formatStop(p)
	case "PreToolUse":
		return formatPreToolUse(p)
	default:
		return ""
	}
}

// formatPostToolUse creates a friendly message for a completed tool use.
func formatPostToolUse(p hookPayload) string {
	phrase := uxphrases.ToolPhrase(p.ToolName, 0)
	detail := toolDetail(p.ToolName, p.ToolInput)
	if detail != "" {
		return fmt.Sprintf("%s\n%s", phrase, detail)
	}
	return phrase
}

// formatPreToolUse creates a friendly message for a tool about to be used.
func formatPreToolUse(p hookPayload) string {
	phrase := uxphrases.ToolPhrase(p.ToolName, 1)
	thinking := extractPreToolThinkingLine(p.Thinking)
	detail := toolDetail(p.ToolName, p.ToolInput)
	lines := []string{phrase}
	if thinking != "" {
		lines = append(lines, "思路："+thinking)
	}
	if detail != "" {
		lines = append(lines, detail)
	}
	return strings.Join(lines, "\n")
}

// formatStop creates a completion message.
func formatStop(p hookPayload) string {
	var sb strings.Builder
	sb.WriteString("任务已完成")
	answer := p.Answer
	if utils.IsBlank(answer) {
		answer = p.Output
	}
	if utils.HasContent(answer) {
		sb.WriteString("\n")
		sb.WriteString(truncateHookText(answer, 800))
	}
	if p.Error != "" {
		sb.WriteString("\n出错了：")
		sb.WriteString(truncateHookText(p.Error, 400))
	}
	return sb.String()
}

// formatToolSummary compresses multiple tool messages into a single notification.
func formatToolSummary(messages []string) string {
	if len(messages) == 1 {
		return messages[0]
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("执行了 %d 步操作：\n", len(messages)))
	for _, m := range messages {
		line := firstLine(m)
		sb.WriteString("  • ")
		sb.WriteString(truncateHookText(line, 80))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx > 0 {
		return s[:idx]
	}
	return s
}

// toolDetail extracts a brief context hint from tool input.
func toolDetail(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}

	lower := strings.ToLower(toolName)

	if hasFilePrefix(lower) {
		if path := extractString(m, "path", "file_path", "filename"); path != "" {
			return fmt.Sprintf("📄 %s", filepath.Base(path))
		}
	}
	if hasShellPrefix(lower) {
		if cmd := extractString(m, "command", "cmd"); cmd != "" {
			return fmt.Sprintf("$ %s", truncateHookText(cmd, 120))
		}
	}
	if hasSearchPrefix(lower) {
		if q := extractString(m, "query", "search_query", "pattern"); q != "" {
			return fmt.Sprintf("🔍 %s", truncateHookText(q, 120))
		}
	}
	return ""
}

// hasFilePrefix reports whether the tool name indicates a file operation.
func hasFilePrefix(name string) bool {
	switch name {
	case "read", "write", "edit", "glob", "grep":
		return true
	}
	for _, p := range []string{"read_", "write_", "edit_", "replace_in_file", "create_file",
		"view_file", "patch_file", "list_dir", "list_files"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// hasShellPrefix reports whether the tool name indicates a shell/exec operation.
func hasShellPrefix(name string) bool {
	if name == "bash" {
		return true
	}
	for _, p := range []string{"shell_exec", "run_command", "terminal", "exec_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// hasSearchPrefix reports whether the tool name indicates a search operation.
func hasSearchPrefix(name string) bool {
	for _, p := range []string{"web_search", "web_fetch", "tavily", "search_web", "search_file", "search_code"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// extractString returns the first non-empty string value for the given keys.
func extractString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// truncateHookText trims and truncates a string to max runes, adding ellipsis.
func truncateHookText(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if max <= 0 || len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// compactHookText normalizes whitespace in a string.
func compactHookText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// extractPreToolThinkingLine compacts and truncates thinking text for display.
func extractPreToolThinkingLine(thinking string) string {
	thinking = compactHookText(thinking)
	if thinking == "" {
		return ""
	}
	return truncateHookText(thinking, 240)
}
