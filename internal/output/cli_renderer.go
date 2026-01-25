package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"alex/internal/agent/domain"
	"alex/internal/agent/domain/formatter"
	"alex/internal/agent/types"

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

type CLIRenderer struct {
	// verbose controls the DETAIL LEVEL of output, NOT whether to show output:
	// - verbose=false (default): Show compact previews (e.g., "150 lines", "12 matches")
	// - verbose=true: Show full tool output (first 500 chars, full args, etc.)
	//
	// Note: Whether to show output at all is controlled by OutputContext.Level:
	// - LevelCore: Always show tool calls
	// - LevelSubagent/LevelParallel: Hide tool details, show progress summary only
	verbose    bool
	formatter  *formatter.ToolFormatter
	mdRenderer MarkdownRenderer
}

const nonVerbosePreviewLimit = 80

// NewCLIRenderer creates a new CLI renderer
// verbose=true enables detailed output (full args, more content preview)
// verbose=false shows compact output (tool name + brief summary)
func NewCLIRenderer(verbose bool) *CLIRenderer {
	return NewCLIRendererWithMarkdown(verbose, nil)
}

// NewCLIRendererWithMarkdown allows tests to supply a lightweight markdown renderer.
func NewCLIRendererWithMarkdown(verbose bool, md MarkdownRenderer) *CLIRenderer {
	renderer := &CLIRenderer{
		verbose:   verbose,
		formatter: formatter.NewToolFormatter(),
	}

	if md != nil {
		renderer.mdRenderer = md
		return renderer
	}

	// Set lipgloss to use stdout for color detection only when using the default renderer.
	lipgloss.SetColorProfile(lipgloss.NewRenderer(os.Stdout).ColorProfile())

	renderer.mdRenderer = buildDefaultMarkdownRenderer()

	return renderer
}

func buildDefaultMarkdownRenderer() MarkdownRenderer {
	profile := lipgloss.NewRenderer(os.Stdout).ColorProfile()
	return newMarkdownHighlighter(profile)
}

// Target returns the output target
func (r *CLIRenderer) Target() OutputTarget {
	return TargetCLI
}

// RenderToolCallStart renders tool call start with hierarchy awareness
func (r *CLIRenderer) RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string {
	// Hide tool calls for subagents (they show progress summary instead)
	if ctx.Level == types.LevelSubagent || ctx.Level == types.LevelParallel {
		return ""
	}
	if isConversationalTool(toolName) {
		return ""
	}

	// Core agent: always show tool calls (concise or verbose format)
	// Determine indentation based on hierarchy
	indent := ""
	if ctx.Level == types.LevelSubagent {
		indent = "  "
	}

	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)
	toolNameStyle := lipgloss.NewStyle().Bold(true)

	presentation := r.formatter.PrepareArgs(toolName, args)
	spinner := nextSpinnerFrame()

	if presentation.ShouldDisplay && presentation.InlinePreview != "" {
		preview := presentation.InlinePreview
		if !r.verbose {
			preview = truncateInlinePreview(preview, nonVerbosePreviewLimit)
		}
		return fmt.Sprintf("%s%s %s(%s)\n", indent, spinnerStyle.Render(spinner), toolNameStyle.Render(toolName), preview)
	}

	return fmt.Sprintf("%s%s %s\n", indent, spinnerStyle.Render(spinner), toolNameStyle.Render(toolName))
}

func truncateInlinePreview(preview string, limit int) string {
	if limit <= 0 {
		return preview
	}

	if utf8.RuneCountInString(preview) <= limit {
		return preview
	}

	runes := []rune(preview)
	if len(runes) <= limit {
		return preview
	}

	if limit == 1 {
		return string(runes[0])
	}

	return string(runes[:limit-1]) + "…"
}

func nextSpinnerFrame() string {
	frames := []string{"|", "/", "-", "\\"}
	idx := time.Now().UnixNano() % int64(len(frames))
	return frames[idx]
}

func isConversationalTool(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "plan", "clearify", "claify", "request_user":
		return true
	default:
		return false
	}
}

