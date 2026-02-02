package richcontent

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// TableBuilder constructs a formatted text table suitable for inclusion in
// Lark post messages. Since Lark's post message type does not natively
// support table elements, the builder renders an aligned text representation
// using fixed-width characters and pipes.
type TableBuilder struct {
	headers []string
	rows    [][]string
}

// NewTableBuilder creates a TableBuilder with the given column headers.
func NewTableBuilder(headers []string) *TableBuilder {
	h := make([]string, len(headers))
	copy(h, headers)
	return &TableBuilder{headers: h}
}

// AddRow appends a data row. If the row has fewer cells than headers, the
// missing cells are treated as empty strings. Extra cells beyond the header
// count are silently ignored.
func (tb *TableBuilder) AddRow(cells []string) *TableBuilder {
	row := make([]string, len(tb.headers))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		}
	}
	tb.rows = append(tb.rows, row)
	return tb
}

// Build renders the table as a formatted text string. The output uses pipes
// and dashes to create a readable ASCII table:
//
//	| Name   | Status |
//	|--------|--------|
//	| Task 1 | Done   |
//	| Task 2 | Active |
func (tb *TableBuilder) Build() string {
	if len(tb.headers) == 0 {
		return ""
	}

	colWidths := tb.computeColumnWidths()
	var sb strings.Builder

	// Header row.
	sb.WriteString(tb.formatRow(tb.headers, colWidths))
	sb.WriteByte('\n')

	// Separator row.
	sb.WriteString(tb.formatSeparator(colWidths))
	sb.WriteByte('\n')

	// Data rows.
	for _, row := range tb.rows {
		sb.WriteString(tb.formatRow(row, colWidths))
		sb.WriteByte('\n')
	}

	return strings.TrimRight(sb.String(), "\n")
}

// BuildPost renders the table as a Lark post JSON message. The table is
// placed inside a single text element so it preserves alignment.
func (tb *TableBuilder) BuildPost(title string) string {
	tableText := tb.Build()
	if tableText == "" {
		return NewPostBuilder(title).Build()
	}

	builder := NewPostBuilder(title)
	builder.AddText(tableText)
	return builder.Build()
}

// computeColumnWidths returns the maximum display width for each column,
// considering both headers and data rows.
func (tb *TableBuilder) computeColumnWidths() []int {
	widths := make([]int, len(tb.headers))
	for i, h := range tb.headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range tb.rows {
		for i, cell := range row {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

// formatRow formats a single row with pipe separators and padding.
func (tb *TableBuilder) formatRow(cells []string, widths []int) string {
	var sb strings.Builder
	sb.WriteByte('|')
	for i, cell := range cells {
		sb.WriteByte(' ')
		sb.WriteString(cell)
		padding := widths[i] - displayWidth(cell)
		for j := 0; j < padding; j++ {
			sb.WriteByte(' ')
		}
		sb.WriteString(" |")
	}
	return sb.String()
}

// formatSeparator creates the dash separator row between header and body.
func (tb *TableBuilder) formatSeparator(widths []int) string {
	var sb strings.Builder
	sb.WriteByte('|')
	for _, w := range widths {
		sb.WriteByte('-')
		for j := 0; j < w; j++ {
			sb.WriteByte('-')
		}
		sb.WriteString("-|")
	}
	return sb.String()
}

// displayWidth returns the visual display width of a string. For simplicity
// this counts UTF-8 rune count, which works well for Latin text. CJK
// characters would ideally be counted as double-width, but that level of
// precision is not critical for message display.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// FormatTableJSON is a convenience function that renders a table directly
// as a Lark post JSON string.
func FormatTableJSON(title string, headers []string, rows [][]string) string {
	tb := NewTableBuilder(headers)
	for _, row := range rows {
		tb.AddRow(row)
	}
	return tb.BuildPost(title)
}

// FormatTableText is a convenience function that renders a table as plain
// aligned text.
func FormatTableText(headers []string, rows [][]string) string {
	tb := NewTableBuilder(headers)
	for _, row := range rows {
		tb.AddRow(row)
	}
	return tb.Build()
}

// BuildPostElements returns the table formatted as Lark post JSON content
// paragraphs. This can be used to embed a table within a larger PostBuilder
// by converting the table to element arrays.
func (tb *TableBuilder) BuildPostElements() string {
	tableText := tb.Build()
	data, _ := json.Marshal(tableText)
	return string(data)
}
