package react

import (
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// Domain-shared types used by the ReAct engine.
type (
	AgentEvent    = agent.AgentEvent
	EventListener = agent.EventListener
	Message       = ports.Message
	ToolCall      = ports.ToolCall
	ToolResult    = ports.ToolResult
	TaskState     = agent.TaskState
	TaskResult    = agent.TaskResult
	Services      = agent.ServiceBundle
)
