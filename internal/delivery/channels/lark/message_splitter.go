package lark

import (
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	// messageSplitMaxChunks caps the number of chunks to avoid message spam.
	messageSplitMaxChunks = 5
	// messageSplitDelay is the pause between consecutive messages to
	// simulate a natural typing rhythm.
	messageSplitDelay = 500 * time.Millisecond
)

// splitMessage splits text into multiple chunks by markdown structure using
// a goldmark AST parser. Each chunk is a complete structural unit — headings
// stay with their body content, code fences, lists, tables, and blockquotes
// are never broken apart.
// Returns a single chunk when the text has no structural breaks.
func splitMessage(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	segments := splitByAST(text)
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

// topLevelBlock describes a top-level AST node's position in source lines.
type topLevelBlock struct {
	kind      ast.NodeKind
	startLine int // inclusive, 0-indexed
	endLine   int // inclusive, 0-indexed
}

// splitByAST parses markdown into a goldmark AST and groups top-level nodes
// into structural segments. Headings act as section delimiters: each heading
// and all following non-heading nodes form one segment. Without headings,
// consecutive blocks are individual segments, with intro+list merging.
func splitByAST(source string) []string {
	src := []byte(source)
	sourceLines := strings.Split(source, "\n")
	md := goldmark.New()
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)

	// Build line offset index for byte→line conversion.
	lineOffsets := buildLineOffsets(src)

	// Collect top-level blocks with their source line ranges.
	var blocks []topLevelBlock
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		b := nodeToBlock(child, src, lineOffsets)
		if b.startLine >= 0 {
			blocks = append(blocks, b)
		}
	}

	if len(blocks) == 0 {
		return []string{source}
	}

	// Fix any gaps between blocks (blank lines, thematic breaks, etc.)
	// by assigning unaccounted source lines to surrounding blocks.
	blocks = fillGaps(blocks, len(sourceLines)-1)

	// Group into segments.
	hasHeadings := false
	for _, b := range blocks {
		if b.kind == ast.KindHeading {
			hasHeadings = true
			break
		}
	}

	var rawSegments []string
	if hasHeadings {
		rawSegments = groupByHeadings(blocks, sourceLines)
	} else {
		rawSegments = groupByBlocks(blocks, sourceLines)
		rawSegments = mergeIntroWithList(rawSegments)
	}

	return rawSegments
}

// buildLineOffsets returns the byte offset of the start of each line.
func buildLineOffsets(src []byte) []int {
	offsets := []int{0}
	for i, b := range src {
		if b == '\n' && i+1 <= len(src) {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// byteToLine converts a byte offset to a 0-indexed line number.
func byteToLine(offsets []int, bytePos int) int {
	// Binary search for the line containing bytePos.
	lo, hi := 0, len(offsets)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if offsets[mid] <= bytePos {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// nodeToBlock maps a top-level AST node to its source line range.
func nodeToBlock(node ast.Node, src []byte, lineOffsets []int) topLevelBlock {
	minByte := len(src)
	maxByte := 0
	collectBlockBytes(node, &minByte, &maxByte)

	if minByte >= maxByte {
		// Node with no content lines (e.g. ThematicBreak).
		// These will be handled by fillGaps.
		return topLevelBlock{kind: node.Kind(), startLine: -1, endLine: -1}
	}

	startLine := byteToLine(lineOffsets, minByte)
	endLine := byteToLine(lineOffsets, maxByte-1)

	// Extend for fenced code blocks to include fence lines.
	if node.Kind() == ast.KindFencedCodeBlock {
		if startLine > 0 {
			startLine--
		}
		endLine++
	}

	return topLevelBlock{kind: node.Kind(), startLine: startLine, endLine: endLine}
}

// collectBlockBytes finds the min and max byte offsets across all block-level
// Lines() in a node tree. Inline nodes are skipped (they panic on Lines()).
func collectBlockBytes(node ast.Node, minByte, maxByte *int) {
	if node.Type() == ast.TypeInline {
		return
	}
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		if seg.Start < *minByte {
			*minByte = seg.Start
		}
		if seg.Stop > *maxByte {
			*maxByte = seg.Stop
		}
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		collectBlockBytes(child, minByte, maxByte)
	}
}

// fillGaps assigns unaccounted source lines to adjacent blocks.
// Lines between blocks are appended to the preceding block.
func fillGaps(blocks []topLevelBlock, maxLine int) []topLevelBlock {
	if len(blocks) == 0 {
		return blocks
	}

	result := make([]topLevelBlock, len(blocks))
	copy(result, blocks)

	// Extend each block's endLine to cover lines up to (but not including)
	// the next block's startLine.
	for i := 0; i < len(result)-1; i++ {
		nextStart := result[i+1].startLine
		if nextStart > result[i].endLine+1 {
			// There's a gap — extend current block to cover it.
			result[i].endLine = nextStart - 1
		}
	}

	// Extend last block to cover remaining lines.
	if result[len(result)-1].endLine < maxLine {
		result[len(result)-1].endLine = maxLine
	}

	return result
}

// groupByHeadings groups blocks by heading sections. Each heading and all
// following non-heading blocks form one segment.
func groupByHeadings(blocks []topLevelBlock, sourceLines []string) []string {
	var segments []string
	var sectionStart, sectionEnd int
	inSection := false

	flush := func() {
		if !inSection {
			return
		}
		seg := extractLines(sourceLines, sectionStart, sectionEnd)
		if seg != "" {
			segments = append(segments, seg)
		}
	}

	for _, b := range blocks {
		if b.kind == ast.KindHeading {
			flush()
			sectionStart = b.startLine
			sectionEnd = b.endLine
			inSection = true
		} else if inSection {
			sectionEnd = b.endLine
		} else {
			// Content before first heading.
			seg := extractLines(sourceLines, b.startLine, b.endLine)
			if seg != "" {
				segments = append(segments, seg)
			}
		}
	}
	flush()

	return segments
}

// groupByBlocks creates one segment per top-level block.
func groupByBlocks(blocks []topLevelBlock, sourceLines []string) []string {
	var segments []string
	for _, b := range blocks {
		seg := extractLines(sourceLines, b.startLine, b.endLine)
		if seg != "" {
			segments = append(segments, seg)
		}
	}
	return segments
}

// extractLines joins source lines [start, end] (inclusive) and trims whitespace.
func extractLines(sourceLines []string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end >= len(sourceLines) {
		end = len(sourceLines) - 1
	}
	if start > end {
		return ""
	}
	return strings.TrimSpace(strings.Join(sourceLines[start:end+1], "\n"))
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
