package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
	id "alex/internal/utils/id"

	"golang.org/x/sync/errgroup"
)

// subagent implements parallel task delegation via the coordinator interface
// It delegates to AgentCoordinator instead of directly creating ReactEngine
// ARCHITECTURE: This tool now properly follows hexagonal architecture by:
// - NOT importing domain layer (no internal/agent/domain)
// - NOT importing output layer (no internal/output)
// - Delegating all execution to ports.AgentCoordinator interface
// - Using agentApp.MarkSubagentContext to trigger registry filtering (RECURSION PREVENTION)
type subagent struct {
	coordinator ports.AgentCoordinator
	maxWorkers  int
}

// NewSubAgent creates a subagent tool with coordinator injection
func NewSubAgent(coordinator ports.AgentCoordinator, maxWorkers int) ports.ToolExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 3 // Default to 3 parallel workers
	}
	return &subagent{
		coordinator: coordinator,
		maxWorkers:  maxWorkers,
	}
}

func (t *subagent) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "subagent",
		Version:  "2.0.0",
		Category: "agent",
		Tags:     []string{"delegation", "parallel", "orchestration"},
	}
}

func (t *subagent) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "subagent",
		Description: `Delegate ONLY COMPLEX, TIME-CONSUMING tasks to parallel sub-agents for concurrent execution.

CRITICAL USAGE RULES:
1. Use ONLY for tasks requiring MULTIPLE INDEPENDENT operations (e.g., "analyze 5 different files", "research 3 technologies")
2. DO NOT use for:
   - Single simple operations (file read, bash command)
   - Sequential dependent tasks (use regular tools)
   - Tasks requiring shared state
3. Break down into SPECIFIC, INDEPENDENT subtasks
4. Each subtask gets a FRESH agent with FULL tool access (except nested subagent - RECURSION PREVENTION)

RECURSION PREVENTION:
- Subagents automatically have the 'subagent' tool REMOVED from their registry
- This prevents infinite nested subagent calls
- Implemented via context marking and registry filtering

EXAMPLES:

Good use cases:
- "Analyze security in auth.go, session.go, and crypto.go" → 3 parallel file analyses
- "Research React, Vue, and Angular frameworks" → 3 parallel research tasks
- "Test endpoints /api/users, /api/posts, /api/comments" → 3 parallel tests

Bad use cases:
- "Read config.json" → Just use file_read tool
- "Run tests then deploy" → Sequential, not parallel
- "Calculate total from multiple sources" → Needs state sharing

The tool executes subtasks in parallel and aggregates results.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"subtasks": {
					Type:        "array",
					Description: "List of independent subtasks to execute in parallel. Each should be a complete, self-contained task description.",
				},
				"mode": {
					Type:        "string",
					Description: "Execution mode: 'parallel' (default, faster) or 'serial' (sequential, more reliable)",
					Enum:        []any{"parallel", "serial"},
				},
			},
			Required: []string{"subtasks"},
		},
	}
}

func (t *subagent) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Parse parameters from Arguments
	subtasksRaw, ok := call.Arguments["subtasks"]
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Missing required parameter: subtasks",
			Error:   fmt.Errorf("subtasks parameter is required"),
		}, nil
	}

	// Convert to string array
	subtasksAny, ok := subtasksRaw.([]any)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Invalid subtasks parameter: must be an array",
			Error:   fmt.Errorf("subtasks must be an array"),
		}, nil
	}

	subtasks := make([]string, 0, len(subtasksAny))
	for i, t := range subtasksAny {
		taskStr, ok := t.(string)
		if !ok {
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("Invalid subtask at index %d: must be a string", i),
				Error:   fmt.Errorf("subtask %d is not a string", i),
			}, nil
		}
		subtasks = append(subtasks, taskStr)
	}

	// Validate
	if len(subtasks) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No subtasks provided",
			Error:   fmt.Errorf("subtasks array is empty"),
		}, nil
	}

	if len(subtasks) > 10 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Too many subtasks (%d). Maximum is 10 for performance reasons.", len(subtasks)),
			Error:   fmt.Errorf("too many subtasks: %d", len(subtasks)),
		}, nil
	}

	// Get mode (default to parallel)
	mode := "parallel"
	if modeRaw, ok := call.Arguments["mode"]; ok {
		if modeStr, ok := modeRaw.(string); ok {
			mode = modeStr
		}
	}

	sharedAttachments, sharedIterations := ports.GetAttachmentContext(ctx)

	// CRITICAL: Mark context as subagent execution
	// This triggers ExecutionPreparationService to use filtered registry (without subagent tool)
	// Prevents infinite recursion
	ctx = agentApp.MarkSubagentContext(ctx)

	// Extract parent listener from context if available
	var parentListener ports.EventListener
	if listener := ctx.Value(parentListenerKey{}); listener != nil {
		if pl, ok := listener.(ports.EventListener); ok {
			parentListener = pl
		}
	}

	// Execute based on mode
	var results []SubtaskResult
	var err error

	if mode == "parallel" {
		results, err = t.executeParallel(ctx, subtasks, t.maxWorkers, parentListener, sharedAttachments, sharedIterations)
	} else {
		results, err = t.executeSerial(ctx, subtasks, parentListener, sharedAttachments, sharedIterations)
	}

	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Subagent execution failed: %v", err),
			Error:   err,
		}, nil
	}

	// Format results
	return t.formatResults(call, subtasks, results, mode)
}

// SubtaskResult holds the result of a single subtask execution
type SubtaskResult struct {
	Index      int    `json:"index"`
	Task       string `json:"task"`
	Answer     string `json:"answer,omitempty"`
	Iterations int    `json:"iterations,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	Error      error  `json:"error,omitempty"`
}

