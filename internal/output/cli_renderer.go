package output

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
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
	maxWidth   int
}

const nonVerbosePreviewLimit = 80

var toolDisplayNames = map[string]string{
	"a2ui_emit":                  "a2ui.emit",
	"artifact_manifest":          "artifact.manifest",
	"artifacts_delete":           "artifact.delete",
	"artifacts_list":             "artifact.list",
	"artifacts_write":            "artifact.write",
	"douyin_hot":                 "douyin.hot",
	"file_edit":                  "file.edit",
	"file_read":                  "file.read",
	"file_write":                 "file.write",
	"image_to_image":             "image.i2i",
	"list_files":                 "file.list",
	"pptx_from_images":           "pptx.images",
	"sandbox_browser":            "sandbox.browser",
	"sandbox_browser_dom":        "browser.dom",
	"sandbox_browser_info":       "browser.info",
	"sandbox_browser_screenshot": "browser.shot",
	"sandbox_code_execute":       "sandbox.exec",
	"sandbox_file_list":          "sandbox.list",
	"sandbox_file_read":          "sandbox.read",
	"sandbox_file_replace":       "sandbox.replace",
	"sandbox_file_search":        "sandbox.search",
	"sandbox_file_write":         "sandbox.write",
	"sandbox_shell_exec":         "sandbox.shell",
	"sandbox_write_attachment":   "sandbox.attach",
	"text_to_image":              "image.t2i",
	"video_generate":             "video.gen",
	"vision_analyze":             "vision.analyze",
	"web_fetch":                  "web.fetch",
	"web_search":                 "web.search",
}

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
		maxWidth:  0,
	}

	if md != nil {
		renderer.mdRenderer = md
		return renderer
	}

	profile := ConfigureCLIColorProfile(os.Stdout)
	renderer.mdRenderer = buildDefaultMarkdownRenderer(profile)
	renderer.maxWidth = detectOutputWidth(os.Stdout)

	return renderer
}

func buildDefaultMarkdownRenderer(profile termenv.Profile) MarkdownRenderer {
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
	displayName := displayToolName(toolName)

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
		return r.constrainWidth(fmt.Sprintf("%s%s %s(%s)\n", indent, spinnerStyle.Render(spinner), toolNameStyle.Render(displayName), preview))
	}

	return r.constrainWidth(fmt.Sprintf("%s%s %s\n", indent, spinnerStyle.Render(spinner), toolNameStyle.Render(displayName)))
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

func displayToolName(toolName string) string {
	normalized := strings.ToLower(strings.TrimSpace(toolName))
	if normalized == "" {
		return toolName
	}
	if display, ok := toolDisplayNames[normalized]; ok {
		return display
	}
	return toolName
}

func appendDurationSuffix(rendered string, duration time.Duration) string {
	if rendered == "" || duration <= 0 {
		return rendered
	}
	formatted := formatDurationShort(duration)
	if formatted == "" {
		return rendered
	}
	suffix := fmt.Sprintf(" (%s)", formatted)
	newline := strings.Index(rendered, "\n")
	if newline == -1 {
		return rendered + suffix
	}
	return rendered[:newline] + suffix + rendered[newline:]
}

func formatDurationShort(duration time.Duration) string {
	if duration <= 0 {
		return ""
	}
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	if duration < time.Minute {
		seconds := duration.Seconds()
		if seconds < 10 {
			return fmt.Sprintf("%.2fs", seconds)
		}
		if seconds < 100 {
			return fmt.Sprintf("%.1fs", seconds)
		}
		return fmt.Sprintf("%.0fs", seconds)
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", hours, minutes)
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
	displayName := displayToolName(toolName)

	// Core agent: always show tool results (concise or verbose format)
	// Determine indentation based on hierarchy
	indent := ""
	if ctx.Level == types.LevelSubagent {
		indent = "  "
	}

	if isConversationalTool(toolName) {
		if err != nil {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
			return r.constrainWidth(fmt.Sprintf("%s\n", errStyle.Render(fmt.Sprintf("✗ %v", err))))
		}
		content := strings.TrimSpace(result)
		if content == "" {
			return ""
		}
		return r.renderMarkdown(content)
	}

	if err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		return r.constrainWidth(fmt.Sprintf("%s  %s\n", indent, errStyle.Render(fmt.Sprintf("✗ %s failed: %v", displayName, err))))
	}

	// Smart display based on tool category and hierarchy
	formatted := r.formatToolOutput(ctx, toolName, result, indent)
	formatted = appendDurationSuffix(formatted, duration)
	return r.constrainWidth(formatted)
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

	return r.constrainWidth(fmt.Sprintf("\n%s%s\n", indent, errStyle.Render(fmt.Sprintf("✗ Error in %s: %v", phase, err))))
}

