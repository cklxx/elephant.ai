package lark

import (
	"encoding/json"
	"regexp"
	"strings"
)

// postElement represents a single element in a Lark post message line.
type postElement struct {
	Tag   string   `json:"tag"`
	Text  string   `json:"text"`
	Href  string   `json:"href,omitempty"`
	Style []string `json:"style,omitempty"`
}

// postPayload is the JSON structure for a Lark "post" message.
type postPayload struct {
	ZhCN struct {
		Content [][]postElement `json:"content"`
	} `json:"zh_cn"`
}

type replyCardBlock struct {
	text    string
	isTable bool
}

// Markdown detection patterns.
var (
	mdBoldPattern    = regexp.MustCompile(`\*\*[^*]+\*\*`)
	mdHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	mdLinkPattern    = regexp.MustCompile(`\[[^\]]+\]\([^)]+\)`)
	mdCodeFence      = regexp.MustCompile("(?m)^```")
	mdInlineCode     = regexp.MustCompile("`[^`]+`")
	// mdTableSep matches a markdown table separator row: |---|---| or | --- | --- |
	mdTableSep = regexp.MustCompile(`(?m)^\|[\s:]*-{3,}[\s:]*(\|[\s:]*-{3,}[\s:]*)+\|?\s*$`)
)

// hasMarkdownPatterns returns true if text contains any Markdown patterns
// that benefit from rich text rendering.
func hasMarkdownPatterns(text string) bool {
	patterns := []*regexp.Regexp{mdBoldPattern, mdHeadingPattern, mdLinkPattern, mdCodeFence, mdInlineCode}
	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// hasTableSyntax returns true if text contains a Markdown table.
// A valid table requires a separator row (|---|---|) not inside a code fence.
func hasTableSyntax(text string) bool {
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if mdTableSep.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// smartContent inspects text for residual Markdown. If a table is detected,
// it returns an interactive card with plain-text table blocks so Feishu does
// not need to interpret pipe-table markdown; if other Markdown is found, it
// converts to a "post" message; otherwise returns a plain "text" message.
func smartContent(text string) (msgType string, content string) {
	if hasTableSyntax(text) {
		return "interactive", buildTableSafeCard(text)
	}
	if !hasMarkdownPatterns(text) {
		return "text", textContent(text)
	}
	text = renderOutgoingMentions(text)
	return "post", buildPostContent(text)
}

func buildTableSafeCard(text string) string {
	blocks := splitReplyCardBlocks(normalizeOutgoingMentionsForCardText(text))
	elements := make([]any, 0, len(blocks))
	for _, block := range blocks {
		rendered := renderReplyCardBlock(block)
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "plain_text",
				"content": rendered,
			},
		})
	}
	if len(elements) == 0 {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "plain_text",
				"content": "",
			},
		})
	}
	return buildLarkCard("", "blue", elements)
}

func renderReplyCardBlock(block replyCardBlock) string {
	if block.isTable {
		return renderMarkdownTableAsPlainText(block.text)
	}
	return flattenPostContentToText(buildPostContent(block.text))
}

func splitReplyCardBlocks(text string) []replyCardBlock {
	lines := strings.Split(text, "\n")
	blocks := make([]replyCardBlock, 0)
	proseLines := make([]string, 0)
	inCodeBlock := false

	flushProse := func() {
		if len(proseLines) == 0 {
			return
		}
		blockText := strings.TrimSpace(strings.Join(proseLines, "\n"))
		if blockText != "" {
			blocks = append(blocks, replyCardBlock{text: blockText})
		}
		proseLines = proseLines[:0]
	}

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			proseLines = append(proseLines, line)
			i++
			continue
		}
		if !inCodeBlock && isMarkdownTableStart(lines, i) {
			flushProse()
			start := i
			i += 2
			for i < len(lines) && isMarkdownTableRow(lines[i]) {
				i++
			}
			tableText := strings.TrimSpace(strings.Join(lines[start:i], "\n"))
			if tableText != "" {
				blocks = append(blocks, replyCardBlock{text: tableText, isTable: true})
			}
			continue
		}
		proseLines = append(proseLines, line)
		i++
	}
	flushProse()
	return blocks
}

