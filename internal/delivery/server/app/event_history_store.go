package app

import (
	"context"

	agent "alex/internal/agent/ports/agent"
)

// EventHistoryFilter selects persisted events for replay.
type EventHistoryFilter struct {
	SessionID  string
	EventTypes []string
}

// EventHistoryStore persists events for session replay.
type EventHistoryStore interface {
	Append(ctx context.Context, event agent.AgentEvent) error
	Stream(ctx context.Context, filter EventHistoryFilter, fn func(agent.AgentEvent) error) error
	DeleteSession(ctx context.Context, sessionID string) error
	HasSessionEvents(ctx context.Context, sessionID string) (bool, error)
}
