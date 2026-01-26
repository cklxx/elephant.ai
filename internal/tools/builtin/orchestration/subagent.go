package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"
	"alex/internal/workflow"
)

// subagent implements parallel task delegation via the coordinator interface
// It delegates to AgentCoordinator instead of directly creating ReactEngine
// ARCHITECTURE: This tool now properly follows hexagonal architecture by:
// - NOT importing domain layer (no internal/agent/domain)
// - NOT importing output layer (no internal/output)
// - Delegating all execution to agent.AgentCoordinator interface
// - Using appcontext.MarkSubagentContext to trigger registry filtering (RECURSION PREVENTION)
type subagent struct {
	coordinator agent.AgentCoordinator
}

// NewSubAgent creates a subagent tool with coordinator injection
func NewSubAgent(coordinator agent.AgentCoordinator, maxWorkers int) tools.ToolExecutor {
	return &subagent{
		coordinator: coordinator,
	}
}

func (t *subagent) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "subagent",
		Version:  "2.0.0",
		Category: "agent",
		Tags:     []string{"delegation", "orchestration"},
	}
}

func (t *subagent) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "subagent",
		Description: `Delegate complex work to a dedicated subagent (single run). The subagent inherits the current tool access mode/preset and cannot spawn other subagents.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"prompt": {
					Type:        "string",
					Description: "Describe the task to delegate to the subagent.",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

func (t *subagent) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	promptRaw, ok := call.Arguments["prompt"].(string)
	if !ok || strings.TrimSpace(promptRaw) == "" {
		err := fmt.Errorf("missing required parameter: prompt")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	// Reject unexpected parameters to enforce a single prompt.
	for key := range call.Arguments {
		if key != "prompt" {
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	prompt := strings.TrimSpace(promptRaw)
	mode := "single"

	sharedAttachments, sharedIterations := tools.GetAttachmentContext(ctx)

	// CRITICAL: Mark context as subagent execution
	// This triggers ExecutionPreparationService to use filtered registry (without subagent tool)
	// Prevents infinite recursion
	ctx = appcontext.MarkSubagentContext(ctx)

	// Extract parent listener from context if available.
	parentListener := shared.GetParentListenerFromContext(ctx)

	results := make([]SubtaskResult, 1)
	results[0] = t.executeSubtask(ctx, prompt, 0, 1, parentListener, 1, sharedAttachments, sharedIterations)

	if results[0].Error != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Subagent execution failed: %v", results[0].Error),
			Error:   results[0].Error,
		}, nil
	}

	// Format results
	return t.formatResults(call, []string{prompt}, results, mode)
}

// SubtaskResult holds the result of a single subtask execution
type SubtaskResult struct {
	Index      int                        `json:"index"`
	Task       string                     `json:"task"`
	Answer     string                     `json:"answer,omitempty"`
	Iterations int                        `json:"iterations,omitempty"`
	TokensUsed int                        `json:"tokens_used,omitempty"`
	Workflow   *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	Error      error                      `json:"error,omitempty"`
}

// executeSubtask delegates a single subtask to the coordinator
// This is the KEY METHOD that replaces direct ReactEngine creation
func (t *subagent) executeSubtask(
	ctx context.Context,
	task string,
	index int,
	totalTasks int,
	parentListener agent.EventListener,
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
		subtaskCtx = appcontext.WithInheritedAttachments(subtaskCtx, inherited, iterations)
	}

	// Delegate to coordinator - it handles all the domain logic
	// The coordinator's ExecutionPreparationService will:
	// 1. Detect marked context via appcontext.IsSubagentContext()
	// 2. Use GetToolRegistryWithoutSubagent() to get filtered registry
	// 3. This prevents nested subagent calls (recursion prevention)
	taskResult, err := t.coordinator.ExecuteTask(subtaskCtx, task, ids.SessionID, listener)

	if err != nil {
		return SubtaskResult{
			Index: index,
			Task:  task,
			Workflow: func() *workflow.WorkflowSnapshot {
				if taskResult != nil {
					return taskResult.Workflow
				}
				return nil
			}(),
			Error: err,
		}
	}

	return SubtaskResult{
		Index:      index,
		Task:       task,
		Answer:     taskResult.Answer,
		Iterations: taskResult.Iterations,
		TokensUsed: taskResult.TokensUsed,
		Workflow:   taskResult.Workflow,
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

	structured := buildSubtaskMetadata(results)

	// Add individual results to metadata
	resultsJSON, _ := json.Marshal(results)
	metadata["results"] = string(resultsJSON)
	metadata["results_struct"] = structured
	metadata["workflows"] = extractWorkflows(structured)

	return &ports.ToolResult{
		CallID:       call.ID,
		Content:      output.String(),
		Metadata:     metadata,
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
	}, nil
}

type subtaskMetadata struct {
	Index      int                        `json:"index"`
	Task       string                     `json:"task"`
	Answer     string                     `json:"answer,omitempty"`
	Iterations int                        `json:"iterations,omitempty"`
	TokensUsed int                        `json:"tokens_used,omitempty"`
	Workflow   *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	Error      string                     `json:"error,omitempty"`
}

func buildSubtaskMetadata(results []SubtaskResult) []subtaskMetadata {
	structured := make([]subtaskMetadata, 0, len(results))
	for _, r := range results {
		item := subtaskMetadata{
			Index:      r.Index,
			Task:       r.Task,
			Answer:     r.Answer,
			Iterations: r.Iterations,
			TokensUsed: r.TokensUsed,
			Workflow:   r.Workflow,
		}
		if r.Error != nil {
			item.Error = r.Error.Error()
		}
		structured = append(structured, item)
	}
	return structured
}

func extractWorkflows(results []subtaskMetadata) []*workflow.WorkflowSnapshot {
	workflows := make([]*workflow.WorkflowSnapshot, 0, len(results))
	for _, r := range results {
		if r.Workflow != nil {
			workflows = append(workflows, r.Workflow)
		}
	}
	return workflows
}

// subtaskListener wraps a parent listener and adds subtask context to events
type subtaskListener struct {
	taskIndex      int
	totalTasks     int
	taskPreview    string
	parentListener agent.EventListener
	maxParallel    int
}

func newSubtaskListener(index, total int, task string, parent agent.EventListener, maxParallel int) *subtaskListener {
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

func (l *subtaskListener) OnEvent(event agent.AgentEvent) {
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
	OriginalEvent  agent.AgentEvent
	SubtaskIndex   int    // 0-based subtask index
	TotalSubtasks  int    // Total number of subtasks
	SubtaskPreview string // Short preview of the subtask (for display)
	MaxParallel    int    // Maximum number of subtasks running in parallel
}

// Implement agent.AgentEvent interface for SubtaskEvent
func (e *SubtaskEvent) EventType() string {
	if e.OriginalEvent == nil {
		return "subtask"
	}
	return e.OriginalEvent.EventType()
}

func (e *SubtaskEvent) Timestamp() time.Time {
	return e.OriginalEvent.Timestamp()
}

func (e *SubtaskEvent) GetAgentLevel() agent.AgentLevel {
	if e == nil || e.OriginalEvent == nil {
		return agent.LevelSubagent
	}
	if level := e.OriginalEvent.GetAgentLevel(); level != "" && level != agent.LevelCore {
		return level
	}
	return agent.LevelSubagent
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

// SubtaskDetails exposes metadata for downstream consumers without importing the concrete type.
func (e *SubtaskEvent) SubtaskDetails() agent.SubtaskMetadata {
	if e == nil {
		return agent.SubtaskMetadata{}
	}
	return agent.SubtaskMetadata{
		Index:       e.SubtaskIndex,
		Total:       e.TotalSubtasks,
		Preview:     e.SubtaskPreview,
		MaxParallel: e.MaxParallel,
	}
}

// WrappedEvent returns the underlying agent event carried by the subtask envelope.
func (e *SubtaskEvent) WrappedEvent() agent.AgentEvent {
	if e == nil {
		return nil
	}
	return e.OriginalEvent
}

// SetWrappedEvent updates the underlying event for sanitization pipelines.
func (e *SubtaskEvent) SetWrappedEvent(event agent.AgentEvent) {
	if e == nil {
		return
	}
	e.OriginalEvent = event
}