func isMarkdownTableStart(lines []string, idx int) bool {
	if idx+1 >= len(lines) {
		return false
	}
	header := strings.TrimSpace(lines[idx])
	separator := strings.TrimSpace(lines[idx+1])
	return strings.Contains(header, "|") && mdTableSep.MatchString(separator)
}

func isMarkdownTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && strings.Contains(trimmed, "|")
}

func renderMarkdownTableAsPlainText(table string) string {
	lines := strings.Split(strings.TrimSpace(table), "\n")
	if len(lines) < 2 {
		return strings.TrimSpace(table)
	}
	rendered := make([]string, 0, len(lines)-1)
	for idx, line := range lines {
		if idx == 1 {
			continue
		}
		cells := parseMarkdownTableRow(line)
		if len(cells) == 0 {
			continue
		}
		rendered = append(rendered, strings.Join(cells, " | "))
	}
	return strings.Join(rendered, "\n")
}

func parseMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	rawCells := splitMarkdownTableCells(trimmed)
	cells := make([]string, 0, len(rawCells))
	for _, cell := range rawCells {
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells
}

func splitMarkdownTableCells(row string) []string {
	var (
		cells   []string
		current strings.Builder
		escaped bool
	)
	for _, r := range row {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	cells = append(cells, current.String())
	return cells
}

func normalizeOutgoingMentionsForCardText(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return outgoingMentionPattern.ReplaceAllStringFunc(text, func(raw string) string {
		sub := outgoingMentionPattern.FindStringSubmatch(raw)
		if len(sub) != 3 {
			return raw
		}
		name := strings.TrimSpace(sub[1])
		userID := strings.TrimSpace(sub[2])
		switch {
		case userID == "all" && (name == "" || strings.EqualFold(name, "all")):
			return "@所有人"
		case name != "":
			return "@" + name
		default:
			return raw
		}
	})
}

func extractCardText(cardJSON string) string {
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		return ""
	}
	elements, ok := card["elements"].([]any)
	if !ok || len(elements) == 0 {
		return ""
	}
	parts := make([]string, 0, len(elements))
	for _, el := range elements {
		elem, ok := el.(map[string]any)
		if !ok {
			continue
		}
		switch tag, _ := elem["tag"].(string); tag {
		case "markdown":
			if content, _ := elem["content"].(string); strings.TrimSpace(content) != "" {
				parts = append(parts, content)
			}
		case "div":
			textNode, ok := elem["text"].(map[string]any)
			if !ok {
				continue
			}
			if content, _ := textNode["content"].(string); strings.TrimSpace(content) != "" {
				parts = append(parts, content)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// buildPostContent converts Markdown-flavored text into a Lark post JSON payload.
func buildPostContent(text string) string {
	lines := strings.Split(text, "\n")
	var content [][]postElement
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code fence toggle.
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Inside code block: preserve as-is.
		if inCodeBlock {
			content = append(content, []postElement{{Tag: "text", Text: line}})
			continue
		}

		// Empty line: preserve as blank separator.
		if trimmed == "" {
			content = append(content, []postElement{{Tag: "text", Text: ""}})
			continue
		}

		// Heading: strip #s and render bold.
		if loc := mdHeadingPattern.FindStringIndex(line); loc != nil {
			heading := strings.TrimSpace(line[loc[1]:])
			if heading != "" {
				content = append(content, []postElement{{
					Tag:   "text",
					Text:  heading,
					Style: []string{"bold"},
				}})
			}
			continue
		}

		// Unordered list: convert to bullet.
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			item := strings.TrimSpace(trimmed[2:])
			elems := convertInlineMarkdown("  • " + item)
			content = append(content, elems)
			continue
		}

		// Blockquote.
		if strings.HasPrefix(trimmed, "> ") {
			quote := strings.TrimSpace(trimmed[2:])
			elems := convertInlineMarkdown("｜" + quote)
			content = append(content, elems)
			continue
		}

		// Regular line with inline markdown.
		elems := convertInlineMarkdown(line)
		content = append(content, elems)
	}

	var payload postPayload
	payload.ZhCN.Content = content
	data, _ := json.Marshal(payload)
	return string(data)
}

func flattenPostContentToText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	var payload postPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return ""
	}
	lines := make([]string, 0, len(payload.ZhCN.Content))
	for _, line := range payload.ZhCN.Content {
		var sb strings.Builder
		for _, el := range line {
			switch el.Tag {
			case "a":
				label := strings.TrimSpace(el.Text)
				href := strings.TrimSpace(el.Href)
				switch {
				case label != "" && href != "":
					sb.WriteString(label + " (" + href + ")")
				case label != "":
					sb.WriteString(label)
				case href != "":
					sb.WriteString(href)
				}
			default:
				sb.WriteString(el.Text)
			}
		}
		lines = append(lines, sb.String())
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// convertInlineMarkdown converts inline Markdown (bold, italic, inline code, links)
// within a single line into a slice of postElement.
func convertInlineMarkdown(line string) []postElement {
	type segment struct {
		text  string
		style []string
		href  string
		tag   string
	}

	var segments []segment
	remaining := line

	for remaining != "" {
		// Find the earliest inline pattern.
		boldIdx := mdBoldPattern.FindStringIndex(remaining)
		linkIdx := mdLinkPattern.FindStringIndex(remaining)
		codeIdx := mdInlineCode.FindStringIndex(remaining)

		// Pick the earliest match.
		type match struct {
			kind  string
			start int
			end   int
		}
		var earliest *match

		if boldIdx != nil {
			earliest = &match{"bold", boldIdx[0], boldIdx[1]}
		}
		if linkIdx != nil && (earliest == nil || linkIdx[0] < earliest.start) {
			earliest = &match{"link", linkIdx[0], linkIdx[1]}
		}
		if codeIdx != nil && (earliest == nil || codeIdx[0] < earliest.start) {
			earliest = &match{"code", codeIdx[0], codeIdx[1]}
		}

		if earliest == nil {
			// No more patterns.
			segments = append(segments, segment{text: remaining, tag: "text"})
			break
		}

		// Text before the match.
		if earliest.start > 0 {
			segments = append(segments, segment{text: remaining[:earliest.start], tag: "text"})
		}

		raw := remaining[earliest.start:earliest.end]
		switch earliest.kind {
		case "bold":
			inner := raw[2 : len(raw)-2]
			segments = append(segments, segment{text: inner, style: []string{"bold"}, tag: "text"})
		case "link":
			// Parse [text](url)
			bracketEnd := strings.Index(raw, "](")
			linkText := raw[1:bracketEnd]
			linkURL := raw[bracketEnd+2 : len(raw)-1]
			segments = append(segments, segment{text: linkText, href: linkURL, tag: "a"})
		case "code":
			inner := strings.Trim(raw, "`")
			segments = append(segments, segment{text: inner, style: []string{"bold"}, tag: "text"})
		}

		remaining = remaining[earliest.end:]
	}

	var elems []postElement
	for _, seg := range segments {
		if seg.text == "" && seg.href == "" {
			continue
		}
		el := postElement{Tag: seg.tag, Text: seg.text}
		if len(seg.style) > 0 {
			el.Style = seg.style
		}
		if seg.href != "" {
			el.Href = seg.href
		}
		elems = append(elems, el)
	}

	if len(elems) == 0 {
		return []postElement{{Tag: "text", Text: line}}
	}
	return elems
}
