package app

import (
	"context"

	agentports "alex/internal/agent/ports"
)

// EventHistoryFilter selects persisted events for replay.
type EventHistoryFilter struct {
	SessionID  string
	EventTypes []string
}

// EventHistoryStore persists events for session replay.
type EventHistoryStore interface {
	Append(ctx context.Context, event agentports.AgentEvent) error
	Stream(ctx context.Context, filter EventHistoryFilter, fn func(agentports.AgentEvent) error) error
	DeleteSession(ctx context.Context, sessionID string) error
	HasSessionEvents(ctx context.Context, sessionID string) (bool, error)
}
