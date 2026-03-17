package lark

import (
	"fmt"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

// slotProgressRecorder intercepts tool lifecycle events and records short
// progress descriptions into the session slot's ring buffer. All other events
// are forwarded to the downstream listener unchanged.
type slotProgressRecorder struct {
	slot     *sessionSlot
	delegate agent.EventListener
}

func newSlotProgressRecorder(slot *sessionSlot, delegate agent.EventListener) *slotProgressRecorder {
	return &slotProgressRecorder{slot: slot, delegate: delegate}
}

func (r *slotProgressRecorder) OnEvent(event agent.AgentEvent) {
	if text := extractProgressText(event); text != "" {
		r.slot.appendProgress(text)
	}
	r.delegate.OnEvent(event)
}

// extractProgressText returns a short human-readable description for tool
// lifecycle events. Returns empty for non-tool events.
func extractProgressText(event agent.AgentEvent) string {
	e, ok := event.(*domain.Event)
	if !ok {
		return ""
	}
	switch e.Kind {
	case types.EventToolStarted:
		return fmt.Sprintf("▶ %s", e.Data.ToolName)
	case types.EventToolCompleted:
		d := e.Data.Duration.Truncate(time.Millisecond)
		if e.Data.Error != nil {
			return fmt.Sprintf("✗ %s (%s, 失败)", e.Data.ToolName, d)
		}
		return fmt.Sprintf("✓ %s (%s)", e.Data.ToolName, d)
	default:
		return ""
	}
}
