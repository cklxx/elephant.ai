package domain

import (
	"time"

	"alex/internal/agent/ports"
)

// Re-export the event listener contract defined at the port layer.
type AgentEvent = ports.AgentEvent
type EventListener = ports.EventListener

// BaseEvent provides common fields for all events
type BaseEvent struct {
	timestamp  time.Time
	agentLevel ports.AgentLevel
	sessionID  string
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *BaseEvent) GetAgentLevel() ports.AgentLevel {
	return e.agentLevel
}

func (e *BaseEvent) GetSessionID() string {
	return e.sessionID
}

func newBaseEventWithSession(level ports.AgentLevel, sessionID string, ts time.Time) BaseEvent {
	return BaseEvent{
		timestamp:  ts,
		agentLevel: level,
		sessionID:  sessionID,
	}
}

// TaskAnalysisEvent - emitted after task pre-analysis
type TaskAnalysisEvent struct {
	BaseEvent
	ActionName string // e.g., "Optimizing context collection pipeline"
	Goal       string // Brief description of what needs to be achieved
}

func (e *TaskAnalysisEvent) EventType() string { return "task_analysis" }

// NewTaskAnalysisEvent creates a new task analysis event
func NewTaskAnalysisEvent(level ports.AgentLevel, sessionID, actionName, goal string, ts time.Time) *TaskAnalysisEvent {
	return &TaskAnalysisEvent{
		BaseEvent:  newBaseEventWithSession(level, sessionID, ts),
		ActionName: actionName,
		Goal:       goal,
	}
}

// IterationStartEvent - emitted at start of each ReAct iteration
type IterationStartEvent struct {
	BaseEvent
	Iteration  int
	TotalIters int
}

func (e *IterationStartEvent) EventType() string { return "iteration_start" }

// ThinkingEvent - emitted when LLM is generating response
type ThinkingEvent struct {
	BaseEvent
	Iteration    int
	MessageCount int
}

func (e *ThinkingEvent) EventType() string { return "thinking" }

// ThinkCompleteEvent - emitted when LLM response received
type ThinkCompleteEvent struct {
	BaseEvent
	Iteration     int
	Content       string
	ToolCallCount int
}

func (e *ThinkCompleteEvent) EventType() string { return "think_complete" }

// ToolCallStartEvent - emitted when tool execution begins
type ToolCallStartEvent struct {
	BaseEvent
	Iteration int
	CallID    string
	ToolName  string
	Arguments map[string]interface{}
}

func (e *ToolCallStartEvent) EventType() string { return "tool_call_start" }

// ToolCallStreamEvent - emitted during tool execution (for streaming tools)
type ToolCallStreamEvent struct {
	BaseEvent
	CallID     string
	Chunk      string
	IsComplete bool
}

func (e *ToolCallStreamEvent) EventType() string { return "tool_call_stream" }

// ToolCallCompleteEvent - emitted when tool execution finishes
type ToolCallCompleteEvent struct {
	BaseEvent
	CallID   string
	ToolName string
	Result   string
	Error    error
	Duration time.Duration
	Metadata map[string]any
}

func (e *ToolCallCompleteEvent) EventType() string { return "tool_call_complete" }

// IterationCompleteEvent - emitted at end of iteration
type IterationCompleteEvent struct {
	BaseEvent
	Iteration  int
	TokensUsed int
	ToolsRun   int
}

func (e *IterationCompleteEvent) EventType() string { return "iteration_complete" }

// TaskCompleteEvent - emitted when entire task finishes
type TaskCompleteEvent struct {
	BaseEvent
	FinalAnswer     string
	TotalIterations int
	TotalTokens     int
	StopReason      string
	Duration        time.Duration
	SessionStats    *ports.SessionStats // Optional: session-level cost/token accumulation
}

func (e *TaskCompleteEvent) EventType() string { return "task_complete" }

// ErrorEvent - emitted on errors
type ErrorEvent struct {
	BaseEvent
	Iteration   int
	Phase       string // "think", "execute", "observe"
	Error       error
	Recoverable bool
}

func (e *ErrorEvent) EventType() string { return "error" }

// ContextCompressionEvent - emitted when context is compressed
type ContextCompressionEvent struct {
	BaseEvent
	OriginalCount   int
	CompressedCount int
	CompressionRate float64 // percentage of messages retained
}

func (e *ContextCompressionEvent) EventType() string { return "context_compression" }

