package utils

import "strings"

// TrimLower trims whitespace and lowercases s. Use for case-insensitive normalization.
func TrimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// IsBlank returns true when s is empty or contains only whitespace.
func IsBlank(s string) bool { return strings.TrimSpace(s) == "" }

// HasContent returns true when s contains at least one non-whitespace character.
func HasContent(s string) bool { return strings.TrimSpace(s) != "" }

// TruncateWithSuffix truncates value to maxRunes and appends suffix when truncated.
func TruncateWithSuffix(value string, maxRunes int, suffix string) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if suffix == "" {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes]) + suffix
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

	const maxRunes = 32
	if len([]rune(trimmed)) > maxRunes {
		trimmed = TruncateWithSuffix(trimmed, maxRunes, "…")
	}
	return trimmed
}
