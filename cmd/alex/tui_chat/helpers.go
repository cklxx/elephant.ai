package tui_chat

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// generateID creates a unique message ID
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}

// getToolIcon returns an icon for the given tool name
func getToolIcon(toolName string) string {
	icons := map[string]string{
		"file_read":    "📄",
		"file_write":   "✍️",
		"file_edit":    "✏️",
		"grep":         "🔍",
		"ripgrep":      "🔍",
		"code_search":  "🔎",
		"bash":         "💻",
		"code_execute": "▶️",
		"web_search":   "🌐",
		"web_fetch":    "📡",
		"list_files":   "📁",
		"find":         "🔎",
		"think":        "💭",
		"todo_read":    "📋",
		"todo_update":  "✅",
		"subagent":     "🤖",
		"git_commit":   "📝",
		"git_history":  "📜",
		"git_pr":       "🔀",
	}

	if icon, ok := icons[toolName]; ok {
		return icon
	}
	return "🔧" // Default tool icon
}

// createToolPreview generates a concise preview of tool results
func createToolPreview(toolName, result string) string {
	switch toolName {
	case "file_read":
		lines := strings.Count(result, "\n")
		return fmt.Sprintf("%d lines", lines)

	case "grep", "ripgrep", "code_search":
		matches := strings.Count(result, "\n")
		return fmt.Sprintf("%d matches", matches)

	case "file_write", "file_edit":
		return "✓ written"

	case "bash", "code_execute":
		if len(result) == 0 {
			return "success"
		}
		firstLine := strings.Split(result, "\n")[0]
		if len(firstLine) > 40 {
			return firstLine[:37] + "..."
		}
		return firstLine

	case "list_files":
		files := strings.Count(result, "\n")
		return fmt.Sprintf("%d items", files)

	case "web_search":
		return "search complete"

	case "web_fetch":
		return "fetched"

	case "think":
		if len(result) > 40 {
			return result[:37] + "..."
		}
		return result

	default:
		if len(result) > 40 {
			return result[:37] + "..."
		}
		return result
	}
}
