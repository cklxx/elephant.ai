package lark

import (
	"strings"
	"time"
)

const (
	// messageSplitMaxChunks caps the number of chunks to avoid message spam.
	messageSplitMaxChunks = 5
	// messageSplitDelay is the pause between consecutive messages to
	// simulate a natural typing rhythm.
	messageSplitDelay = 500 * time.Millisecond
)

// splitMessage splits text into multiple chunks by markdown structure.
// It preserves structural integrity: headings stay with their body content,
// code fences and numbered lists are kept intact, and intro paragraphs
// are grouped with immediately following lists.
// Returns a single chunk when the text has no structural breaks.
func splitMessage(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	segments := splitIntoSegments(text)
	if len(segments) <= 1 {
		return []string{text}
	}

	// Enforce maxChunks by merging trailing segments.
	if len(segments) > messageSplitMaxChunks {
		merged := make([]string, messageSplitMaxChunks)
		copy(merged, segments[:messageSplitMaxChunks-1])
		merged[messageSplitMaxChunks-1] = strings.Join(segments[messageSplitMaxChunks-1:], "\n\n")
		segments = merged
	}

	return segments
}

// splitIntoSegments splits text into semantic segments preserving markdown
// structure. If the text contains headings, it splits by sections (heading +
// body). Otherwise, it splits by paragraphs while keeping code fences,
// lists intact, and grouping intro text with following lists.
func splitIntoSegments(text string) []string {
	if hasHeadings(text) {
		return splitBySections(text)
	}
	return splitByParagraphs(text)
}

// hasHeadings returns true if text contains any markdown heading lines.
func hasHeadings(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if isHeadingLine(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

// isHeadingLine checks if a line is a markdown heading (starts with # ).
func isHeadingLine(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	return i >= 1 && i <= 6 && i < len(trimmed) && trimmed[i] == ' '
}

// splitBySections splits text into heading-delimited sections.
// Each heading starts a new section that includes all content until the
// next heading. Content before the first heading becomes its own section.
func splitBySections(text string) []string {
	lines := strings.Split(text, "\n")
	var sections []string
	var current []string

	flushSection := func() {
		if len(current) == 0 {
			return
		}
		seg := strings.TrimSpace(strings.Join(current, "\n"))
		if seg != "" {
			sections = append(sections, seg)
		}
		current = nil
	}

	inCodeFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track code fence state — never split inside code fences.
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
			current = append(current, line)
			continue
		}
		if inCodeFence {
			current = append(current, line)
			continue
		}

		// Heading outside code fence starts a new section.
		if isHeadingLine(trimmed) {
			flushSection()
			current = append(current, line)
			continue
		}

		current = append(current, line)
	}

	flushSection()
	return sections
}

// splitByParagraphs splits text by double-newline paragraph breaks, keeping
// code fences and lists intact. An intro paragraph immediately followed by
// a list is grouped together.
func splitByParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	var segments []string
	var current []string
	inCodeFence := false

	flushCurrent := func() {
		if len(current) == 0 {
			return
		}
		seg := strings.TrimSpace(strings.Join(current, "\n"))
		if seg != "" {
			segments = append(segments, seg)
		}
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code fence state.
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeFence {
				flushCurrent()
				inCodeFence = true
				current = append(current, line)
				continue
			}
			current = append(current, line)
			inCodeFence = false
			flushCurrent()
			continue
		}

		if inCodeFence {
			current = append(current, line)
			continue
		}

		// Empty line = paragraph break (outside code fence).
		if trimmed == "" {
			flushCurrent()
			continue
		}

		// Check if this is a numbered list continuation.
		if isNumberedListLine(trimmed) && len(current) > 0 && isNumberedListLine(strings.TrimSpace(current[len(current)-1])) {
			current = append(current, line)
			continue
		}

		// Check if this is a bullet list continuation.
		if isBulletListLine(trimmed) && len(current) > 0 && isBulletListLine(strings.TrimSpace(current[len(current)-1])) {
			current = append(current, line)
			continue
		}

		// Non-list line after a list — break.
		if len(current) > 0 {
			lastFirst := strings.TrimSpace(current[0])
			if (isNumberedListLine(lastFirst) || isBulletListLine(lastFirst)) && !isNumberedListLine(trimmed) && !isBulletListLine(trimmed) {
				flushCurrent()
			}
		}

		current = append(current, line)
	}

	flushCurrent()

	// Post-process: merge an intro paragraph with a following list.
	segments = mergeIntroWithList(segments)

	return segments
}

// mergeIntroWithList merges a non-list segment with the immediately following
// list segment, keeping the intro text together with its list.
func mergeIntroWithList(segments []string) []string {
	if len(segments) <= 1 {
		return segments
	}

	var merged []string
	i := 0
	for i < len(segments) {
		seg := segments[i]
		if i+1 < len(segments) && !isListSegment(seg) && isListSegment(segments[i+1]) {
			merged = append(merged, seg+"\n\n"+segments[i+1])
			i += 2
			continue
		}
		merged = append(merged, seg)
		i++
	}
	return merged
}

// isListSegment checks if a segment starts with a list item.
func isListSegment(seg string) bool {
	firstLine := seg
	if idx := strings.IndexByte(seg, '\n'); idx >= 0 {
		firstLine = seg[:idx]
	}
	trimmed := strings.TrimSpace(firstLine)
	return isNumberedListLine(trimmed) || isBulletListLine(trimmed)
}

// isBulletListLine checks if a line starts with a bullet list marker.
func isBulletListLine(line string) bool {
	return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ")
}

// isNumberedListLine checks if a line starts with a numbered list pattern (e.g. "1.", "2.").
func isNumberedListLine(line string) bool {
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 {
			return true
		}
		return false
	}
	return false
}
