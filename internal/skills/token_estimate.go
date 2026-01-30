package skills

import "strings"

// EstimateTokens provides a lightweight token estimate for budget enforcement.
func EstimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	runes := len([]rune(trimmed))
	words := len(strings.Fields(trimmed))
	estimate := runes / 4
	if estimate < words {
		estimate = words
	}
	if estimate == 0 {
		estimate = 1
	}
	return estimate
}
