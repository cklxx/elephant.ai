package utils

import "strings"

// TruncateTrimmedRunesASCII trims leading/trailing spaces and truncates to
// limit runes, appending "..." when truncation happens.
func TruncateTrimmedRunesASCII(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}