// RenderSubagentProgress renders subagent progress with proper indentation
func (r *CLIRenderer) RenderSubagentProgress(ctx *types.OutputContext, completed, total int, tokens, toolCalls int) string {
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	progressText := fmt.Sprintf("  ✓ [%d/%d] Task %d | %d tokens | %d tools",
		completed, total, completed, tokens, toolCalls)
	return r.constrainWidth(grayStyle.Render(progressText) + "\n")
}

// RenderSubagentComplete renders subagent completion summary
func (r *CLIRenderer) RenderSubagentComplete(ctx *types.OutputContext, total, success, failed int, tokens, toolCalls int) string {
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	summaryText := fmt.Sprintf("  ━━━ Completed: %d/%d tasks | Total: %d tokens, %d tool calls",
		success, total, tokens, toolCalls)
	return r.constrainWidth(grayStyle.Render(summaryText) + "\n\n")
}

// formatToolOutput formats tool output based on tool category and hierarchy
func (r *CLIRenderer) formatToolOutput(ctx *types.OutputContext, toolName, result string, indent string) string {
	// Use brighter gray (#808080) that works on both light and dark backgrounds
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	normalizedTool := strings.TrimSpace(toolName)
	if strings.HasPrefix(normalizedTool, "sandbox_file_") {
		return r.formatSandboxFileOutput(normalizedTool, result, indent, grayStyle)
	}
	category := CategorizeToolName(normalizedTool)

	switch category {
	case types.CategoryFile:
		return r.formatFileOutput(normalizedTool, result, indent, grayStyle)
	case types.CategorySearch:
		return r.formatSearchOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryShell, types.CategoryExecution:
		return r.formatExecutionOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryWeb:
		return r.formatWebOutput(normalizedTool, result, indent, grayStyle)
	case types.CategoryTask:
		return r.formatTaskOutput(normalizedTool, result, indent, grayStyle)
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
		lines := countLines(cleaned)
		return fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d lines read", lines)))
	case "file_write", "file_edit":
		if summary, ok := summarizeFileOperation(cleaned); ok {
			return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+summary))
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
	summary := parseSearchSummary(cleaned)
	matchCount := summary.Total
	lines := summary.Matches
	if summary.NoMatches {
		matchCount = 0
	}

	var output strings.Builder
	if summary.NoMatches {
		output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ no matches")))
		return output.String()
	}
	summaryLine := fmt.Sprintf("→ %d matches", matchCount)
	if summary.Truncated {
		summaryLine += " (truncated)"
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(summaryLine)))

	// In verbose mode, show first few matches
	if r.verbose && matchCount > 0 {
		preview := lines
		if len(preview) > 5 {
			preview = preview[:5]
		}
		for _, line := range preview {
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
		if len(lines) > len(preview) {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", len(lines)-len(preview)))))
		}
		if summary.Warning != "" {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(summary.Warning)))
		}
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
	cleaned := filterSystemReminders(result)
	switch toolName {
	case "web_search":
		return r.formatWebSearchOutput(cleaned, indent, style)
	case "web_fetch":
		return r.formatWebFetchOutput(cleaned, indent, style)
	default:
		preview := truncateWithEllipsis(cleaned, 100)
		if preview == "" {
			preview = "ok"
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
	}
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

func (r *CLIRenderer) constrainWidth(rendered string) string {
	return ConstrainWidth(rendered, r.maxWidth)
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
	summary := parseListFilesSummary(lines)
	totalCount := summary.Total
	if strings.TrimSpace(content) == "" {
		totalCount = 0
		summary = listFilesSummary{}
	}

	var output strings.Builder

	// Show count summary
	summaryParts := []string{}
	summaryParts = append(summaryParts, fmt.Sprintf("%d entries", totalCount))
	if summary.Dirs > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s", summary.Dirs, pluralize("dir", summary.Dirs)))
	}
	if summary.Files > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d %s", summary.Files, pluralize("file", summary.Files)))
	}
	if summary.TotalBytes > 0 {
		summaryParts = append(summaryParts, formatBytes(summary.TotalBytes))
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+strings.Join(summaryParts, ", "))))

	// In verbose mode, show first few files
	if r.verbose && totalCount > 0 && totalCount <= 10 {
		for _, line := range lines {
			if line != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
			}
		}
	} else if r.verbose && totalCount > 10 {
		for i := 0; i < 5; i++ {
			if lines[i] != "" {
				output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(lines[i])))
			}
		}
		output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", totalCount-5))))
	}

	return output.String()
}

