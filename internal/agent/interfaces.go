package agent

import (
	"context"

	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/tools/builtin"
	"alex/pkg/types"
)

// Core interfaces for agent architecture

// LLMClient defines the interface for LLM interactions
type LLMClient interface {
	Chat(ctx context.Context, req *llm.ChatRequest, sessionID string) (*llm.ChatResponse, error)
	ChatStream(ctx context.Context, req *llm.ChatRequest, sessionID string) (<-chan llm.StreamDelta, error)
}

// ToolExecutor defines the interface for tool management
type ToolExecutor interface {
	Execute(ctx context.Context, name string, args map[string]interface{}, callID string) (*types.ReactToolResult, error)
	ListTools(ctx context.Context) []string
	GetAllToolDefinitions(ctx context.Context) []llm.Tool
	GetTool(ctx context.Context, name string) (builtin.Tool, error)
}

// SessionManager defines the interface for session management
type SessionManager interface {
	StartSession(sessionID string) (*session.Session, error)
	RestoreSession(sessionID string) (*session.Session, error)
	GetCurrentSession() *session.Session
	SaveSession(sess *session.Session) error
}

// MessageProcessor defines the interface for message processing
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, message string, callback StreamCallback) (*types.ReactTaskResult, error)
	ConvertSessionToLLM(messages []*session.Message) []llm.Message
}

// ReactEngine defines the interface for ReAct processing
type ReactEngine interface {
	ProcessTask(ctx context.Context, task string, callback StreamCallback) (*types.ReactTaskResult, error)
	ExecuteTaskCore(ctx context.Context, execCtx *TaskExecutionContext, callback StreamCallback) (*types.ReactExecutionResult, error)
}