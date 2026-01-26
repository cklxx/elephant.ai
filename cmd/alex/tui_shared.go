package main

import "strings"

const (
	tuiAgentName   = "alex"
	tuiAgentIndent = "  "
)

func indentBlock(content string, prefix string) string {
	if content == "" || prefix == "" {
		return content
	}

	hasTrailingNewline := strings.HasSuffix(content, "\n")
	content = strings.TrimSuffix(content, "\n")

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}

	out := strings.Join(lines, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
