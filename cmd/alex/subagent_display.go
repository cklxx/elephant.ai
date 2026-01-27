package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"alex/internal/agent/domain"
	"alex/internal/output"
	"alex/internal/tools/builtin/orchestration"

	"github.com/charmbracelet/lipgloss"
)

// SubagentDisplay coordinates consistent CLI output for subagent progress across
// both streaming and interactive CLI modes.
type SubagentDisplay struct {
	mu sync.Mutex

	headerPrinted     bool
	totalTasks        int
	maxParallel       int
	summaryForTotal   int
	headerTotals      int
	headerMaxParallel int

	// Aggregate counters
	completed int
	failed    int
	tokens    int
	toolCalls int

	tasks map[int]*subagentTaskState

	useUnicode bool
}

type subagentTaskState struct {
	preview   string
	toolCalls int
	tokens    int

	started bool
	done    bool
	failed  bool
	err     error
}

func NewSubagentDisplay() *SubagentDisplay {
	return &SubagentDisplay{
		tasks:      make(map[int]*subagentTaskState),
		useUnicode: supportsUnicodeSymbols(),
	}
}

// Handle consumes a SubtaskEvent and returns the lines that should be printed
// to the CLI. The caller is responsible for writing the returned strings to
// stdout in order.
func (d *SubagentDisplay) Handle(event *orchestration.SubtaskEvent) []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var lines []string

	d.updateTotals(event)

	shouldPrintHeader := false
	updatedHeader := false
	if !d.headerPrinted {
		d.headerPrinted = true
		shouldPrintHeader = true
	} else if d.totalTasks != d.headerTotals || d.maxParallel != d.headerMaxParallel {
		shouldPrintHeader = true
		updatedHeader = true
	}

	if shouldPrintHeader {
		lines = append(lines, d.renderHeader(updatedHeader))
		d.headerTotals = d.totalTasks
		d.headerMaxParallel = d.maxParallel
	}

	state := d.ensureState(event.SubtaskIndex, event.SubtaskPreview)

	// Track if we need to show start line for fast-completing tasks
	needsStartLine := !state.started

	if !state.started {
		state.started = true
		// For regular events, show start line immediately
		switch event.OriginalEvent.(type) {
		case *domain.WorkflowResultFinalEvent, *domain.WorkflowNodeFailedEvent:
			// Don't show start line yet; will show before completion/failure
		default:
			lines = append(lines, d.renderStartLine(event.SubtaskIndex, state))
			needsStartLine = false
		}
	}

	switch e := event.OriginalEvent.(type) {
	case *domain.WorkflowToolCompletedEvent:
		if !state.done {
			state.toolCalls++
		}
	case *domain.WorkflowResultFinalEvent:
		if state.done {
			break
		}
		// Show start line first if this is a fast-completing task
		if needsStartLine {
			lines = append(lines, d.renderStartLine(event.SubtaskIndex, state))
		}
		state.done = true
		state.tokens = e.TotalTokens
		d.completed++
		d.tokens += e.TotalTokens
		d.toolCalls += state.toolCalls
		lines = append(lines, d.renderCompletionLine(event.SubtaskIndex, state))
	case *domain.WorkflowNodeFailedEvent:
		if state.done {
			break
		}
		// Show start line first if this is a fast-completing task
		if needsStartLine {
			lines = append(lines, d.renderStartLine(event.SubtaskIndex, state))
		}
		state.done = true
		state.failed = true
		state.err = e.Error
		d.failed++
		lines = append(lines, d.renderFailureLine(event.SubtaskIndex, state))
	}

	concluded := d.completed + d.failed
	if concluded >= d.totalTasks && d.totalTasks > 0 && d.totalTasks != d.summaryForTotal {
		if summary := d.renderSummary(); summary != "" {
			lines = append(lines, summary)
			d.summaryForTotal = d.totalTasks
		}
	}

	return lines
}

func (d *SubagentDisplay) ensureState(index int, preview string) *subagentTaskState {
	if d.tasks == nil {
		d.tasks = make(map[int]*subagentTaskState)
	}
	state, ok := d.tasks[index]
	if !ok {
		state = &subagentTaskState{}
		d.tasks[index] = state
	}
	if state.preview == "" {
		state.preview = sanitizePreview(preview)
	}
	if d.totalTasks < index+1 {
		d.totalTasks = index + 1
	}
	return state
}

func (d *SubagentDisplay) updateTotals(event *orchestration.SubtaskEvent) {
	if d.totalTasks == 0 {
		d.totalTasks = event.TotalSubtasks
	}
	if event.TotalSubtasks > d.totalTasks {
		d.totalTasks = event.TotalSubtasks
	}
	if d.totalTasks <= 0 {
		d.totalTasks = 1
	}

	if d.maxParallel == 0 {
		d.maxParallel = event.MaxParallel
	}
	if event.MaxParallel > d.maxParallel {
		d.maxParallel = event.MaxParallel
	}
	if d.maxParallel <= 0 {
		d.maxParallel = 1
	}
}

