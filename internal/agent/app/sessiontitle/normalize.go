package sessiontitle

import "strings"

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