// NewContextCompressionEvent creates a new context compression event
func NewContextCompressionEvent(level ports.AgentLevel, sessionID string, originalCount, compressedCount int, ts time.Time) *ContextCompressionEvent {
	compressionRate := 0.0
	if originalCount > 0 {
		compressionRate = float64(compressedCount) / float64(originalCount) * 100.0
	}
	return &ContextCompressionEvent{
		BaseEvent:       newBaseEventWithSession(level, sessionID, ts),
		OriginalCount:   originalCount,
		CompressedCount: compressedCount,
		CompressionRate: compressionRate,
	}
}

// ToolFilteringEvent - emitted when tools are filtered by preset
type ToolFilteringEvent struct {
	BaseEvent
	PresetName      string
	OriginalCount   int
	FilteredCount   int
	FilteredTools   []string
	ToolFilterRatio float64 // percentage of tools retained
}

func (e *ToolFilteringEvent) EventType() string { return "tool_filtering" }

// NewToolFilteringEvent creates a new tool filtering event
func NewToolFilteringEvent(level ports.AgentLevel, sessionID, presetName string, originalCount, filteredCount int, filteredTools []string, ts time.Time) *ToolFilteringEvent {
	filterRatio := 0.0
	if originalCount > 0 {
		filterRatio = float64(filteredCount) / float64(originalCount) * 100.0
	}
	return &ToolFilteringEvent{
		BaseEvent:       newBaseEventWithSession(level, sessionID, ts),
		PresetName:      presetName,
		OriginalCount:   originalCount,
		FilteredCount:   filteredCount,
		FilteredTools:   filteredTools,
		ToolFilterRatio: filterRatio,
	}
}

// BrowserInfoEvent - emitted when sandbox browser diagnostics are captured
type BrowserInfoEvent struct {
	BaseEvent
	Success        *bool
	Message        string
	UserAgent      string
	CDPURL         string
	VNCURL         string
	ViewportWidth  int
	ViewportHeight int
	Captured       time.Time
}

func (e *BrowserInfoEvent) EventType() string { return "browser_info" }

// NewBrowserInfoEvent creates a new browser diagnostics event
func NewBrowserInfoEvent(
	level ports.AgentLevel,
	sessionID string,
	captured time.Time,
	success *bool,
	message, userAgent, cdpURL, vncURL string,
	viewportWidth, viewportHeight int,
) *BrowserInfoEvent {
	event := &BrowserInfoEvent{
		BaseEvent:      newBaseEventWithSession(level, sessionID, captured),
		Success:        success,
		Message:        message,
		UserAgent:      userAgent,
		CDPURL:         cdpURL,
		VNCURL:         vncURL,
		ViewportWidth:  viewportWidth,
		ViewportHeight: viewportHeight,
		Captured:       captured,
	}
	return event
}

// EnvironmentSnapshotEvent - emitted when host/sandbox environments are captured
type EnvironmentSnapshotEvent struct {
	BaseEvent
	Host     map[string]string
	Sandbox  map[string]string
	Captured time.Time
}

func (e *EnvironmentSnapshotEvent) EventType() string { return "environment_snapshot" }

// NewEnvironmentSnapshotEvent constructs a new environment snapshot event.
func NewEnvironmentSnapshotEvent(host, sandbox map[string]string, captured time.Time) *EnvironmentSnapshotEvent {
	return &EnvironmentSnapshotEvent{
		BaseEvent: newBaseEventWithSession(ports.LevelCore, "", captured),
		Host:      cloneStringMap(host),
		Sandbox:   cloneStringMap(sandbox),
		Captured:  captured,
	}
}

// SandboxProgressEvent captures initialization progress for the shared sandbox runtime.
type SandboxProgressEvent struct {
	BaseEvent
	Status     string
	Stage      string
	Message    string
	Step       int
	TotalSteps int
	Error      string
	Updated    time.Time
}

func (e *SandboxProgressEvent) EventType() string { return "sandbox_progress" }

// NewSandboxProgressEvent constructs a sandbox progress event.
func NewSandboxProgressEvent(status, stage, message string, step, totalSteps int, errMessage string, updated time.Time) *SandboxProgressEvent {
	return &SandboxProgressEvent{
		BaseEvent:  newBaseEventWithSession(ports.LevelCore, "", updated),
		Status:     status,
		Stage:      stage,
		Message:    message,
		Step:       step,
		TotalSteps: totalSteps,
		Error:      errMessage,
		Updated:    updated,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for k, v := range values {
		clone[k] = v
	}
	return clone
}

// EventListenerFunc is a function adapter for EventListener
type EventListenerFunc func(AgentEvent)

func (f EventListenerFunc) OnEvent(event AgentEvent) {
	f(event)
}
