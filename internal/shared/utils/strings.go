package utils

import "strings"

// TrimLower trims whitespace and lowercases s. Use for case-insensitive normalization.
func TrimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// IsBlank returns true when s is empty or contains only whitespace.
func IsBlank(s string) bool { return strings.TrimSpace(s) == "" }

// HasContent returns true when s contains at least one non-whitespace character.
func HasContent(s string) bool { return !IsBlank(s) }

// Truncate returns s truncated to maxRunes runes. If truncated, the suffix is appended.
// The returned string (including suffix) never exceeds maxRunes runes.
func Truncate(s string, maxRunes int, suffix string) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= len([]rune(suffix)) {
		return suffix
	}
	return string(runes[:maxRunes-len([]rune(suffix))]) + suffix
}

// TruncateWithEllipsis is a convenience wrapper that uses "..." as suffix.
func TruncateWithEllipsis(s string, maxRunes int) string {
	return Truncate(s, maxRunes, "...")
}

// CountLines returns the number of lines in content. Returns 0 for empty strings.
func CountLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// Pluralize appends "s" to word when count != 1.
func Pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
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

	return Truncate(trimmed, 32, "…")
}
