package utils

import "strings"

// TrimDedupeStrings trims entries, removes empties, and keeps first-seen order.
func TrimDedupeStrings(values []string) []string {
	if values == nil {
		return nil
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
