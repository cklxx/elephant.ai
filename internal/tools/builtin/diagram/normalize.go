package diagram

import (
	"errors"
	"strings"
)

const mermaidInitDirective = "%%{init:"

func normalizeMermaidSource(raw string) (normalized string, hasInitDirective bool, err error) {
	source := strings.ReplaceAll(raw, "\r\n", "\n")
	source = strings.TrimSpace(source)
	if source == "" {
		return "", false, errors.New("source is required")
	}

	source = stripMermaidCodeFence(source)
	source = strings.TrimSpace(source)
	if source == "" {
		return "", false, errors.New("source is required")
	}

	return source, strings.Contains(source, mermaidInitDirective), nil
}

func stripMermaidCodeFence(source string) string {
	lines := strings.Split(source, "\n")
	if len(lines) < 2 {
		return source
	}

	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, "```") {
		return source
	}

	lang := strings.TrimSpace(strings.TrimPrefix(first, "```"))
	if !strings.EqualFold(lang, "mermaid") {
		return source
	}

	end := len(lines)
	if strings.TrimSpace(lines[len(lines)-1]) == "```" {
		end = len(lines) - 1
	}
	return strings.Join(lines[1:end], "\n")
}

