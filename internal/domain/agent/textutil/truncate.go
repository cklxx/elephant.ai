package textutil

import "strings"

// TruncateWithEllipsis shortens input and appends an ellipsis.
func TruncateWithEllipsis(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	if limit == 1 {
		return "…"
	}
	trimmed := strings.TrimSpace(string(runes[:limit-1]))
	if trimmed == "" {
		trimmed = string(runes[:limit-1])
	}
	return trimmed + "…"
}
