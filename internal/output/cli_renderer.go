package output

import (
	"alex/internal/agent/domain"
	"alex/internal/agent/types"
	"alex/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// CLIRenderer renders output for CLI display with hierarchical context awareness
type MarkdownRenderer interface {
	Render(string) (string, error)
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
	formatter  *domain.ToolFormatter
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
		formatter: domain.NewToolFormatter(),
	}

	if md != nil {
		renderer.mdRenderer = md
		return renderer
	}

	// Set lipgloss to use stdout for color detection only when using the default renderer.
	lipgloss.SetColorProfile(lipgloss.NewRenderer(os.Stdout).ColorProfile())

	if defaultRenderer := buildDefaultMarkdownRenderer(); defaultRenderer != nil {
		renderer.mdRenderer = defaultRenderer
	}

	return renderer
}

func buildDefaultMarkdownRenderer() MarkdownRenderer {
	options := []glamour.TermRendererOption{
		glamour.WithWordWrap(100),
		glamour.WithPreservedNewLines(),
	}

	if value, ok := config.DefaultEnvLookup("GLAMOUR_STYLE"); ok && value != "" {
		options = append(options, glamour.WithEnvironmentConfig())
	} else if term.IsTerminal(int(os.Stdout.Fd())) {
		options = append(options, glamour.WithAutoStyle())
	} else {
		options = append(options, glamour.WithStandardStyle("dark"))
	}

	mdRenderer, err := glamour.NewTermRenderer(options...)
	if err != nil {
		return nil
	}
	return mdRenderer
}

// Target returns the output target
func (r *CLIRenderer) Target() OutputTarget {
	return TargetCLI
}

// RenderTaskAnalysis renders task analysis details with hierarchy-aware styling.
func (r *CLIRenderer) RenderTaskAnalysis(ctx *types.OutputContext, event *domain.TaskAnalysisEvent) string {
	if event == nil {
		return ""
	}

	action := strings.TrimSpace(event.ActionName)
	if action == "" {
		action = "Task analysis"
	}

	var prefix string
	var detailIndent string
	switch ctx.Level {
	case types.LevelCore:
		prefix = "👾"
	case types.LevelSubagent:
		prefix = "  ↳"
		detailIndent = "  "
	case types.LevelParallel:
		prefix = "  ⇉"
		detailIndent = "  "
	default:
		prefix = "👾"
	}

	header := fmt.Sprintf("%s %s", prefix, action)
	var output strings.Builder

	switch ctx.Level {
	case types.LevelCore:
		output.WriteString("\n")
		output.WriteString(renderPurpleGradient(header))
		output.WriteString("\n")
	case types.LevelSubagent:
		output.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).Render(header))
		output.WriteString("\n")
	case types.LevelParallel:
		output.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#909090")).Render(header))
		output.WriteString("\n")
	default:
		output.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")).Render(header))
		output.WriteString("\n")
	}

	type section []string
	var sections []section

	goal := strings.TrimSpace(event.Goal)
	approach := strings.TrimSpace(event.Approach)
	if goal != "" || approach != "" {
		var lines section
		if goal != "" {
			lines = append(lines, fmt.Sprintf("%sGoal: %s", detailIndent, goal))
		}
		if approach != "" {
			lines = append(lines, fmt.Sprintf("%sApproach: %s", detailIndent, approach))
		}
		if len(lines) > 0 {
			sections = append(sections, lines)
		}
	}

	if len(event.SuccessCriteria) > 0 {
		criteriaLines := section{fmt.Sprintf("%sSuccess criteria:", detailIndent)}
		for _, c := range event.SuccessCriteria {
			trimmed := strings.TrimSpace(c)
			if trimmed == "" {
				continue
			}
			criteriaLines = append(criteriaLines, fmt.Sprintf("%s  • %s", detailIndent, trimmed))
		}
		if len(criteriaLines) > 1 {
			sections = append(sections, criteriaLines)
		}
	}

	if len(event.Steps) > 0 {
		planLines := section{fmt.Sprintf("%sPlan:", detailIndent)}
		for idx, step := range event.Steps {
			desc := strings.TrimSpace(step.Description)
			if desc == "" {
				continue
			}
			line := fmt.Sprintf("%s  %d. %s", detailIndent, idx+1, desc)
			if step.NeedsExternalContext {
				line += " [needs external context]"
			}
			planLines = append(planLines, line)

			rationale := strings.TrimSpace(step.Rationale)
			if rationale != "" {
				planLines = append(planLines, fmt.Sprintf("%s     ↳ %s", detailIndent, rationale))
			}
		}
		if len(planLines) > 1 {
			sections = append(sections, planLines)
		}
	}

	retrieval := event.Retrieval
	hasRetrievalData := retrieval.ShouldRetrieve || len(retrieval.LocalQueries) > 0 || len(retrieval.SearchQueries) > 0 ||
		len(retrieval.CrawlURLs) > 0 || len(retrieval.KnowledgeGaps) > 0 || strings.TrimSpace(retrieval.Notes) != ""
	if hasRetrievalData {
		retrievalLines := section{fmt.Sprintf("%sRetrieval plan:", detailIndent)}
		if retrieval.ShouldRetrieve {
			retrievalLines = append(retrievalLines, fmt.Sprintf("%s  Status: Gather additional context", detailIndent))
		} else {
			retrievalLines = append(retrievalLines, fmt.Sprintf("%s  Status: Skip retrieval", detailIndent))
		}

		appendList := func(label string, values []string) {
			var cleaned []string
			for _, value := range values {
				trimmed := strings.TrimSpace(value)
				if trimmed != "" {
					cleaned = append(cleaned, trimmed)
				}
			}
			if len(cleaned) == 0 {
				return
			}
			retrievalLines = append(retrievalLines, fmt.Sprintf("%s  %s:", detailIndent, label))
			for _, item := range cleaned {
				retrievalLines = append(retrievalLines, fmt.Sprintf("%s    • %s", detailIndent, item))
			}
		}

		appendList("Local queries", retrieval.LocalQueries)
		appendList("Search queries", retrieval.SearchQueries)
		appendList("Crawl targets", retrieval.CrawlURLs)
		appendList("Knowledge gaps / TODOs", retrieval.KnowledgeGaps)

		if notes := strings.TrimSpace(retrieval.Notes); notes != "" {
			retrievalLines = append(retrievalLines, fmt.Sprintf("%s  Notes: %s", detailIndent, notes))
		}

		if len(retrievalLines) > 1 {
			sections = append(sections, retrievalLines)
		}
	}

	if len(sections) == 0 {
		if ctx.Level == types.LevelCore {
			output.WriteString("\n")
		}
		return output.String()
	}

	output.WriteString("\n")
	for idx, block := range sections {
		for _, line := range block {
			output.WriteString(line)
			output.WriteString("\n")
		}
		if idx < len(sections)-1 {
			output.WriteString("\n")
		}
	}
	output.WriteString("\n")

	return output.String()
}

