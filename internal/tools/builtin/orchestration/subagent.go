package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/async"
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
	shared.BaseTool
	coordinator agent.AgentCoordinator
}

// NewSubAgent creates a subagent tool with coordinator injection
func NewSubAgent(coordinator agent.AgentCoordinator, maxWorkers int) tools.ToolExecutor {
	return &subagent{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "subagent",
				Description: `Delegate complex work to dedicated subagents. Provide a single prompt or a list of tasks to run in parallel/serial. The subagent inherits the current tool access mode/preset and cannot spawn other subagents.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"prompt": {
							Type:        "string",
							Description: "Single task to delegate when tasks are not provided.",
						},
						"tasks": {
							Type:        "array",
							Description: "Optional list of subtasks to delegate to subagents.",
							Items:       &ports.Property{Type: "string"},
						},
						"mode": {
							Type:        "string",
							Description: "Execution mode for multiple tasks (parallel or serial).",
							Enum:        []any{"parallel", "serial"},
						},
						"max_parallel": {
							Type:        "integer",
							Description: "Maximum number of subtasks to run concurrently when mode=parallel.",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "subagent",
				Version:  "2.0.0",
				Category: "agent",
				Tags:     []string{"delegation", "orchestration"},
			},
		),
		coordinator: coordinator,
	}
}

func (t *subagent) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "prompt", "tasks", "mode", "max_parallel":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	prompt := ""
	if raw, ok := call.Arguments["prompt"]; ok {
		promptStr, ok := raw.(string)
		if !ok {
			return shared.ToolError(call.ID, "prompt must be a string")
		}
		prompt = strings.TrimSpace(promptStr)
	}

	tasks, err := parseStringList(call.Arguments, "tasks")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	mode, err := parseSubagentMode(call.Arguments, len(tasks))
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	maxParallel, err := parseOptionalPositiveInt(call.Arguments, "max_parallel")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if len(tasks) == 0 {
		if prompt == "" {
			return shared.ToolError(call.ID, "missing required parameter: prompt")
		}
		tasks = []string{prompt}
	}

	if mode == "" {
		if len(tasks) > 1 {
			mode = "parallel"
		} else {
			mode = "single"
		}
	}
	if len(tasks) <= 1 && mode == "parallel" {
		mode = "single"
	}

	sharedAttachments, sharedIterations := tools.GetAttachmentContext(ctx)

	// CRITICAL: Mark context as subagent execution
	// This triggers ExecutionPreparationService to use filtered registry (without subagent tool)
	// Prevents infinite recursion
	ctx = appcontext.MarkSubagentContext(ctx)

	// Propagate causal chain: set causation_id to the subagent tool call ID
	// so every subtask knows which call spawned it.
	ctx = id.WithCausationID(ctx, call.ID)

	// Extract parent listener from context if available.
	parentListener := shared.GetParentListenerFromContext(ctx)

	results := make([]SubtaskResult, len(tasks))
	collector := newAttachmentCollector(sharedAttachments)
	parallelism := resolveParallelism(mode, len(tasks), maxParallel)
	if parallelism <= 1 {
		for i, task := range tasks {
			results[i] = t.executeSubtask(ctx, task, i, len(tasks), parentListener, parallelism, sharedAttachments, sharedIterations, collector)
		}
	} else {
		jobs := make(chan int)
		var wg sync.WaitGroup
		wg.Add(len(tasks))

		for i := 0; i < parallelism; i++ {
			async.Go(nil, "subagent.parallel", func() {
				for idx := range jobs {
					results[idx] = t.executeSubtask(ctx, tasks[idx], idx, len(tasks), parentListener, parallelism, sharedAttachments, sharedIterations, collector)
					wg.Done()
				}
			})
		}

		for i := range tasks {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
	}

	if len(tasks) == 1 && results[0].Error != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Subagent execution failed: %v", results[0].Error),
			Error:   results[0].Error,
		}, nil
	}

	// Format results
	formatted, err := t.formatResults(call, tasks, results, mode)
	if err != nil {
		return nil, err
	}
	if attachments := collector.Snapshot(); len(attachments) > 0 {
		formatted.Attachments = attachments
	}
	return formatted, nil
}

func parseSubagentMode(args map[string]any, taskCount int) (string, error) {
	raw, exists := args["mode"]
	if !exists || raw == nil {
		if taskCount > 1 {
			return "parallel", nil
		}
		return "", nil
	}
	mode, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("mode must be a string")
	}
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "parallel", "serial":
		return normalized, nil
	case "":
		if taskCount > 1 {
			return "parallel", nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("unsupported mode: %s", mode)
	}
}

func parseOptionalPositiveInt(args map[string]any, key string) (int, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return 0, nil
	}
	switch v := raw.(type) {
	case int:
		if v < 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return v, nil
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return int(v), nil
	default:
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
}

func resolveParallelism(mode string, taskCount int, maxParallel int) int {
	if mode == "serial" || taskCount <= 1 {
		return 1
	}
	if mode == "" || mode == "parallel" {
		parallelism := taskCount
		if maxParallel > 0 && maxParallel < parallelism {
			parallelism = maxParallel
		}
		if parallelism < 1 {
			return 1
		}
		return parallelism
	}
	return 1
}

// SubtaskResult holds the result of a single subtask execution
type SubtaskResult struct {
	Index      int                        `json:"index"`
	Task       string                     `json:"task"`
	Answer     string                     `json:"answer,omitempty"`
	Iterations int                        `json:"iterations,omitempty"`
	TokensUsed int                        `json:"tokens_used,omitempty"`
	Workflow   *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	LogID      string                     `json:"log_id,omitempty"`
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
	collector *attachmentCollector,
) SubtaskResult {
	// Create a listener that wraps events with subtask context
	listener := newSubtaskListener(index, totalTasks, task, parentListener, maxParallel, collector)

	ids := id.IDsFromContext(ctx)
	subtaskCtx := ctx
	if ids.RunID != "" {
		subtaskCtx = id.WithParentRunID(subtaskCtx, ids.RunID)
	}
	if ids.SessionID != "" {
		subtaskCtx = id.WithSessionID(subtaskCtx, ids.SessionID)
	}
	subLogID := ""
	if ids.LogID != "" {
		subLogID = fmt.Sprintf("%s:sub:%s", ids.LogID, id.NewLogID())
		subtaskCtx = id.WithLogID(subtaskCtx, subLogID)
	} else {
		subtaskCtx, subLogID = id.EnsureLogID(subtaskCtx, id.NewLogID)
	}
	subtaskCtx = id.WithRunID(subtaskCtx, id.NewRunID())

	// Propagate correlation_id: inherit from parent or use parent's runID as root.
	if ids.CorrelationID != "" {
		subtaskCtx = id.WithCorrelationID(subtaskCtx, ids.CorrelationID)
	} else if ids.RunID != "" {
		subtaskCtx = id.WithCorrelationID(subtaskCtx, ids.RunID)
	}
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
			LogID: subLogID,
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
		LogID:      subLogID,
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
	LogID      string                     `json:"log_id,omitempty"`
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
			LogID:      r.LogID,
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
	collector      *attachmentCollector
}

func newSubtaskListener(index, total int, task string, parent agent.EventListener, maxParallel int, collector *attachmentCollector) *subtaskListener {
	// Create task preview (max 60 chars)
	taskPreview := truncatePreview(task, 60)

	return &subtaskListener{
		taskIndex:      index,
		totalTasks:     total,
		taskPreview:    taskPreview,
		parentListener: parent,
		maxParallel:    maxParallel,
		collector:      collector,
	}
}

func truncatePreview(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	cut := maxRunes - 3
	if cut < 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:cut]) + "..."
}

func (l *subtaskListener) OnEvent(event agent.AgentEvent) {
	if l.collector != nil {
		l.collector.Capture(event)
	}

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

func (e *SubtaskEvent) GetRunID() string {
	return e.OriginalEvent.GetRunID()
}

func (e *SubtaskEvent) GetParentRunID() string {
	return e.OriginalEvent.GetParentRunID()
}

func (e *SubtaskEvent) GetCorrelationID() string {
	return e.OriginalEvent.GetCorrelationID()
}

func (e *SubtaskEvent) GetCausationID() string {
	return e.OriginalEvent.GetCausationID()
}

func (e *SubtaskEvent) GetEventID() string {
	return e.OriginalEvent.GetEventID()
}

func (e *SubtaskEvent) GetSeq() uint64 {
	return e.OriginalEvent.GetSeq()
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

type attachmentCollector struct {
	mu          sync.Mutex
	attachments map[string]ports.Attachment
	inherited   map[string]ports.Attachment
}

func newAttachmentCollector(inherited map[string]ports.Attachment) *attachmentCollector {
	return &attachmentCollector{inherited: normalizeAttachmentMap(inherited)}
}

func (c *attachmentCollector) Capture(event agent.AgentEvent) {
	if c == nil || event == nil {
		return
	}
	if wrapper, ok := event.(agent.SubtaskWrapper); ok {
		event = wrapper.WrappedEvent()
	}
	carrier, ok := event.(agent.AttachmentCarrier)
	if !ok {
		return
	}
	c.merge(carrier.GetAttachments())
}

func (c *attachmentCollector) Snapshot() map[string]ports.Attachment {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.attachments) == 0 {
		return nil
	}
	return ports.CloneAttachmentMap(c.attachments)
}

func (c *attachmentCollector) merge(values map[string]ports.Attachment) {
	normalized := normalizeAttachmentMap(values)
	if len(normalized) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attachments == nil {
		c.attachments = make(map[string]ports.Attachment, len(normalized))
	}
	for name, att := range normalized {
		if c.isInherited(name, att) {
			continue
		}
		c.attachments[name] = ports.CloneAttachment(att)
	}
}

func (c *attachmentCollector) isInherited(name string, att ports.Attachment) bool {
	if len(c.inherited) == 0 {
		return false
	}
	if existing, ok := c.inherited[name]; ok {
		return attachmentsEqual(existing, att)
	}
	return false
}

func normalizeAttachmentMap(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]ports.Attachment, len(values))
	for key, att := range values {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		normalized[name] = att
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func attachmentsEqual(a, b ports.Attachment) bool {
	if a.Name != b.Name ||
		a.MediaType != b.MediaType ||
		a.Data != b.Data ||
		a.URI != b.URI ||
		a.Source != b.Source ||
		a.Description != b.Description ||
		a.Kind != b.Kind ||
		a.Format != b.Format ||
		a.PreviewProfile != b.PreviewProfile {
		return false
	}
	return previewAssetsEqual(a.PreviewAssets, b.PreviewAssets)
}

func previewAssetsEqual(a, b []ports.AttachmentPreviewAsset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