// executeParallel runs subtasks concurrently using the coordinator
func (t *subagent) executeParallel(
	ctx context.Context,
	subtasks []string,
	maxWorkers int,
	parentListener ports.EventListener,
	inherited map[string]ports.Attachment,
	iterations map[string]int,
) ([]SubtaskResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxWorkers)

	results := make([]SubtaskResult, len(subtasks))
	var mu sync.Mutex

	for i, task := range subtasks {
		i, task := i, task // Capture loop variables

		g.Go(func() error {
			// Execute subtask via coordinator
			// The coordinator will see marked context and use filtered registry
			result := t.executeSubtask(ctx, task, i, len(subtasks), parentListener, maxWorkers, inherited, iterations)

			mu.Lock()
			results[i] = result
			mu.Unlock()

			return nil // Don't fail the whole group on individual task errors
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// executeSerial runs subtasks sequentially using the coordinator
func (t *subagent) executeSerial(
	ctx context.Context,
	subtasks []string,
	parentListener ports.EventListener,
	inherited map[string]ports.Attachment,
	iterations map[string]int,
) ([]SubtaskResult, error) {
	results := make([]SubtaskResult, len(subtasks))

	for i, task := range subtasks {
		results[i] = t.executeSubtask(ctx, task, i, len(subtasks), parentListener, 1, inherited, iterations)
	}

	return results, nil
}

// executeSubtask delegates a single subtask to the coordinator
// This is the KEY METHOD that replaces direct ReactEngine creation
func (t *subagent) executeSubtask(
	ctx context.Context,
	task string,
	index int,
	totalTasks int,
	parentListener ports.EventListener,
	maxParallel int,
	inherited map[string]ports.Attachment,
	iterations map[string]int,
) SubtaskResult {
	// Create a listener that wraps events with subtask context
	listener := newSubtaskListener(index, totalTasks, task, parentListener, maxParallel)

	ids := id.IDsFromContext(ctx)
	subtaskCtx := ctx
	if ids.TaskID != "" {
		subtaskCtx = id.WithParentTaskID(subtaskCtx, ids.TaskID)
	}
	if ids.SessionID != "" {
		subtaskCtx = id.WithSessionID(subtaskCtx, ids.SessionID)
	}
	subtaskCtx = id.WithTaskID(subtaskCtx, id.NewTaskID())
	if len(inherited) > 0 {
		subtaskCtx = agentApp.WithInheritedAttachments(subtaskCtx, inherited, iterations)
	}

	// Delegate to coordinator - it handles all the domain logic
	// The coordinator's ExecutionPreparationService will:
	// 1. Detect marked context via agentApp.IsSubagentContext()
	// 2. Use GetToolRegistryWithoutSubagent() to get filtered registry
	// 3. This prevents nested subagent calls (recursion prevention)
	taskResult, err := t.coordinator.ExecuteTask(subtaskCtx, task, "", listener)

	if err != nil {
		return SubtaskResult{
			Index: index,
			Task:  task,
			Error: err,
		}
	}

	return SubtaskResult{
		Index:      index,
		Task:       task,
		Answer:     taskResult.Answer,
		Iterations: taskResult.Iterations,
		TokensUsed: taskResult.TokensUsed,
	}
}

// formatResults formats subtask results for the LLM
func (t *subagent) formatResults(call ports.ToolCall, subtasks []string, results []SubtaskResult, mode string) (*ports.ToolResult, error) {
	var output strings.Builder

	// Calculate summary statistics
	successCount := 0
	failureCount := 0
	totalTokens := 0
	totalIterations := 0

	for _, r := range results {
		if r.Error == nil {
			successCount++
			totalTokens += r.TokensUsed
			totalIterations += r.Iterations
		} else {
			failureCount++
		}
	}

	// Concise output for LLM
	output.WriteString(fmt.Sprintf("Subagent completed %d/%d tasks (%s mode)\n\n", successCount, len(subtasks), mode))

	for _, r := range results {
		if r.Error != nil {
			output.WriteString(fmt.Sprintf("Task %d failed: %v\n\n", r.Index+1, r.Error))
		} else {
			output.WriteString(fmt.Sprintf("Task %d result:\n%s\n\n", r.Index+1, strings.TrimSpace(r.Answer)))
		}
	}

	// Metadata for programmatic access
	metadata := map[string]any{
		"mode":             mode,
		"total_tasks":      len(subtasks),
		"success_count":    successCount,
		"failure_count":    failureCount,
		"total_tokens":     totalTokens,
		"total_iterations": totalIterations,
	}

	// Add individual results to metadata
	resultsJSON, _ := json.Marshal(results)
	metadata["results"] = string(resultsJSON)

	return &ports.ToolResult{
		CallID:       call.ID,
		Content:      output.String(),
		Metadata:     metadata,
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
	}, nil
}

// Context key for parent listener
type parentListenerKey struct{}

// WithParentListener adds a parent listener to context for subagent event forwarding
func WithParentListener(ctx context.Context, listener ports.EventListener) context.Context {
	return context.WithValue(ctx, parentListenerKey{}, listener)
}

// subtaskListener wraps a parent listener and adds subtask context to events
type subtaskListener struct {
	taskIndex      int
	totalTasks     int
	taskPreview    string
	parentListener ports.EventListener
	maxParallel    int
}

func newSubtaskListener(index, total int, task string, parent ports.EventListener, maxParallel int) *subtaskListener {
	// Create task preview (max 60 chars)
	taskPreview := task
	if len(taskPreview) > 60 {
		taskPreview = taskPreview[:57] + "..."
	}

	return &subtaskListener{
		taskIndex:      index,
		totalTasks:     total,
		taskPreview:    taskPreview,
		parentListener: parent,
		maxParallel:    maxParallel,
	}
}

func (l *subtaskListener) OnEvent(event ports.AgentEvent) {
	// Forward event to parent listener if present
	// Parent can choose to wrap/modify the event based on subtask context
	if l.parentListener == nil {
		return
	}

	// Avoid double-wrapping if upstream already produced a subtask event
	if _, isWrapped := event.(*SubtaskEvent); isWrapped {
		l.parentListener.OnEvent(event)
		return
	}

	wrapped := &SubtaskEvent{
		OriginalEvent:  event,
		SubtaskIndex:   l.taskIndex,
		TotalSubtasks:  l.totalTasks,
		SubtaskPreview: l.taskPreview,
		MaxParallel:    l.maxParallel,
	}

	l.parentListener.OnEvent(wrapped)
}

// SubtaskEvent wraps agent events with subtask context
// This is exported for UI compatibility
type SubtaskEvent struct {
	OriginalEvent  ports.AgentEvent
	SubtaskIndex   int    // 0-based subtask index
	TotalSubtasks  int    // Total number of subtasks
	SubtaskPreview string // Short preview of the subtask (for display)
	MaxParallel    int    // Maximum number of subtasks running in parallel
}

// Implement ports.AgentEvent interface for SubtaskEvent
func (e *SubtaskEvent) EventType() string {
	if e.OriginalEvent == nil {
		return "subtask"
	}
	return e.OriginalEvent.EventType()
}

func (e *SubtaskEvent) Timestamp() time.Time {
	return e.OriginalEvent.Timestamp()
}

func (e *SubtaskEvent) GetAgentLevel() ports.AgentLevel {
	if e == nil || e.OriginalEvent == nil {
		return ports.LevelSubagent
	}
	if level := e.OriginalEvent.GetAgentLevel(); level != "" && level != ports.LevelCore {
		return level
	}
	return ports.LevelSubagent
}

func (e *SubtaskEvent) GetSessionID() string {
	return e.OriginalEvent.GetSessionID()
}

func (e *SubtaskEvent) GetTaskID() string {
	return e.OriginalEvent.GetTaskID()
}

func (e *SubtaskEvent) GetParentTaskID() string {
	return e.OriginalEvent.GetParentTaskID()
}
