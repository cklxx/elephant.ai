package utils

import "strings"

// IsBlank returns true when s is empty or contains only whitespace.
func IsBlank(s string) bool { return strings.TrimSpace(s) == "" }

// HasContent returns true when s contains at least one non-whitespace character.
func HasContent(s string) bool { return strings.TrimSpace(s) != "" }