type listFilesSummary struct {
	Total      int
	Files      int
	Dirs       int
	TotalBytes int64
}

func parseListFilesSummary(lines []string) listFilesSummary {
	var summary listFilesSummary
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		summary.Total++
		if strings.HasPrefix(trimmed, "[DIR]") {
			summary.Dirs++
			continue
		}
		if strings.HasPrefix(trimmed, "[FILE]") {
			summary.Files++
			if size, ok := parseFileSize(trimmed); ok {
				summary.TotalBytes += size
			}
		}
	}
	return summary
}

func parseFileSize(line string) (int64, bool) {
	open := strings.LastIndex(line, "(")
	close := strings.LastIndex(line, ")")
	if open == -1 || close == -1 || close <= open {
		return 0, false
	}
	inner := strings.TrimSpace(line[open+1 : close])
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return 0, false
	}
	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return size, true
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	value := float64(bytes)
	for _, unit := range units {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f PB", value)
}

type searchSummary struct {
	Total     int
	Matches   []string
	Truncated bool
	Warning   string
	NoMatches bool
}

func parseSearchSummary(content string) searchSummary {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return searchSummary{NoMatches: true}
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return searchSummary{NoMatches: true}
	}
	first := strings.TrimSpace(lines[0])
	summary := searchSummary{}
	if strings.HasPrefix(first, "No matches found") {
		summary.NoMatches = true
		return summary
	}
	if strings.HasPrefix(first, "Found ") {
		if total, ok := parseFoundMatches(first); ok {
			summary.Total = total
		}
		lines = lines[1:]
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[TRUNCATED]") {
			summary.Truncated = true
			summary.Warning = line
			continue
		}
		summary.Matches = append(summary.Matches, line)
	}
	if summary.Total == 0 {
		summary.Total = len(summary.Matches)
	}
	return summary
}

func parseFoundMatches(line string) (int, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "Found "))
	if rest == "" {
		return 0, false
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return 0, false
	}
	value, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	return value, true
}

func summarizeFileOperation(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "Wrote ") && strings.Contains(trimmed, " bytes to ") {
		parts := strings.SplitN(trimmed, " bytes to ", 2)
		if len(parts) != 2 {
			return "", false
		}
		bytesText := strings.TrimSpace(strings.TrimPrefix(parts[0], "Wrote "))
		path := strings.TrimSpace(parts[1])
		if path == "" {
			return "", false
		}
		if bytesValue, err := strconv.ParseInt(bytesText, 10, 64); err == nil {
			return fmt.Sprintf("wrote %s (%s)", path, formatBytes(bytesValue)), true
		}
		return fmt.Sprintf("wrote %s", path), true
	}
	if strings.HasPrefix(trimmed, "Created ") {
		return parseFileLineSummary("created", strings.TrimPrefix(trimmed, "Created "))
	}
	if strings.HasPrefix(trimmed, "Updated ") {
		return parseFileLineSummary("updated", strings.TrimPrefix(trimmed, "Updated "))
	}
	if strings.HasPrefix(trimmed, "Replaced ") && strings.Contains(trimmed, " in ") {
		parts := strings.SplitN(strings.TrimPrefix(trimmed, "Replaced "), " in ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("replaced %s in %s", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])), true
		}
	}
	return "", false
}

