package channels

import (
	"context"

	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

// AgentExecutor captures the agent execution surface needed by channel gateways.
type AgentExecutor interface {
	EnsureSession(ctx context.Context, sessionID string) (*storage.Session, error)
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}
