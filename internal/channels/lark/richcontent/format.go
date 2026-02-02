package richcontent

import (
	"regexp"
	"strings"
)

// FormatMarkdown converts simple Markdown text to a Lark post JSON string.
// Supported Markdown constructs:
//   - **bold** or __bold__
//   - *italic* or _italic_
//   - [text](url) links
//   - `inline code`
//   - Newlines become paragraph breaks
//
// This is intentionally limited to inline formatting. Block-level constructs
// (headings, lists, blockquotes) are passed through as plain text.
func FormatMarkdown(md string) string {
	return FormatMarkdownWithTitle(md, "")
}

// FormatMarkdownWithTitle converts Markdown to a Lark post JSON string with
// a custom title.
func FormatMarkdownWithTitle(md, title string) string {
	lines := strings.Split(md, "\n")
	builder := NewPostBuilder(title)

	for i, line := range lines {
		if i > 0 {
			builder.NewLine()
		}
		parseMarkdownLine(builder, line)
	}

	return builder.Build()
}

// markdownPattern matches inline Markdown elements in priority order.
// Groups:
//
//	1: bold (**text** or __text__)
//	2: italic (*text* or _text_) — single delimiters
//	3: inline code (`code`)
//	4: link text [text]
//	5: link URL (url)
var markdownPattern = regexp.MustCompile(
	`\*\*(.+?)\*\*` + // bold **
		`|__(.+?)__` + // bold __
		"|\\*(.+?)\\*" + // italic *
		"|_(.+?)_" + // italic _
		"|`(.+?)`" + // inline code
		`|\[(.+?)\]\((.+?)\)`, // link
)

// parseMarkdownLine converts a single line of Markdown into PostBuilder
// elements appended to the current paragraph.
func parseMarkdownLine(b *PostBuilder, line string) {
	remaining := line
	for remaining != "" {
		loc := markdownPattern.FindStringSubmatchIndex(remaining)
		if loc == nil {
			// No more Markdown tokens; emit the rest as plain text.
			if remaining != "" {
				b.AddText(remaining)
			}
			break
		}

		// Emit any text before the match as plain text.
		if loc[0] > 0 {
			b.AddText(remaining[:loc[0]])
		}

		matches := markdownPattern.FindStringSubmatch(remaining)

		switch {
		case matches[1] != "":
			// Bold **text**
			b.AddBold(matches[1])
		case matches[2] != "":
			// Bold __text__
			b.AddBold(matches[2])
		case matches[3] != "":
			// Italic *text*
			b.AddItalic(matches[3])
		case matches[4] != "":
			// Italic _text_
			b.AddItalic(matches[4])
		case matches[5] != "":
			// Inline code `code`
			b.AddText(matches[5])
		case matches[6] != "" && matches[7] != "":
			// Link [text](url)
			b.AddLink(matches[6], matches[7])
		}

		remaining = remaining[loc[1]:]
	}
}

// FormatCodeBlock formats a code block for display in a Lark post message.
// It wraps the code in a PostBuilder with a code block element and returns
// the serialized post JSON.
func FormatCodeBlock(code, language string) string {
	return FormatCodeBlockWithTitle(code, language, "")
}

// FormatCodeBlockWithTitle formats a code block with a custom title.
func FormatCodeBlockWithTitle(code, language, title string) string {
	builder := NewPostBuilder(title)
	builder.AddCodeBlock(code, language)
	return builder.Build()
}

// FormatTable renders tabular data as a formatted Lark post message.
// This is a convenience wrapper around TableBuilder.
func FormatTable(headers []string, rows [][]string) string {
	return FormatTableText(headers, rows)
}
