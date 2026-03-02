package lark

import (
	"encoding/json"
	"regexp"
	"strings"
)

// postElement represents a single element in a Lark post message line.
type postElement struct {
	Tag   string   `json:"tag"`
	Text  string   `json:"text,omitempty"`
	Href  string   `json:"href,omitempty"`
	Style []string `json:"style,omitempty"`
}

// postPayload is the JSON structure for a Lark "post" message.
type postPayload struct {
	ZhCN struct {
		Content [][]postElement `json:"content"`
	} `json:"zh_cn"`
}

// Markdown detection patterns.
var (
	mdBoldPattern    = regexp.MustCompile(`\*\*[^*]+\*\*`)
	mdHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	mdLinkPattern    = regexp.MustCompile(`\[[^\]]+\]\([^)]+\)`)
	mdCodeFence      = regexp.MustCompile("(?m)^```")
	mdInlineCode     = regexp.MustCompile("`[^`]+`")
)

// hasMarkdownPatterns returns true if text contains 2+ distinct Markdown patterns,
// suggesting the LLM ignored the plain-text formatting instruction.
func hasMarkdownPatterns(text string) bool {
	patterns := []*regexp.Regexp{mdBoldPattern, mdHeadingPattern, mdLinkPattern, mdCodeFence, mdInlineCode}
	count := 0
	for _, p := range patterns {
		if p.MatchString(text) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// smartContent inspects text for residual Markdown. If detected, it converts
// to a Lark "post" message; otherwise returns a plain "text" message.
func smartContent(text string) (msgType string, content string) {
	if !hasMarkdownPatterns(text) {
		return "text", textContent(text)
	}
	text = renderOutgoingMentions(text)
	return "post", buildPostContent(text)
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
