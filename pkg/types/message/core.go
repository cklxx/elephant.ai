package message

import (
	"time"
)

// MessageType represents different message types
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeSystem    MessageType = "system"
	MessageTypeTool      MessageType = "tool"
)

// BaseMessage defines the core message interface
type BaseMessage interface {
	GetRole() string
	GetContent() string
	GetToolCalls() []ToolCall
	GetToolCallID() string
	GetMetadata() map[string]interface{}
	GetTimestamp() time.Time

	// Reasoning fields for advanced models
	GetReasoning() string
	GetReasoningSummary() string
	GetThink() string

	// Conversion methods
	ToLLMMessage() LLMMessage
	ToSessionMessage() SessionMessage
}

// ToolCall defines the core tool call interface
type ToolCall interface {
	GetID() string
	GetName() string
	GetArguments() map[string]interface{}
	GetArgumentsJSON() string
	GetTimestamp() time.Time

	// Conversion methods
	ToLLMToolCall() LLMToolCall
	ToSessionToolCall() SessionToolCall
}

// ToolResult defines tool execution result interface
type ToolResult interface {
	GetCallID() string
	GetToolName() string
	GetSuccess() bool
	GetContent() string
	GetData() map[string]interface{}
	GetError() string
	GetDuration() time.Duration
}

// LLMMessage represents LLM protocol message
type LLMMessage struct {
	Role             string        `json:"role"`
	Content          string        `json:"content,omitempty"`
	ToolCalls        []LLMToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
	Name             string        `json:"name,omitempty"`
	Reasoning        string        `json:"reasoning,omitempty"`
	ReasoningSummary string        `json:"reasoning_summary,omitempty"`
	Think            string        `json:"think,omitempty"`
}

// LLMToolCall represents LLM protocol tool call
type LLMToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function LLMFunction `json:"function"`
}

// LLMFunction represents LLM protocol function
type LLMFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Arguments   string      `json:"arguments,omitempty"`
}

// SessionMessage represents session storage message
type SessionMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	ToolCalls []SessionToolCall      `json:"tool_calls,omitempty"`
	ToolID    string                 `json:"tool_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// SessionToolCall represents session storage tool call
type SessionToolCall struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}
