package main

import (
	"alex/internal/agent/types"
	"context"
	"fmt"
	"os"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/output"
	"alex/internal/tools/builtin"
)

// ToolInfo stores information about an active tool call
type ToolInfo struct {
	Name      string
	StartTime time.Time
}

// SubtaskProgress tracks real-time progress for a single subtask
type SubtaskProgress struct {
	Index          int
	Preview        string
	CurrentTool    string
	ToolsCompleted int
	Status         string // "running", "completed", "failed"
}

// StreamingOutputHandler handles streaming output to terminal using unified renderers
type StreamingOutputHandler struct {
	container *Container
	renderer  *output.CLIRenderer
	ctx       context.Context // Context from coordinator for OutputContext

	// State
	activeTools     map[string]ToolInfo
	subtaskProgress map[int]*SubtaskProgress // Track each subtask's progress
	totalSubtasks   int                      // Total number of subtasks (for display layout)
	headerPrinted   bool                     // Whether we've printed the header
	verbose         bool
}

func NewStreamingOutputHandler(container *Container, verbose bool) *StreamingOutputHandler {
	return &StreamingOutputHandler{
		container:       container,
		renderer:        output.NewCLIRenderer(verbose),
		activeTools:     make(map[string]ToolInfo),
		subtaskProgress: make(map[int]*SubtaskProgress),
		verbose:         verbose,
	}
}

// RunTaskWithStreamOutput executes a task with inline streaming output
func RunTaskWithStreamOutput(container *Container, task string, sessionID string) error {
	// Start execution with stream handler
	ctx := context.Background()

	// Set core agent output context
	coreOutCtx := &types.OutputContext{
		Level:   types.LevelCore,
		AgentID: "core",
		Verbose: isVerbose(),
	}
	ctx = types.WithOutputContext(ctx, coreOutCtx)

	handler := NewStreamingOutputHandler(container, isVerbose())
	handler.ctx = ctx // Store context for OutputContext lookup

	// Create event bridge
	bridge := NewStreamEventBridge(handler)

	// Add listener to context so subagent can forward events
	ctx = builtin.WithParentListener(ctx, bridge)

	// Execute task with streaming via listener
	domainResult, err := container.Coordinator.ExecuteTask(ctx, task, sessionID, bridge)
	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Convert to domain.TaskResult for completion rendering
	result := &domain.TaskResult{
		Answer:     domainResult.Answer,
		Iterations: domainResult.Iterations,
		TokensUsed: domainResult.TokensUsed,
		StopReason: domainResult.StopReason,
	}

	// Print completion summary
	handler.printCompletion(result)

	return nil
}

// StreamEventBridge converts domain events to stream output
type StreamEventBridge struct {
	handler *StreamingOutputHandler
}

func NewStreamEventBridge(handler *StreamingOutputHandler) *StreamEventBridge {
	return &StreamEventBridge{handler: handler}
}

// OnEvent implements domain.EventListener
func (b *StreamEventBridge) OnEvent(event domain.AgentEvent) {
	// Check if this is a wrapped subtask event
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		// Handle subtask-specific tracking
		b.handler.handleSubtaskEvent(subtaskEvent)
		return
	}

	// Handle regular events
	switch e := event.(type) {
	case *domain.TaskAnalysisEvent:
		b.handler.onTaskAnalysis(e)
	case *domain.IterationStartEvent:
		b.handler.onIterationStart(e)
	case *domain.ThinkingEvent:
		b.handler.onThinking(e)
	case *domain.ThinkCompleteEvent:
		b.handler.onThinkComplete(e)
	case *domain.ToolCallStartEvent:
		b.handler.onToolCallStart(e)
	case *domain.ToolCallCompleteEvent:
		b.handler.onToolCallComplete(e)
	case *domain.ErrorEvent:
		b.handler.onError(e)
	}
}

// Event handlers

func (h *StreamingOutputHandler) onTaskAnalysis(event *domain.TaskAnalysisEvent) {
	// Use agent level from event (events now carry their source info)
	outCtx := &types.OutputContext{
		Level:   event.GetAgentLevel(),
		AgentID: string(event.GetAgentLevel()),
		Verbose: h.verbose,
	}
	rendered := h.renderer.RenderTaskAnalysis(outCtx, event)
	if rendered != "" {
		fmt.Print(rendered)
	}
}

func (h *StreamingOutputHandler) onIterationStart(event *domain.IterationStartEvent) {
	// Silent - don't print iteration headers in simple mode
}

func (h *StreamingOutputHandler) onThinking(event *domain.ThinkingEvent) {
	// Silent - analysis is shown in think complete
}

func (h *StreamingOutputHandler) onThinkComplete(event *domain.ThinkCompleteEvent) {
	// Silent - don't print analysis output
	// Analysis is internal reasoning, not user-facing output
}