func truncateWithEllipsis(preview string, limit int) string {
	if limit <= 0 {
		return preview
	}

	runes := []rune(preview)
	if len(runes) <= limit {
		return preview
	}

	ellipsis := "..."
	if limit <= len(ellipsis) {
		return string(runes[:limit])
	}

	return string(runes[:limit-len(ellipsis)]) + ellipsis
}

// RenderToolCallComplete renders tool call completion with hierarchy and category awareness
func (r *CLIRenderer) RenderToolCallComplete(ctx *types.OutputContext, toolName string, result string, err error, duration time.Duration) string {
	// Hide tool calls for subagents (they show progress summary instead)
	if ctx.Level == types.LevelSubagent || ctx.Level == types.LevelParallel {
		return ""
	}

	// Core agent: always show tool results (concise or verbose format)
	// Determine indentation based on hierarchy
	indent := ""
	if ctx.Level == types.LevelSubagent {
		indent = "  "
	}

	if isConversationalTool(toolName) {
		if err != nil {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
			return fmt.Sprintf("%s\n", errStyle.Render(fmt.Sprintf("✗ %v", err)))
		}
		content := strings.TrimSpace(result)
		if content == "" {
			return ""
		}
		return r.renderMarkdown(content)
	}

	if err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		return fmt.Sprintf("%s  %s\n", indent, errStyle.Render(fmt.Sprintf("✗ %s failed: %v", toolName, err)))
	}

	// Smart display based on tool category and hierarchy
	return r.formatToolOutput(ctx, toolName, result, indent)
}

// RenderTaskStart renders task start metadata for immediate CLI feedback
func (r *CLIRenderer) RenderTaskStart(ctx *types.OutputContext, task string) string {
	// Silent start - don't show "▶ Start" header
	return ""
}

