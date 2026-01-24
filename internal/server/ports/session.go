package ports

import (
	"context"

	agentPorts "alex/internal/agent/ports"
)

// ServerSessionManager extends the basic session store with server-specific operations
type ServerSessionManager interface {
	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (*agentPorts.Session, error)

	// List returns session IDs with optional limit/offset pagination.
	List(ctx context.Context, limit int, offset int) ([]string, error)

	// Delete removes a session
	Delete(ctx context.Context, id string) error
}