func parseFileLineSummary(action, remainder string) (string, bool) {
	value := strings.TrimSpace(remainder)
	if value == "" {
		return "", false
	}
	idx := strings.LastIndex(value, " (")
	if idx == -1 || !strings.HasSuffix(value, ")") {
		return fmt.Sprintf("%s %s", action, value), true
	}
	path := strings.TrimSpace(value[:idx])
	suffix := strings.TrimSuffix(value[idx+2:], ")")
	fields := strings.Fields(suffix)
	if len(fields) > 0 {
		return fmt.Sprintf("%s %s (%s lines)", action, path, fields[0]), true
	}
	return fmt.Sprintf("%s %s", action, path), true
}

type webSearchItem struct {
	Title string
	URL   string
}

type webSearchSummary struct {
	Query       string
	Summary     string
	ResultCount int
	Results     []webSearchItem
}

func (r *CLIRenderer) formatWebSearchOutput(content, indent string, style lipgloss.Style) string {
	summary := parseWebSearchContent(content)
	var parts []string
	query := strings.TrimSpace(summary.Query)
	if query != "" {
		parts = append(parts, fmt.Sprintf("search %q", truncateInlinePreview(query, 48)))
	} else {
		parts = append(parts, "search")
	}
	if summary.ResultCount > 0 {
		parts = append(parts, fmt.Sprintf("%d results", summary.ResultCount))
	}
	if strings.TrimSpace(summary.Summary) != "" {
		parts = append(parts, "summary available")
	}

	line := strings.Join(parts, ", ")
	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+line)))

	if r.verbose {
		if summaryText := strings.TrimSpace(summary.Summary); summaryText != "" {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render("summary: "+truncateWithEllipsis(summaryText, 200))))
		}
		for i, item := range summary.Results {
			if i >= 3 {
				break
			}
			title := strings.TrimSpace(item.Title)
			if title == "" {
				continue
			}
			host := hostFromURL(item.URL)
			line := title
			if host != "" {
				line = fmt.Sprintf("%s (%s)", title, host)
			}
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
		}
	}

	return output.String()
}

func parseWebSearchContent(content string) webSearchSummary {
	var summary webSearchSummary
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "Search:"):
			summary.Query = strings.TrimSpace(strings.TrimPrefix(trimmed, "Search:"))
		case strings.HasPrefix(trimmed, "Summary:"):
			summary.Summary = strings.TrimSpace(strings.TrimPrefix(trimmed, "Summary:"))
		case strings.HasSuffix(trimmed, "Results:"):
			fields := strings.Fields(trimmed)
			if len(fields) > 0 {
				if count, err := strconv.Atoi(fields[0]); err == nil {
					summary.ResultCount = count
				}
			}
		case strings.HasPrefix(trimmed, "URL:"):
			if len(summary.Results) > 0 {
				summary.Results[len(summary.Results)-1].URL = strings.TrimSpace(strings.TrimPrefix(trimmed, "URL:"))
			}
		default:
			if title, ok := parseNumberedTitle(trimmed); ok {
				summary.Results = append(summary.Results, webSearchItem{Title: title})
			}
		}
	}
	if summary.ResultCount == 0 && len(summary.Results) > 0 {
		summary.ResultCount = len(summary.Results)
	}
	return summary
}

func parseNumberedTitle(line string) (string, bool) {
	idx := strings.Index(line, ".")
	if idx <= 0 {
		return "", false
	}
	if _, err := strconv.Atoi(strings.TrimSpace(line[:idx])); err != nil {
		return "", false
	}
	title := strings.TrimSpace(line[idx+1:])
	if title == "" {
		return "", false
	}
	return title, true
}

type webFetchSummary struct {
	URL      string
	Cached   bool
	Question string
	Analysis string
	Content  string
}

