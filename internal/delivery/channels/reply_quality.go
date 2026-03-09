package channels

import (
	"strings"

	"alex/internal/shared/utils"
)

// ShapeReply7C applies low-risk structural cleanup for direct user replies.
// It keeps factual content intact while improving readability/coherence:
// - normalize line endings
// - trim trailing whitespace
// - collapse repeated blank lines
// - remove accidental consecutive duplicate lines (outside code fences)
func ShapeReply7C(raw string) string {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}

	var out []string
	prevNormalized := ""
	blankStreak := 0
	inCodeFence := false

	for _, line := range lines {
		trimmedRight := strings.TrimRight(line, " \t")
		trimmed := strings.TrimSpace(trimmedRight)

		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
			out = append(out, trimmedRight)
			prevNormalized = ""
			blankStreak = 0
			continue
		}

		if inCodeFence {
			out = append(out, trimmedRight)
			continue
		}

		if trimmed == "" {
			blankStreak++
			if len(out) == 0 || blankStreak > 1 {
				continue
			}
			out = append(out, "")
			prevNormalized = ""
			continue
		}

		// Strip standalone horizontal rules ("---", "***", "___") — they
		// render poorly in Lark and add visual noise.  Check before
		// resetting blankStreak so surrounding blank lines collapse.
		if isHorizontalRule(trimmed) {
			continue
		}

		blankStreak = 0

		if prevNormalized != "" && trimmed == prevNormalized && !isStructuredMarkdownLine(trimmed) {
			continue
		}
		out = append(out, trimmedRight)
		prevNormalized = trimmed
	}

	for len(out) > 0 && utils.IsBlank(out[len(out)-1]) {
		out = out[:len(out)-1]
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

// isHorizontalRule returns true for markdown horizontal rules: ---, ***, ___.
func isHorizontalRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	ch := line[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for i := 1; i < len(line); i++ {
		if line[i] != ch {
			return false
		}
	}
	return true
}

func isStructuredMarkdownLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "+ ") ||
		strings.HasPrefix(trimmed, ">") ||
		strings.HasPrefix(trimmed, "|") {
		return true
	}
	if len(trimmed) >= 3 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' && trimmed[2] == ' ' {
		return true
	}
	return false
}
