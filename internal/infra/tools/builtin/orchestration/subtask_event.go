package orchestration

import (
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// SubtaskEvent wraps agent events with subtask context for CLI display.
type SubtaskEvent struct {
	OriginalEvent  agent.AgentEvent
	SubtaskIndex   int    // 0-based subtask index
	TotalSubtasks  int    // Total number of subtasks
	SubtaskPreview string // Short preview of the subtask (for display)
	MaxParallel    int    // Maximum number of subtasks running in parallel
}

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

func (e *SubtaskEvent) GetSessionID() string     { return e.OriginalEvent.GetSessionID() }
func (e *SubtaskEvent) GetRunID() string          { return e.OriginalEvent.GetRunID() }
func (e *SubtaskEvent) GetParentRunID() string    { return e.OriginalEvent.GetParentRunID() }
func (e *SubtaskEvent) GetCorrelationID() string  { return e.OriginalEvent.GetCorrelationID() }
func (e *SubtaskEvent) GetCausationID() string    { return e.OriginalEvent.GetCausationID() }
func (e *SubtaskEvent) GetEventID() string        { return e.OriginalEvent.GetEventID() }
func (e *SubtaskEvent) GetSeq() uint64            { return e.OriginalEvent.GetSeq() }

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

func (e *SubtaskEvent) WrappedEvent() agent.AgentEvent {
	if e == nil {
		return nil
	}
	return e.OriginalEvent
}

func (e *SubtaskEvent) SetWrappedEvent(event agent.AgentEvent) {
	if e == nil {
		return
	}
	e.OriginalEvent = event
}
