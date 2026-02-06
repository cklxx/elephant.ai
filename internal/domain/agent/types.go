package domain

import (
	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/agent"
)

// Re-export port contracts to keep domain API stable while sharing DTOs.
type (
	Message    = ports.Message
	ToolCall   = ports.ToolCall
	ToolResult = ports.ToolResult
	TaskState  = agent.TaskState
	TaskResult = agent.TaskResult
)
