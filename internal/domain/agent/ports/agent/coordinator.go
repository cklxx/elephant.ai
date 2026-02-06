package agent

import (
	"context"

	"alex/internal/agent/ports/llm"
	"alex/internal/agent/ports/storage"
	"alex/internal/agent/ports/tools"
)

// AgentCoordinator represents the main agent coordinator for subagent delegation.
type AgentCoordinator interface {
	// ExecuteTask executes a task with optional event listener and returns the result.
	ExecuteTask(ctx context.Context, task string, sessionID string, listener EventListener) (*TaskResult, error)

	// PrepareExecution prepares the execution environment (session, state, services) without running the task.
	PrepareExecution(ctx context.Context, task string, sessionID string) (*ExecutionEnvironment, error)

	// SaveSessionAfterExecution saves session state after task completion.
	SaveSessionAfterExecution(ctx context.Context, session *storage.Session, result *TaskResult) error

	// ListSessions lists available sessions with optional limit/offset pagination.
	ListSessions(ctx context.Context, limit int, offset int) ([]string, error)

	// GetConfig returns the coordinator configuration.
	GetConfig() AgentConfig

	// GetLLMClient returns an LLM client.
	GetLLMClient() (llm.LLMClient, error)

	// GetToolRegistry returns the tool registry (without subagent for nested calls).
	GetToolRegistryWithoutSubagent() tools.ToolRegistry

	// GetParser returns the function call parser.
	GetParser() tools.FunctionCallParser

	// GetContextManager returns the context manager.
	GetContextManager() ContextManager

	// GetSystemPrompt returns the system prompt.
	GetSystemPrompt() string
}
