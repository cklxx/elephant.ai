package app

import agentports "alex/internal/agent/ports"

// BaseAgentEvent unwraps any subtask wrapper to expose the underlying event so
// downstream handlers (history, metrics, streaming) can behave consistently for
// core and delegated agents.
func BaseAgentEvent(event agentports.AgentEvent) agentports.AgentEvent {
	for {
		wrapper, ok := event.(agentports.SubtaskWrapper)
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
