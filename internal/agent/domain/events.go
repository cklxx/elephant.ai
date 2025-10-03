package domain

import (
	"time"

	"alex/internal/agent/types"
)

// AgentEvent is the base interface for all agent events
type AgentEvent interface {
	EventType() string
	Timestamp() time.Time
	GetAgentLevel() types.AgentLevel
	GetSessionID() string
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	timestamp  time.Time
	agentLevel types.AgentLevel
	sessionID  string
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *BaseEvent) GetAgentLevel() types.AgentLevel {
	return e.agentLevel
}

func (e *BaseEvent) GetSessionID() string {
	return e.sessionID
}

func newBaseEventWithSession(level types.AgentLevel, sessionID string) BaseEvent {
	return BaseEvent{
		timestamp:  time.Now(),
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
func NewTaskAnalysisEvent(level types.AgentLevel, sessionID, actionName, goal string) *TaskAnalysisEvent {
	return &TaskAnalysisEvent{
		BaseEvent:  newBaseEventWithSession(level, sessionID),
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

// EventListener receives agent events
type EventListener interface {
	OnEvent(event AgentEvent)
}

// EventListenerFunc is a function adapter for EventListener
type EventListenerFunc func(AgentEvent)

func (f EventListenerFunc) OnEvent(event AgentEvent) {
	f(event)
}
