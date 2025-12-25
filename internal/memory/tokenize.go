package memory

import "strings"

// tokenize performs a light-weight whitespace and punctuation based split
// intended for demo purposes only. It favours simplicity over linguistic
// accuracy to keep the dependency surface minimal.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		// Keep CJK characters as part of tokens by treating them as non-separators.
		if r >= 0x4E00 && r <= 0x9FFF {
			return false
		}
		return true
	})

	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		terms = append(terms, trimmed)
	}
	return terms
}
