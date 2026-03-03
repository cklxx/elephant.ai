package utils

import "strings"

// TrimLower trims whitespace and lowercases s. Use for case-insensitive normalization.
func TrimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// IsBlank returns true when s is empty or contains only whitespace.
func IsBlank(s string) bool { return strings.TrimSpace(s) == "" }

// HasContent returns true when s contains at least one non-whitespace character.
func HasContent(s string) bool { return strings.TrimSpace(s) != "" }

// Truncate trims surrounding whitespace and limits s to maxRunes runes.
func Truncate(s string, maxRunes int) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || maxRunes <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}

// TruncateWithEllipsis trims whitespace and truncates with "..." when needed.
func TruncateWithEllipsis(s string, maxRunes int) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || maxRunes <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "..."
}

// NormalizeSessionTitle trims and truncates session titles for storage/display.
func NormalizeSessionTitle(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexAny(trimmed, "\r\n"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}

	runes := []rune(trimmed)
	const maxRunes = 32
	if len(runes) > maxRunes {
		trimmed = string(runes[:maxRunes]) + "…"
	}
	return trimmed
}
