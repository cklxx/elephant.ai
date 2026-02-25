package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/workflow"
	id "alex/internal/shared/utils/id"
)

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
		a.Fingerprint != b.Fingerprint ||
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
