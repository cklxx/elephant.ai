package output

import (
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/types"
	"alex/internal/presentation/formatter"

	"github.com/charmbracelet/lipgloss"
)

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

// NewCLIRenderer creates a new CLI renderer
// verbose=true enables detailed output (full args, more content preview)
// verbose=false shows compact output (tool name + brief summary)
func NewCLIRenderer(verbose bool) *CLIRenderer {
	return NewCLIRendererWithMarkdown(verbose, nil)
}

// NewCLIRendererWithMarkdown allows tests to supply a lightweight markdown renderer.
func NewCLIRendererWithMarkdown(verbose bool, md MarkdownRenderer) *CLIRenderer {
	renderer := &CLIRenderer{
		verbose:    verbose,
		formatter:  formatter.NewToolFormatter(),
		maxWidth:   0,
		mdRenderer: md,
	}

	if md != nil {
		return renderer
	}

	profile := ConfigureCLIColorProfile(os.Stdout)
	renderer.mdRenderer = buildDefaultMarkdownRenderer(profile)
	renderer.maxWidth = detectOutputWidth(os.Stdout)

	return renderer
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

func (r *CLIRenderer) constrainWidth(rendered string) string {
	return ConstrainWidth(rendered, r.maxWidth)
}