// RenderTaskComplete renders task completion
func (r *CLIRenderer) RenderTaskComplete(ctx *types.OutputContext, result *domain.TaskResult) string {
	// Don't show completion for subagents (they show progress instead)
	if ctx.Level == types.LevelSubagent {
		return ""
	}

	statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green

	var output strings.Builder

	if r.verbose {
		output.WriteString(fmt.Sprintf("\n%s\n", statsStyle.Render(fmt.Sprintf("✓ Task completed in %d iterations", result.Iterations))))
		output.WriteString(fmt.Sprintf("%s\n\n", lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")).Render(fmt.Sprintf("Tokens used: %d", result.TokensUsed))))
	} else {
		output.WriteString(fmt.Sprintf("\n%s\n\n", statsStyle.Render(fmt.Sprintf("✓ Done | %d iterations | %d tokens", result.Iterations, result.TokensUsed))))
	}

	// Render markdown answer
	if result.Answer != "" {
		rendered := r.renderMarkdown(result.Answer)
		output.WriteString(rendered)
		if !strings.HasSuffix(rendered, "\n") {
			output.WriteString("\n")
		}
	}

	return output.String()
}

// RenderError renders an error with hierarchy awareness
func (r *CLIRenderer) RenderError(ctx *types.OutputContext, phase string, err error) string {
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true)

	indent := ""
	if ctx.Level == types.LevelSubagent {
		indent = "  "
	}

	return fmt.Sprintf("\n%s%s\n", indent, errStyle.Render(fmt.Sprintf("✗ Error in %s: %v", phase, err)))
}

// RenderSubagentProgress renders subagent progress with proper indentation
func (r *CLIRenderer) RenderSubagentProgress(ctx *types.OutputContext, completed, total int, tokens, toolCalls int) string {
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	progressText := fmt.Sprintf("  ✓ [%d/%d] Task %d | %d tokens | %d tools",
		completed, total, completed, tokens, toolCalls)
	return grayStyle.Render(progressText) + "\n"
}

// RenderSubagentComplete renders subagent completion summary
func (r *CLIRenderer) RenderSubagentComplete(ctx *types.OutputContext, total, success, failed int, tokens, toolCalls int) string {
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	summaryText := fmt.Sprintf("  ━━━ Completed: %d/%d tasks | Total: %d tokens, %d tool calls",
		success, total, tokens, toolCalls)
	return grayStyle.Render(summaryText) + "\n\n"
}

// formatToolOutput formats tool output based on tool category and hierarchy
func (r *CLIRenderer) formatToolOutput(ctx *types.OutputContext, toolName, result string, indent string) string {
	// Use brighter gray (#808080) that works on both light and dark backgrounds
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	category := CategorizeToolName(toolName)

	switch category {
	case types.CategoryFile:
		return r.formatFileOutput(toolName, result, indent, grayStyle)
	case types.CategorySearch:
		return r.formatSearchOutput(toolName, result, indent, grayStyle)
	case types.CategoryShell, types.CategoryExecution:
		return r.formatExecutionOutput(toolName, result, indent, grayStyle)
	case types.CategoryWeb:
		return r.formatWebOutput(toolName, result, indent, grayStyle)
	case types.CategoryTask:
		return r.formatTaskOutput(toolName, result, indent, grayStyle)
	case types.CategoryReasoning:
		return r.formatReasoningOutput(result, indent, grayStyle)
	default:
		cleaned := filterSystemReminders(result)
		preview := truncateWithEllipsis(cleaned, 80)
		return fmt.Sprintf("%s  %s\n", indent, grayStyle.Render("→ "+preview))
	}
}

// Category-specific formatters

func (r *CLIRenderer) formatFileOutput(toolName, result, indent string, style lipgloss.Style) string {
	// Clean system reminders
	cleaned := filterSystemReminders(result)

	switch toolName {
	case "file_read":
		lines := strings.Count(cleaned, "\n")
		return fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d lines read", lines)))
	case "file_write", "file_edit":
		if strings.Contains(cleaned, "Success") || strings.Contains(cleaned, "written") {
			return fmt.Sprintf("%s  %s\n", indent, style.Render("→ ✓ file written"))
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	case "list_files":
		return r.formatListFiles(cleaned, indent, style)
	default:
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	}
}

func (r *CLIRenderer) formatSearchOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	lines := strings.Split(strings.TrimSpace(cleaned), "\n")
	matchCount := len(lines)
	if cleaned == "" {
		matchCount = 0
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d matches", matchCount))))

	// In verbose mode, show first few matches
	if r.verbose && matchCount > 0 && matchCount <= 5 {
		for _, line := range lines {
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
	} else if r.verbose && matchCount > 5 {
		for i := 0; i < 3; i++ {
			if lines[i] != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(lines[i])))
			}
		}
		output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", matchCount-3))))
	}

	return output.String()
}

func (r *CLIRenderer) formatExecutionOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)

	if toolName == "bash" {
		if formatted, ok := r.formatBashExecutionOutput(cleaned, indent, style); ok {
			return formatted
		}
	}

	// Show execution output with proper indentation
	if r.verbose {
		// In verbose mode, show more output
		lines := strings.Split(strings.TrimSpace(cleaned), "\n")
		var output strings.Builder
		output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ execution output:")))
		for i, line := range lines {
			if i >= 10 {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... (%d more lines)", len(lines)-10))))
				break
			}
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
		return output.String()
	}

	// Concise mode: just show summary
	preview := truncateWithEllipsis(cleaned, 100)
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
}

