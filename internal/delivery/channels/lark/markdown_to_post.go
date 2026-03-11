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

var (
	mdBoldPattern    = regexp.MustCompile(`\*\*[^*]+\*\*`)
	mdHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	mdLinkPattern    = regexp.MustCompile(`\[[^\]]+\]\([^)]+\)`)
	mdCodeFence      = regexp.MustCompile("(?m)^```")
	mdInlineCode     = regexp.MustCompile("`[^`]+`")
	mdTableSep       = regexp.MustCompile(`(?m)^\|[\s:]*-{3,}[\s:]*(\|[\s:]*-{3,}[\s:]*)+\|?\s*$`)
)

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
// it returns an interactive card using richer card elements; if other Markdown
// is found, it converts to a "post" message; otherwise returns a plain "text"
// message.
func smartContent(text string) (msgType string, content string) {
	if hasTableSyntax(text) {
		text = renderOutgoingMentions(text)
		return "interactive", buildContentCard(text)
	}
	if !hasMarkdownPatterns(text) {
		return "text", textContent(text)
	}
	text = renderOutgoingMentions(text)
	return "post", buildPostContent(text)
}

// buildContentCard wraps mixed markdown/table content in a Lark interactive card.
// Markdown tables are lifted into card table components so replies render
// reliably in Feishu instead of relying on markdown table support.
func buildContentCard(text string) string {
	return buildLarkCard("", "blue", buildCardElementsFromMarkdown(text))
}

func buildCardElementsFromMarkdown(text string) []any {
	text = strings.TrimSpace(text)
	if text == "" {
		return []any{map[string]any{"tag": "markdown", "content": ""}}
	}

	lines := strings.Split(text, "\n")
	var elements []any
	var mdBuf []string
	flushMarkdown := func() {
		md := strings.TrimSpace(strings.Join(mdBuf, "\n"))
		if md != "" {
			elements = append(elements, map[string]any{
				"tag":     "markdown",
				"content": md,
			})
		}
		mdBuf = nil
	}

	inCodeBlock := false
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			mdBuf = append(mdBuf, lines[i])
			i++
			continue
		}
		if !inCodeBlock && i+1 < len(lines) && isTableHeaderLine(lines[i]) && mdTableSep.MatchString(strings.TrimSpace(lines[i+1])) {
			flushMarkdown()
			tableLines := []string{lines[i], lines[i+1]}
			i += 2
			for i < len(lines) {
				row := strings.TrimSpace(lines[i])
				if row == "" || !strings.Contains(row, "|") || mdTableSep.MatchString(row) {
					break
				}
				tableLines = append(tableLines, lines[i])
				i++
			}
			elements = append(elements, buildTableElement(tableLines))
			continue
		}
		mdBuf = append(mdBuf, lines[i])
		i++
	}
	flushMarkdown()
	if len(elements) == 0 {
		return []any{map[string]any{"tag": "markdown", "content": text}}
	}
	return elements
}

func isTableHeaderLine(line string) bool {
	cells := parseMarkdownTableRow(line)
	return len(cells) >= 2
}

func parseMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func buildTableElement(lines []string) map[string]any {
	headers := parseMarkdownTableRow(lines[0])
	columns := make([]any, 0, len(headers))
	for idx, header := range headers {
		name := "col_" + strings.TrimSpace(strings.ReplaceAll(strings.ToLower(header), " ", "_"))
		if name == "col_" {
			name = "col"
		}
		name = name + "_" + strings.TrimSpace(strings.Join([]string{string(rune('0' + (idx % 10)))}, ""))
		columns = append(columns, map[string]any{
			"name":             name,
			"display_name":     header,
			"data_type":        "markdown",
			"horizontal_align": "left",
			"vertical_align":   "top",
			"width":            "auto",
		})
	}
	rows := make([]any, 0, max(0, len(lines)-2))
	for _, line := range lines[2:] {
		cells := parseMarkdownTableRow(line)
		if len(cells) == 0 {
			continue
		}
		row := map[string]any{}
		for idx := range headers {
			key := columns[idx].(map[string]any)["name"].(string)
			if idx < len(cells) {
				row[key] = cells[idx]
			} else {
				row[key] = ""
			}
		}
		rows = append(rows, row)
	}
	return map[string]any{
		"tag":        "table",
		"page_size":  len(rows),
		"row_height": "low",
		"header_style": map[string]any{
			"text_align":       "left",
			"text_size":        "normal",
			"background_style": "grey",
			"text_color":       "default",
			"bold":             true,
			"lines":            1,
		},
		"columns": columns,
		"rows":    rows,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extractCardMarkdown returns the markdown content from a card JSON built by
// buildContentCard. Returns empty string if the JSON cannot be parsed.
func extractCardMarkdown(cardJSON string) string {
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		return ""
	}
	elements, ok := card["elements"].([]any)
	if !ok || len(elements) == 0 {
		return ""
	}
	var blocks []string
	for _, el := range elements {
		elem, ok := el.(map[string]any)
		if !ok {
			continue
		}
		switch tag, _ := elem["tag"].(string); tag {
		case "markdown":
			if content, ok := elem["content"].(string); ok {
				blocks = append(blocks, content)
			}
		case "table":
			if md := flattenCardTableToMarkdown(elem); md != "" {
				blocks = append(blocks, md)
			}
		}
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func flattenCardTableToMarkdown(table map[string]any) string {
	colsRaw, ok := table["columns"].([]any)
	if !ok || len(colsRaw) == 0 {
		return ""
	}
	headers := make([]string, 0, len(colsRaw))
	keys := make([]string, 0, len(colsRaw))
	for _, colRaw := range colsRaw {
		col, ok := colRaw.(map[string]any)
		if !ok {
			continue
		}
		display, _ := col["display_name"].(string)
		key, _ := col["name"].(string)
		headers = append(headers, display)
		keys = append(keys, key)
	}
	if len(headers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("| " + strings.Join(seps, " | ") + " |")
	rowsRaw, _ := table["rows"].([]any)
	for _, rowRaw := range rowsRaw {
		row, ok := rowRaw.(map[string]any)
		if !ok {
			continue
		}
		vals := make([]string, len(keys))
		for i, key := range keys {
			if v, ok := row[key].(string); ok {
				vals[i] = v
			}
		}
		b.WriteString("\n| " + strings.Join(vals, " | ") + " |")
	}
	return b.String()
}

// buildPostContent converts Markdown-flavored text into a Lark post JSON payload.
func buildPostContent(text string) string {
	lines := strings.Split(text, "\n")
	var content [][]postElement
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			content = append(content, []postElement{{Tag: "text", Text: line}})
			continue
		}
		if trimmed == "" {
			content = append(content, []postElement{{Tag: "text", Text: ""}})
			continue
		}
		if loc := mdHeadingPattern.FindStringIndex(line); loc != nil {
			heading := strings.TrimSpace(line[loc[1]:])
			if heading != "" {
				content = append(content, []postElement{{Tag: "text", Text: heading, Style: []string{"bold"}}})
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			item := strings.TrimSpace(trimmed[2:])
			elems := convertInlineMarkdown("  • " + item)
			content = append(content, elems)
			continue
		}
		if strings.HasPrefix(trimmed, "> ") {
			quote := strings.TrimSpace(trimmed[2:])
			elems := convertInlineMarkdown("｜" + quote)
			content = append(content, elems)
			continue
		}
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
		boldIdx := mdBoldPattern.FindStringIndex(remaining)
		linkIdx := mdLinkPattern.FindStringIndex(remaining)
		codeIdx := mdInlineCode.FindStringIndex(remaining)

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
			segments = append(segments, segment{text: remaining, tag: "text"})
			break
		}
		if earliest.start > 0 {
			segments = append(segments, segment{text: remaining[:earliest.start], tag: "text"})
		}

		raw := remaining[earliest.start:earliest.end]
		switch earliest.kind {
		case "bold":
			inner := raw[2 : len(raw)-2]
			segments = append(segments, segment{text: inner, style: []string{"bold"}, tag: "text"})
		case "link":
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