func (r *CLIRenderer) formatWebFetchOutput(content, indent string, style lipgloss.Style) string {
	summary := parseWebFetchContent(content)
	host := hostFromURL(summary.URL)
	if host == "" {
		host = strings.TrimSpace(summary.URL)
	}
	body := summary.Content
	action := "fetched"
	if strings.TrimSpace(summary.Analysis) != "" || strings.TrimSpace(summary.Question) != "" {
		action = "analyzed"
		if summary.Analysis != "" {
			body = summary.Analysis
		}
	}
	lineCount := countLines(strings.TrimSpace(body))
	var parts []string
	if host != "" {
		parts = append(parts, fmt.Sprintf("%s %s", action, host))
	} else {
		parts = append(parts, action)
	}
	if summary.Cached {
		parts = append(parts, "cached")
	}
	if lineCount > 0 {
		parts = append(parts, fmt.Sprintf("%d lines", lineCount))
	}
	line := strings.Join(parts, ", ")

	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+line)))
	if r.verbose {
		if question := strings.TrimSpace(summary.Question); question != "" {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render("question: "+truncateWithEllipsis(question, 160))))
		}
		label := "content"
		if strings.TrimSpace(summary.Analysis) != "" {
			label = "analysis"
			body = summary.Analysis
		}
		preview := takePreviewLines(body, 3)
		if len(preview) > 0 {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(label+":")))
			for _, line := range preview {
				output.WriteString(fmt.Sprintf("%s      %s\n", indent, style.Render(truncateWithEllipsis(line, 200))))
			}
		}
	}
	return output.String()
}

func parseWebFetchContent(content string) webFetchSummary {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return webFetchSummary{}
	}
	lines := strings.Split(trimmed, "\n")
	summary := webFetchSummary{}
	index := 0
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "Source:") {
		source := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "Source:"))
		if strings.HasSuffix(source, "(cached)") {
			summary.Cached = true
			source = strings.TrimSpace(strings.TrimSuffix(source, "(cached)"))
		}
		summary.URL = source
		index = 1
	}
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	if index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), "Question:") {
		summary.Question = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[index]), "Question:"))
		index++
		for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
			index++
		}
		if index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), "Analysis:") {
			index++
			summary.Analysis = strings.TrimSpace(strings.Join(lines[index:], "\n"))
			return summary
		}
		summary.Content = strings.TrimSpace(strings.Join(lines[index:], "\n"))
		return summary
	}
	summary.Content = strings.TrimSpace(strings.Join(lines[index:], "\n"))
	return summary
}

func hostFromURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return trimmed
}

func takePreviewLines(content string, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

func (r *CLIRenderer) formatSandboxFileOutput(toolName, result, indent string, style lipgloss.Style) string {
	cleaned := filterSystemReminders(result)
	switch toolName {
	case "sandbox_file_read":
		lines := countLines(cleaned)
		return fmt.Sprintf("%s  %s\n", indent, style.Render(fmt.Sprintf("→ %d lines read", lines)))
	case "sandbox_file_write", "sandbox_file_replace":
		if summary, ok := summarizeFileOperation(cleaned); ok {
			return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+summary))
		}
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	case "sandbox_file_list":
		if summary, ok := parseSandboxFileListSummary(cleaned); ok {
			return r.renderSandboxFileList(summary, indent, style)
		}
		preview := truncateWithEllipsis(cleaned, 100)
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
	case "sandbox_file_search":
		if summary, ok := parseSandboxFileSearchSummary(cleaned); ok {
			return r.renderSandboxFileSearch(summary, indent, style)
		}
		preview := truncateWithEllipsis(cleaned, 100)
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
	default:
		return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+cleaned))
	}
}

type sandboxFileListSummary struct {
	Path       string
	Total      int
	Files      int
	Dirs       int
	TotalBytes int64
	Entries    []sandboxFileEntry
}

type sandboxFileEntry struct {
	Path  string
	IsDir bool
	Size  *int64
}