func (r *CLIRenderer) formatBashExecutionOutput(result, indent string, style lipgloss.Style) (string, bool) {
	type bashPayload struct {
		Command  string `json:"command"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode *int   `json:"exit_code"`
	}

	var payload bashPayload
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return "", false
	}

	stdout := strings.TrimRight(payload.Stdout, "\n")
	stderr := strings.TrimRight(payload.Stderr, "\n")
	exitCode := 0
	if payload.ExitCode != nil {
		exitCode = *payload.ExitCode
	}

	var summaryParts []string
	summaryParts = append(summaryParts, fmt.Sprintf("exit %d", exitCode))

	trimmedStdout := strings.TrimSpace(stdout)
	if trimmedStdout != "" {
		stdoutLines := countLines(trimmedStdout)
		if stdoutLines == 1 && utf8.RuneCountInString(trimmedStdout) <= 80 {
			summaryParts = append(summaryParts, trimmedStdout)
		} else {
			summaryParts = append(summaryParts, fmt.Sprintf("stdout %d %s", stdoutLines, pluralize("line", stdoutLines)))
		}
	} else {
		summaryParts = append(summaryParts, "stdout empty")
	}

	trimmedStderr := strings.TrimSpace(stderr)
	if trimmedStderr != "" {
		stderrLines := countLines(trimmedStderr)
		if stderrLines == 1 && utf8.RuneCountInString(trimmedStderr) <= 80 {
			summaryParts = append(summaryParts, fmt.Sprintf("stderr: %s", trimmedStderr))
		} else {
			summaryParts = append(summaryParts, fmt.Sprintf("stderr %d %s", stderrLines, pluralize("line", stderrLines)))
		}
	}

	var output strings.Builder
	fmt.Fprintf(&output, "%s  %s\n", indent, style.Render("→ "+strings.Join(summaryParts, ", ")))

	if r.verbose {
		if stdout != "" {
			r.writeVerboseStream(&output, indent, style, "stdout", stdout)
		}
		if stderr != "" {
			r.writeVerboseStream(&output, indent, style, "stderr", stderr)
		}
	}

	return output.String(), true
}

func (r *CLIRenderer) writeVerboseStream(builder *strings.Builder, indent string, style lipgloss.Style, label string, content string) {
	fmt.Fprintf(builder, "%s    %s\n", indent, style.Render(label+":"))
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i >= 10 {
			fmt.Fprintf(builder, "%s      %s\n", indent, style.Render(fmt.Sprintf("... (%d more lines)", len(lines)-10)))
			break
		}
		fmt.Fprintf(builder, "%s      %s\n", indent, style.Render(line))
	}
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func (r *CLIRenderer) formatWebOutput(toolName, result, indent string, style lipgloss.Style) string {
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ ✓ fetched"))
}

func (r *CLIRenderer) formatTaskOutput(toolName, result, indent string, style lipgloss.Style) string {
	// Clean system reminders from output
	cleaned := filterSystemReminders(result)

	// For todo tools, format the task list nicely
	if toolName == "todo_update" || toolName == "todo_read" {
		return r.formatTodoList(cleaned, indent, style)
	}

	// Other task tools: show cleaned result
	lines := strings.Split(strings.TrimSpace(cleaned), "\n")
	var output strings.Builder
	for _, line := range lines {
		if line != "" {
			output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(line)))
		}
	}
	return output.String()
}

func (r *CLIRenderer) formatReasoningOutput(result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	preview := truncateWithEllipsis(cleaned, 100)
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
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

// filterSystemReminders removes <system-reminder> tags from output
func filterSystemReminders(content string) string {
	lines := strings.Split(content, "\n")
	var filtered []string
	inReminder := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<system-reminder>") {
			inReminder = true
			if strings.HasSuffix(trimmed, "</system-reminder>") {
				inReminder = false
			}
			continue
		}
		if strings.HasSuffix(trimmed, "</system-reminder>") {
			inReminder = false
			continue
		}
		if !inReminder {
			filtered = append(filtered, line)
		}
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

// formatTodoList formats todo list output with proper indentation
func (r *CLIRenderer) formatTodoList(content, indent string, style lipgloss.Style) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var output strings.Builder

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Add double indent for all lines (tool output should be indented)
		trimmed := strings.TrimSpace(line)
		output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(trimmed)))
	}

	return output.String()
}

// formatListFiles formats file list with count summary and optional preview
func (r *CLIRenderer) formatListFiles(content, indent string, style lipgloss.Style) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	fileCount := len(lines)
	if content == "" {
		fileCount = 0
	}

	var output strings.Builder

	// Show count summary
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d files/directories", fileCount))))

	// In verbose mode, show first few files
	if r.verbose && fileCount > 0 && fileCount <= 10 {
		for _, line := range lines {
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
	} else if r.verbose && fileCount > 10 {
		for i := 0; i < 5; i++ {
			if lines[i] != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(lines[i])))
			}
		}
		output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", fileCount-5))))
	}

	return output.String()
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
