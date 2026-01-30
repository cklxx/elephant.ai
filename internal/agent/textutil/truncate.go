package textutil

import "strings"

// SmartTruncate trims and shortens a string with a head/tail snippet.
func SmartTruncate(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || limit <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	if limit <= 10 {
		return string(runes[:limit])
	}
	headLen := limit * 6 / 10
	tailLen := limit - headLen - 5
	if tailLen < 1 {
		tailLen = 1
	}
	return string(runes[:headLen]) + " ... " + string(runes[len(runes)-tailLen:])
}

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
