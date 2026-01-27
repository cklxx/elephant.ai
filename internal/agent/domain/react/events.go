package react

import (
	"context"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	id "alex/internal/utils/id"
)

// SetEventListener configures event emission for TUI/streaming
func (e *ReactEngine) SetEventListener(listener EventListener) {
	e.eventListener = listener
}

// GetEventListener returns the current event listener (for saving/restoring)
func (e *ReactEngine) GetEventListener() EventListener {
	return e.eventListener
}

// getAgentLevel reads the current agent level from context
func (e *ReactEngine) getAgentLevel(ctx context.Context) agent.AgentLevel {
	if ctx == nil {
		return agent.LevelCore
	}
	outCtx := agent.GetOutputContext(ctx)
	if outCtx == nil {
		return agent.LevelCore
	}
	return outCtx.Level
}

// emitEvent sends event to listener if one is set
func (e *ReactEngine) emitEvent(event AgentEvent) {
	if e.eventListener != nil {
		e.logger.Debug("[emitEvent] Emitting event type=%s, sessionID=%s to listener", event.EventType(), event.GetSessionID())
		e.eventListener.OnEvent(event)
		e.logger.Debug("[emitEvent] Event emitted successfully")
	} else {
		e.logger.Debug("[emitEvent] No listener set, skipping event type=%s", event.EventType())
	}
}

func (e *ReactEngine) newBaseEvent(ctx context.Context, sessionID, taskID, parentTaskID string) domain.BaseEvent {
	base := domain.NewBaseEvent(e.getAgentLevel(ctx), sessionID, taskID, parentTaskID, e.clock.Now())
	if logID := id.LogIDFromContext(ctx); logID != "" {
		base.SetLogID(logID)
	}
	return base
}
