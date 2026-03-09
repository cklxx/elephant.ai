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

// splitMessage splits text into multiple chunks by semantic boundaries
// (double-newline paragraphs, code fence blocks, numbered lists).
// Each segment becomes its own message. If there are more than
// messageSplitMaxChunks segments, trailing ones are merged into the last chunk.
// Returns a single chunk when the text has no paragraph breaks.
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

// splitIntoSegments splits text into semantic segments that should not be
// broken apart: paragraphs, code fence blocks, and consecutive numbered lists.
func splitIntoSegments(text string) []string {
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
				// Starting a code fence — flush anything before it.
				flushCurrent()
				inCodeFence = true
				current = append(current, line)
				continue
			}
			// Closing a code fence.
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

		// Non-numbered line after a numbered list — break.
		if len(current) > 0 && isNumberedListLine(strings.TrimSpace(current[0])) && !isNumberedListLine(trimmed) {
			flushCurrent()
		}

		current = append(current, line)
	}

	// Flush remaining (including unclosed code fences).
	flushCurrent()
	return segments
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
