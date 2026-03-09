package coding

import (
	"context"

	agent "alex/internal/domain/agent/ports/agent"
)

// WorkspaceManager allocates an execution workspace for coding tasks.
type WorkspaceManager interface {
	Allocate(ctx context.Context, req TaskRequest) (agent.WorkspaceAllocation, error)
}
