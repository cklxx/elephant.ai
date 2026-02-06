package coding

import (
	"context"
	"os"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// WorkspaceManager allocates an execution workspace for coding tasks.
type WorkspaceManager interface {
	Allocate(ctx context.Context, req TaskRequest) (agent.WorkspaceAllocation, error)
}

// SharedWorkspaceManager uses the provided working directory or cwd.
type SharedWorkspaceManager struct{}

// Allocate returns a shared workspace allocation.
func (m SharedWorkspaceManager) Allocate(_ context.Context, req TaskRequest) (agent.WorkspaceAllocation, error) {
	workingDir := strings.TrimSpace(req.WorkingDir)
	if workingDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return agent.WorkspaceAllocation{}, err
		}
		workingDir = cwd
	}
	return agent.WorkspaceAllocation{
		Mode:       agent.WorkspaceModeShared,
		WorkingDir: workingDir,
	}, nil
}
