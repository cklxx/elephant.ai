package app

import (
	"context"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
	"alex/internal/utils"
)

// ServerCoordinator coordinates task execution for the server
// It wraps AgentCoordinator and integrates with EventBroadcaster
type ServerCoordinator struct {
	agentCoordinator *agentApp.AgentCoordinator
	broadcaster      *EventBroadcaster
	sessionStore     ports.SessionStore
	logger           *utils.Logger
}

// NewServerCoordinator creates a new server coordinator
func NewServerCoordinator(
	agentCoordinator *agentApp.AgentCoordinator,
	broadcaster *EventBroadcaster,
	sessionStore ports.SessionStore,
) *ServerCoordinator {
	return &ServerCoordinator{
		agentCoordinator: agentCoordinator,
		broadcaster:      broadcaster,
		sessionStore:     sessionStore,
		logger:           utils.NewComponentLogger("ServerCoordinator"),
	}
}

// ExecuteTaskAsync executes a task asynchronously and streams events via SSE
func (s *ServerCoordinator) ExecuteTaskAsync(ctx context.Context, task string, sessionID string) (*ports.TaskResult, error) {
	s.logger.Info("ExecuteTaskAsync: task='%s', sessionID='%s'", task, sessionID)

	// Set session context in broadcaster
	ctx = s.broadcaster.SetSessionContext(ctx, sessionID)

	// Execute task with broadcaster as event listener
	result, err := s.agentCoordinator.ExecuteTask(ctx, task, sessionID, s.broadcaster)
	if err != nil {
		s.logger.Error("Task execution failed: %v", err)
		return nil, err
	}

	s.logger.Info("Task execution completed: iterations=%d", result.Iterations)
	return result, nil
}

// GetSession retrieves a session by ID
func (s *ServerCoordinator) GetSession(ctx context.Context, id string) (*ports.Session, error) {
	return s.sessionStore.Get(ctx, id)
}

// ListSessions returns all session IDs
func (s *ServerCoordinator) ListSessions(ctx context.Context) ([]string, error) {
	return s.sessionStore.List(ctx)
}

// DeleteSession removes a session
func (s *ServerCoordinator) DeleteSession(ctx context.Context, id string) error {
	return s.sessionStore.Delete(ctx, id)
}

// GetBroadcaster returns the event broadcaster
func (s *ServerCoordinator) GetBroadcaster() *EventBroadcaster {
	return s.broadcaster
}
