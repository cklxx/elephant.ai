package app

import (
	"context"
	"time"

	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

// AgentExecutor defines the interface for agent task execution.
// This allows for easier testing and mocking.
type AgentExecutor interface {
	GetSession(ctx context.Context, id string) (*storage.Session, error)
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
	GetConfig() agent.AgentConfig
	PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error)
}

// Ensure AgentCoordinator implements AgentExecutor
var _ AgentExecutor = (*agentcoordinator.AgentCoordinator)(nil)

// ContextSnapshotRecord captures a snapshot of the messages sent to the LLM.
type ContextSnapshotRecord struct {
	SessionID   string
	RunID       string
	ParentRunID string
	RequestID   string
	Iteration   int
	Timestamp   time.Time
	Messages    []ports.Message
	Excluded    []ports.Message
}
