package utils

import "strings"

// TrimLower trims whitespace and lowercases s. Use for case-insensitive normalization.
func TrimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// IsBlank returns true when s is empty or contains only whitespace.
func IsBlank(s string) bool { return strings.TrimSpace(s) == "" }

// HasContent returns true when s contains at least one non-whitespace character.
func HasContent(s string) bool { return strings.TrimSpace(s) != "" }

// Truncate returns the first maxRunes runes of the trimmed input. No suffix is added.
func Truncate(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// TruncateWithEllipsis returns the first maxRunes runes of the trimmed input,
// appending "..." when truncation occurs.
func TruncateWithEllipsis(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
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

	runes := []rune(trimmed)
	const maxRunes = 32
	if len(runes) > maxRunes {
		trimmed = string(runes[:maxRunes]) + "…"
	}
	return trimmed
}