// RenderToolCallStart renders tool call start with hierarchy awareness
func (r *CLIRenderer) RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string {
	// Hide tool calls for subagents (they show progress summary instead)
	if ctx.Level == types.LevelSubagent || ctx.Level == types.LevelParallel {
		return ""
	}

	// Core agent: always show tool calls (concise or verbose format)
	// Determine indentation based on hierarchy
	indent := ""
	if ctx.Level == types.LevelSubagent {
		indent = "  "
	}

	dotStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)
	toolNameStyle := lipgloss.NewStyle().Bold(true)

	presentation := r.formatter.PrepareArgs(toolName, args)

	if presentation.ShouldDisplay && presentation.InlinePreview != "" {
		preview := presentation.InlinePreview
		if !r.verbose {
			preview = truncateInlinePreview(preview, nonVerbosePreviewLimit)
		}
		return fmt.Sprintf("%s%s %s(%s)\n", indent, dotStyle.Render("●"), toolNameStyle.Render(toolName), preview)
	}

	return fmt.Sprintf("%s%s %s\n", indent, dotStyle.Render("●"), toolNameStyle.Render(toolName))
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

	if err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		return fmt.Sprintf("%s  %s\n", indent, errStyle.Render(fmt.Sprintf("✗ %s failed: %v", toolName, err)))
	}

	// Smart display based on tool category and hierarchy
	return r.formatToolOutput(ctx, toolName, result, indent)
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
		preview := cleaned
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
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
	preview := cleaned
	if len(preview) > 100 {
		preview = preview[:97] + "..."
	}
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
	preview := cleaned
	if len(preview) > 100 {
		preview = preview[:97] + "..."
	}
	return fmt.Sprintf("%s  %s\n", indent, style.Render("→ "+preview))
}

// Helper functions

func renderPurpleGradient(text string) string {
	colors := []string{
		"#E0B0FF", "#D8A7F5", "#C78EEB",
		"#B678E0", "#9F5FD6", "#8B47CC",
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}

	var result strings.Builder
	colorsLen := len(colors)
	runesLen := len(runes)

	for i, r := range runes {
		colorIdx := (i * (colorsLen - 1)) / max(runesLen-1, 1)
		if colorIdx >= colorsLen {
			colorIdx = colorsLen - 1
		}

		color := colors[colorIdx]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		result.WriteString(style.Render(string(r)))
	}

	return result.String()
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

	return strings.TrimRight(rendered, "\n") + "\n"
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

	if r.mdRenderer == nil {
		if ensureTrailingNewline && !strings.HasSuffix(content, "\n") {
			return content + "\n"
		}
		return content
	}

	rendered, err := r.mdRenderer.Render(content)
	if err != nil {
		if ensureTrailingNewline && !strings.HasSuffix(content, "\n") {
			return content + "\n"
		}
		return content
	}

	rendered = strings.TrimRight(rendered, "\n")
	if ensureTrailingNewline {
		rendered += "\n"
	}
	return rendered
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
