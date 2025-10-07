package domain

import "alex/internal/agent/ports"

// Re-export port contracts to keep domain API stable while sharing DTOs.
type (
	Message    = ports.Message
	ToolCall   = ports.ToolCall
	ToolResult = ports.ToolResult
	TaskState  = ports.TaskState
	TaskResult = ports.TaskResult
)