func (h *StreamingOutputHandler) onToolCallStart(event *domain.ToolCallStartEvent) {
	h.activeTools[event.CallID] = ToolInfo{
		Name:      event.ToolName,
		StartTime: event.Timestamp(),
	}

	// Use agent level from event
	outCtx := &types.OutputContext{
		Level:    event.GetAgentLevel(),
		Category: output.CategorizeToolName(event.ToolName),
		AgentID:  string(event.GetAgentLevel()),
		Verbose:  h.verbose,
	}

	rendered := h.renderer.RenderToolCallStart(outCtx, event.ToolName, event.Arguments)
	if rendered != "" {
		fmt.Print(rendered)
	}
}

func (h *StreamingOutputHandler) onToolCallComplete(event *domain.ToolCallCompleteEvent) {
	info, exists := h.activeTools[event.CallID]
	if !exists {
		return
	}

	// Use agent level from event
	outCtx := &types.OutputContext{
		Level:    event.GetAgentLevel(),
		Category: output.CategorizeToolName(info.Name),
		AgentID:  string(event.GetAgentLevel()),
		Verbose:  h.verbose,
	}

	duration := time.Since(info.StartTime)
	rendered := h.renderer.RenderToolCallComplete(outCtx, info.Name, event.Result, event.Error, duration)
	if rendered != "" {
		fmt.Print(rendered)
	}

	delete(h.activeTools, event.CallID)
}

func (h *StreamingOutputHandler) onError(event *domain.ErrorEvent) {
	outCtx := h.getOutputContext()
	rendered := h.renderer.RenderError(outCtx, event.Phase, event.Error)
	if rendered != "" {
		fmt.Print(rendered)
	}
}

func (h *StreamingOutputHandler) printCompletion(result *domain.TaskResult) {
	outCtx := h.getOutputContext()
	rendered := h.renderer.RenderTaskComplete(outCtx, result)
	if rendered != "" {
		fmt.Print(rendered)
	}
}

// handleSubtaskEvent handles events from subtasks with simple line-by-line output
func (h *StreamingOutputHandler) handleSubtaskEvent(subtaskEvent *builtin.SubtaskEvent) {
	idx := subtaskEvent.SubtaskIndex

	// Gray color for all subagent output
	grayStyle := "\033[90m"
	resetStyle := "\033[0m"

	// Initialize progress tracking for this subtask if needed
	if _, exists := h.subtaskProgress[idx]; !exists {
		// First event - print header once
		if !h.headerPrinted {
			h.totalSubtasks = subtaskEvent.TotalSubtasks
			h.headerPrinted = true
			fmt.Printf("\n%s🤖 Subagent: Running %d tasks%s\n", grayStyle, h.totalSubtasks, resetStyle)
		}

		h.subtaskProgress[idx] = &SubtaskProgress{
			Index:   idx,
			Preview: subtaskEvent.SubtaskPreview,
			Status:  "running",
		}

		// Print task start
		preview := subtaskEvent.SubtaskPreview
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		fmt.Printf("%s  ⇉ Task %d/%d: %s%s\n", grayStyle, idx+1, h.totalSubtasks, preview, resetStyle)
	}

	progress := h.subtaskProgress[idx]

	// Handle different event types from the subtask
	switch e := subtaskEvent.OriginalEvent.(type) {
	case *domain.ToolCallStartEvent:
		// Tool started - show on same line
		progress.CurrentTool = e.ToolName
		progress.Status = "running"
		// Use \r to overwrite current line
		fmt.Printf("\r%s    ● %s%s", grayStyle, e.ToolName, resetStyle)

	case *domain.ToolCallCompleteEvent:
		// Tool completed - clear line and update count
		progress.ToolsCompleted++
		progress.CurrentTool = ""
		fmt.Printf("\r\033[K") // Clear line

	case *domain.TaskCompleteEvent:
		// Task completed - print final status on new line
		progress.Status = "completed"
		fmt.Printf("\r\033[K%s    ✓ Completed | %d tokens%s\n", grayStyle, e.TotalTokens, resetStyle)

	case *domain.ErrorEvent:
		// Subtask failed - print error in red
		progress.Status = "failed"
		redStyle := "\033[91m"
		fmt.Printf("\r\033[K%s    ✗ Error: %v%s\n", redStyle, e.Error, resetStyle)
	}
}

// getOutputContext retrieves OutputContext from coordinator context
func (h *StreamingOutputHandler) getOutputContext() *types.OutputContext {
	// Try to get from context first (will be set by subagent)
	if h.ctx != nil {
		if outCtx := types.GetOutputContext(h.ctx); outCtx != nil {
			// Update verbose flag from handler
			outCtx.Verbose = h.verbose
			return outCtx
		}
	}

	// Default: core agent context
	return &types.OutputContext{
		Level:   types.LevelCore,
		AgentID: "core",
		Verbose: h.verbose,
	}
}

func isVerbose() bool {
	// Check ALEX_VERBOSE env var
	verbose := os.Getenv("ALEX_VERBOSE")
	if verbose == "" {
		verbose = "false"
	}
	return verbose == "1" || verbose == "true" || verbose == "yes"
}
