package tui_chat

import (
	"time"
)

// Message represents a chat message in the TUI
type Message struct {
	ID        string
	Role      string // "user", "assistant", "system", "tool"
	Content   string
	Timestamp time.Time

	// For tool messages
	ToolCall *ToolCallInfo
}

// ToolCallInfo tracks tool execution state
type ToolCallInfo struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
	Result    string
	Error     error
	Status    ToolStatus
	StartTime time.Time
	Duration  time.Duration
}

// ToolStatus represents the current state of a tool execution
type ToolStatus int

const (
	ToolPending ToolStatus = iota
	ToolRunning
	ToolSuccess
	ToolError
)

// cachedMessage stores pre-rendered message content
type cachedMessage struct {
	width   int
	content string
}