func parseSandboxFileListSummary(content string) (sandboxFileListSummary, bool) {
	var payload struct {
		Path           string `json:"path"`
		Files          []struct {
			Path        string  `json:"path"`
			Name        string  `json:"name"`
			IsDirectory bool    `json:"is_directory"`
			Size        *int64  `json:"size"`
			Permissions *string `json:"permissions"`
		} `json:"files"`
		TotalCount     int `json:"total_count"`
		DirectoryCount int `json:"directory_count"`
		FileCount      int `json:"file_count"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return sandboxFileListSummary{}, false
	}
	summary := sandboxFileListSummary{
		Path:  payload.Path,
		Total: payload.TotalCount,
		Files: payload.FileCount,
		Dirs:  payload.DirectoryCount,
	}
	derivedFiles := 0
	derivedDirs := 0
	for _, entry := range payload.Files {
		if entry.IsDirectory {
			derivedDirs++
		} else {
			derivedFiles++
		}
	}
	if summary.Files == 0 && summary.Dirs == 0 && (derivedFiles > 0 || derivedDirs > 0) {
		summary.Files = derivedFiles
		summary.Dirs = derivedDirs
	}
	if summary.Total == 0 && len(payload.Files) > 0 {
		summary.Total = len(payload.Files)
	}
	for _, entry := range payload.Files {
		summary.Entries = append(summary.Entries, sandboxFileEntry{
			Path:  entry.Path,
			IsDir: entry.IsDirectory,
			Size:  entry.Size,
		})
		if entry.Size != nil {
			summary.TotalBytes += *entry.Size
		}
	}
	return summary, true
}

func (r *CLIRenderer) renderSandboxFileList(summary sandboxFileListSummary, indent string, style lipgloss.Style) string {
	var output strings.Builder
	parts := []string{fmt.Sprintf("%d entries", summary.Total)}
	if summary.Dirs > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", summary.Dirs, pluralize("dir", summary.Dirs)))
	}
	if summary.Files > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", summary.Files, pluralize("file", summary.Files)))
	}
	if summary.TotalBytes > 0 {
		parts = append(parts, formatBytes(summary.TotalBytes))
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+strings.Join(parts, ", "))))
	if r.verbose && len(summary.Entries) > 0 {
		preview := summary.Entries
		if len(preview) > 5 {
			preview = preview[:5]
		}
		for _, entry := range preview {
			line := entry.Path
			if entry.IsDir {
				line = "[DIR] " + line
			} else {
				line = "[FILE] " + line
				if entry.Size != nil {
					line = fmt.Sprintf("%s (%s)", line, formatBytes(*entry.Size))
				}
			}
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(line)))
		}
		if len(summary.Entries) > len(preview) {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", len(summary.Entries)-len(preview)))))
		}
	}
	return output.String()
}

type sandboxFileSearchSummary struct {
	File    string
	Matches []string
	Lines   []int
}

func parseSandboxFileSearchSummary(content string) (sandboxFileSearchSummary, bool) {
	var payload struct {
		File        string   `json:"file"`
		Matches     []string `json:"matches"`
		LineNumbers []int    `json:"line_numbers"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return sandboxFileSearchSummary{}, false
	}
	return sandboxFileSearchSummary{
		File:    payload.File,
		Matches: payload.Matches,
		Lines:   payload.LineNumbers,
	}, true
}

func (r *CLIRenderer) renderSandboxFileSearch(summary sandboxFileSearchSummary, indent string, style lipgloss.Style) string {
	var output strings.Builder
	matchCount := len(summary.Matches)
	header := fmt.Sprintf("→ %d matches", matchCount)
	if summary.File != "" {
		header += fmt.Sprintf(" in %s", summary.File)
	}
	output.WriteString(fmt.Sprintf("%s  %s\n", indent, style.Render(header)))
	if r.verbose && matchCount > 0 {
		preview := summary.Matches
		if len(preview) > 5 {
			preview = preview[:5]
		}
		for i, match := range preview {
			line := match
			if i < len(summary.Lines) && summary.Lines[i] > 0 {
				line = fmt.Sprintf("%d: %s", summary.Lines[i], match)
			}
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(truncateWithEllipsis(line, 200))))
		}
		if len(summary.Matches) > len(preview) {
			output.WriteString(fmt.Sprintf("%s    %s\n", indent, style.Render(fmt.Sprintf("... and %d more", len(summary.Matches)-len(preview)))))
		}
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