func (d *SubagentDisplay) renderHeader(updated bool) string {
	taskLabel := "tasks"
	if d.totalTasks == 1 {
		taskLabel = "task"
	}

	parallel := ""
	if d.maxParallel > 1 {
		parallel = fmt.Sprintf(" (max %d parallel)", d.maxParallel)
	}

	icon := "🤖"
	if updated {
		icon = "↻"
	}

	if !d.useUnicode {
		icon = "AGENT"
		if updated {
			icon = "UPDATE"
		}
	}

	line := fmt.Sprintf("%s Subagent: Running %d %s%s", icon, d.totalTasks, taskLabel, parallel)
	return output.ConstrainOutputWidth("\n"+subagentGrayStyle.Render(line)+"\n", os.Stdout)
}

func (d *SubagentDisplay) renderCompletionLine(index int, state *subagentTaskState) string {
	concluded := d.completed + d.failed
	preview := state.preview
	if preview != "" {
		preview = " – " + truncatePreview(preview)
	}

	tokenLabel := "tokens"
	if state.tokens == 1 {
		tokenLabel = "token"
	}

	toolLabel := "tools"
	if state.toolCalls == 1 {
		toolLabel = "tool"
	}

	check := "✓"
	if !d.useUnicode {
		check = "OK"
	}
	line := fmt.Sprintf("   %s [%d/%d] Task %d%s | %d %s | %d %s", check, concluded, d.totalTasks, index+1, preview, state.tokens, tokenLabel, state.toolCalls, toolLabel)
	return output.ConstrainOutputWidth(subagentGrayStyle.Render(line)+"\n", os.Stdout)
}

func (d *SubagentDisplay) renderFailureLine(index int, state *subagentTaskState) string {
	concluded := d.completed + d.failed
	preview := state.preview
	if preview != "" {
		preview = " – " + truncatePreview(preview)
	}

	errText := ""
	if state.err != nil {
		errText = strings.TrimSpace(state.err.Error())
	}
	if errText == "" {
		errText = "failed"
	}

	cross := "✗"
	if !d.useUnicode {
		cross = "X"
	}
	line := fmt.Sprintf("   %s [%d/%d] Task %d%s: %s", cross, concluded, d.totalTasks, index+1, preview, errText)
	return output.ConstrainOutputWidth(subagentRedStyle.Render(line)+"\n", os.Stdout)
}

func (d *SubagentDisplay) renderSummary() string {
	taskLabel := pluralize(d.totalTasks, "task", "tasks")
	tokenLabel := pluralize(d.tokens, "token", "tokens")
	toolLabel := pluralize(d.toolCalls, "tool call", "tool calls")

	if d.failed == 0 {
		arrow := "→"
		if !d.useUnicode {
			arrow = "->"
		}
		line := fmt.Sprintf("   %s All %d %s completed successfully | %d %s | %d %s", arrow, d.totalTasks, taskLabel, d.tokens, tokenLabel, d.toolCalls, toolLabel)
		return output.ConstrainOutputWidth(subagentGrayStyle.Render(line)+"\n", os.Stdout)
	}

	failureLabel := pluralize(d.failed, "failure", "failures")
	arrow := "→"
	if !d.useUnicode {
		arrow = "->"
	}
	line := fmt.Sprintf("   %s %d of %d %s completed, %d %s | %d %s | %d %s", arrow, d.completed, d.totalTasks, taskLabel, d.failed, failureLabel, d.tokens, tokenLabel, d.toolCalls, toolLabel)
	return output.ConstrainOutputWidth(subagentRedStyle.Render(line)+"\n", os.Stdout)
}

func (d *SubagentDisplay) renderStartLine(index int, state *subagentTaskState) string {
	preview := state.preview
	if preview != "" {
		preview = " – " + truncatePreview(preview)
	}

	arrow := "→"
	if !d.useUnicode {
		arrow = "->"
	}
	line := fmt.Sprintf("   %s Task %d%s", arrow, index+1, preview)
	return output.ConstrainOutputWidth(subagentGrayStyle.Render(line)+"\n", os.Stdout)
}

func sanitizePreview(preview string) string {
	preview = strings.TrimSpace(preview)
	if preview == "" {
		return ""
	}
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.Join(strings.Fields(preview), " ")
	return preview
}

func truncatePreview(preview string) string {
	const maxRunes = 60
	runes := []rune(preview)
	if len(runes) <= maxRunes {
		return preview
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func supportsUnicodeSymbols() bool {
	if term, ok := os.LookupEnv("TERM"); ok && strings.EqualFold(strings.TrimSpace(term), "dumb") {
		return false
	}
	return true
}

var (
	subagentGrayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	subagentRedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
)
