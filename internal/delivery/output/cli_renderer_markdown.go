package output

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// CLIRenderer renders output for CLI display with hierarchical context awareness
type MarkdownRenderer interface {
	Render(string) (string, error)
}

type StreamLineRenderer interface {
	RenderLine(string) string
	ResetStream()
}

func buildDefaultMarkdownRenderer(profile termenv.Profile) MarkdownRenderer {
	return newMarkdownHighlighter(profile)
}

func (r *CLIRenderer) renderMarkdown(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	if r.mdRenderer == nil {
		return content
	}

	rendered, err := r.mdRenderer.Render(content)
	if err != nil {
		return content
	}

	rendered = trimWhitespaceLineEdges(rendered)
	if rendered == "" {
		return ""
	}
	return strings.TrimRight(rendered, "\n") + "\n"
}

func (r *CLIRenderer) ResetMarkdownStreamState() {
	if streamRenderer, ok := r.mdRenderer.(StreamLineRenderer); ok {
		streamRenderer.ResetStream()
	}
}

// RenderMarkdownStreamChunk renders a fragment of markdown that may be part of a
// streamed response. The caller can request a trailing newline to preserve
// terminal formatting when a full line has been received.
func (r *CLIRenderer) RenderMarkdownStreamChunk(content string, ensureTrailingNewline bool) string {
	if strings.TrimSpace(content) == "" {
		if ensureTrailingNewline && content != "" && !strings.HasSuffix(content, "\n") {
			return content + "\n"
		}
		return content
	}

	// Streaming fragments are often tiny and do not represent valid markdown on
	// their own. Avoid full markdown parsing for every chunk to keep streaming
	// latency low; only apply lightweight styling for complete lines.
	if !ensureTrailingNewline {
		return content
	}

	if r.mdRenderer == nil {
		if ensureTrailingNewline && !strings.HasSuffix(content, "\n") {
			return content + "\n"
		}
		return content
	}

	if streamRenderer, ok := r.mdRenderer.(StreamLineRenderer); ok {
		return streamRenderer.RenderLine(content)
	}

	rendered, err := r.mdRenderer.Render(content)
	if err != nil {
		if ensureTrailingNewline && !strings.HasSuffix(content, "\n") {
			return content + "\n"
		}
		return content
	}

	rendered = trimWhitespaceLineEdges(rendered)
	rendered = strings.TrimRight(rendered, "\n")
	if ensureTrailingNewline {
		rendered += "\n"
	}
	return rendered
}

// trimWhitespaceLineEdges removes leading/trailing whitespace-only lines from
// rendered markdown to avoid extra blank lines in streaming output.
func trimWhitespaceLineEdges(rendered string) string {
	if rendered == "" {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	start := 0
	for start < len(lines) && isWhitespaceLine(lines[start]) {
		start++
	}
	end := len(lines)
	for end > start && isWhitespaceLine(lines[end-1]) {
		end--
	}
	if start == 0 && end == len(lines) {
		return rendered
	}
	return strings.Join(lines[start:end], "\n")
}

func isWhitespaceLine(line string) bool {
	return strings.TrimSpace(stripANSI(line)) == ""
}

func stripANSI(input string) string {
	if input == "" {
		return input
	}

	var out strings.Builder
	out.Grow(len(input))
	for i := 0; i < len(input); {
		if input[i] == 0x1b && i+1 < len(input) && input[i+1] == '[' {
			i += 2
			for i < len(input) {
				c := input[i]
				i++
				if c >= 0x40 && c <= 0x7e {
					break
				}
			}
			continue
		}
		out.WriteByte(input[i])
		i++
	}
	return out.String()
}

type markdownHighlighter struct {
	formatter chroma.Formatter
	style     *chroma.Style

	headingStyle lipgloss.Style
	bulletStyle  lipgloss.Style

	streamInCodeFence bool
	streamCodeLang    string
}

func newMarkdownHighlighter(profile termenv.Profile) *markdownHighlighter {
	formatter := formatters.NoOp
	switch profile {
	case termenv.TrueColor:
		formatter = formatters.TTY16m
	case termenv.ANSI256:
		formatter = formatters.TTY256
	case termenv.ANSI:
		formatter = formatters.TTY16
	case termenv.Ascii:
		formatter = formatters.NoOp
	}

	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	return &markdownHighlighter{
		formatter:    formatter,
		style:        style,
		headingStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5EA3FF")),
		bulletStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8AA4BF")),
	}
}

func (h *markdownHighlighter) Render(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	return h.renderMarkdown(content), nil
}

func (h *markdownHighlighter) RenderLine(line string) string {
	if line == "" {
		return line
	}

	hasNewline := strings.HasSuffix(line, "\n")
	trimmedLine := strings.TrimRight(line, "\n")
	trimmed := strings.TrimSpace(trimmedLine)

	if strings.HasPrefix(trimmed, "```") {
		if h.streamInCodeFence {
			h.streamInCodeFence = false
			h.streamCodeLang = ""
		} else {
			h.streamInCodeFence = true
			h.streamCodeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
		}

		if hasNewline {
			return "\n"
		}
		return ""
	}

	var rendered string
	if h.streamInCodeFence {
		rendered = h.highlightCode(trimmedLine, h.streamCodeLang)
	} else {
		rendered = h.renderTextLine(trimmedLine)
	}

	if hasNewline && !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}
	return rendered
}

func (h *markdownHighlighter) ResetStream() {
	h.streamInCodeFence = false
	h.streamCodeLang = ""
}

func (h *markdownHighlighter) renderMarkdown(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var output strings.Builder
	var codeLines []string
	inCodeFence := false
	codeLang := ""

	flushCode := func() {
		if len(codeLines) == 0 {
			codeLines = nil
			return
		}
		highlighted := h.highlightCode(strings.Join(codeLines, "\n"), codeLang)
		output.WriteString(highlighted)
		output.WriteString("\n")
		codeLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeFence {
				flushCode()
				inCodeFence = false
				codeLang = ""
			} else {
				inCodeFence = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			}
			continue
		}

		if inCodeFence {
			codeLines = append(codeLines, line)
			continue
		}

		output.WriteString(h.renderTextLine(line))
		output.WriteString("\n")
	}

	if inCodeFence {
		flushCode()
	}

	return strings.TrimRight(output.String(), "\n")
}

func (h *markdownHighlighter) renderTextLine(line string) string {
	if line == "" {
		return line
	}

	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return ""
	}
	indent := line[:len(line)-len(trimmed)]

	if strings.HasPrefix(trimmed, "#") {
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		text := strings.TrimSpace(trimmed[level:])
		if text == "" {
			return line
		}
		return indent + h.headingStyle.Render(text)
	}

	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		text := strings.TrimSpace(trimmed[2:])
		if text == "" {
			return line
		}
		return indent + h.bulletStyle.Render("•") + " " + text
	}

	if strings.HasPrefix(trimmed, "> ") {
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, "> "))
		return indent + lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("│ "+text)
	}

	return line
}

func (h *markdownHighlighter) highlightCode(code string, language string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}

	lang := strings.TrimSpace(language)
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(lang)
	}
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var buf strings.Builder
	if err := h.formatter.Format(&buf, h.style, iterator); err != nil {
		return code
	}
	return buf.String()
}
