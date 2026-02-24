package utils

import "strings"

// TrimDedupeStrings trims each entry, drops empty values, and preserves the
// first occurrence order while removing duplicates.
func TrimDedupeStrings(values []string) []string {
	if values == nil {
		return nil
	}
	normalized := make([]string, 0, len(values))
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
		normalized = append(normalized, trimmed)
	}
	return normalized
}
