package textutil

import "strings"

// NormalizeWhitespace collapses runs of whitespace to single spaces.
func NormalizeWhitespace(value string) string {
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
