package app

import agent "alex/internal/agent/ports/agent"

// BaseAgentEvent unwraps any subtask wrapper to expose the underlying event so
// downstream handlers (history, metrics, streaming) can behave consistently for
// core and delegated agents.
func BaseAgentEvent(event agent.AgentEvent) agent.AgentEvent {
	for {
		wrapper, ok := event.(agent.SubtaskWrapper)
		if !ok || wrapper == nil {
			return event
		}
		inner := wrapper.WrappedEvent()
		if inner == nil {
			return event
		}
		event = inner
	}
}
