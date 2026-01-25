package app

import (
	"context"
	"errors"
	"fmt"

	"alex/internal/agent/ports"
	storage "alex/internal/agent/ports/storage"
)

// GetSession retrieves a session by ID
func (s *ServerCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return s.sessionStore.Get(ctx, id)
}

// UpdateSessionPersona updates the user persona profile for a session.
func (s *ServerCoordinator) UpdateSessionPersona(ctx context.Context, id string, persona *ports.UserPersonaProfile) (*storage.Session, error) {
	if persona == nil {
		return nil, fmt.Errorf("user persona is required")
	}
	session, err := s.sessionStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	session.UserPersona = persona
	if err := s.sessionStore.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// ListSessions returns session IDs with optional pagination.
func (s *ServerCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return s.sessionStore.List(ctx, limit, offset)
}

// CreateSession creates a new session record without executing a task.
func (s *ServerCoordinator) CreateSession(ctx context.Context) (*storage.Session, error) {
	if s.agentCoordinator == nil {
		return nil, fmt.Errorf("agent coordinator not initialized")
	}
	session, err := s.agentCoordinator.GetSession(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	if s.stateStore != nil {
		if err := s.stateStore.Init(ctx, session.ID); err != nil {
			s.logger.Warn("[ServerCoordinator] Failed to initialize state store for session %s: %v", session.ID, err)
		}
	}
	return session, nil
}

// DeleteSession removes a session
func (s *ServerCoordinator) DeleteSession(ctx context.Context, id string) error {
	var combined error
	if err := s.sessionStore.Delete(ctx, id); err != nil {
		combined = errors.Join(combined, err)
	}
	if s.stateStore != nil {
		if err := s.stateStore.ClearSession(ctx, id); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	if s.historyStore != nil {
		if err := s.historyStore.ClearSession(ctx, id); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	if s.broadcaster != nil {
		s.broadcaster.ClearEventHistory(id)
	}
	return combined
}

// ForkSession creates a new session as a fork of an existing one
func (s *ServerCoordinator) ForkSession(ctx context.Context, sessionID string) (*storage.Session, error) {
	// Get original session
	originalSession, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("original session not found: %w", err)
	}

	// Create new session
	newSession, err := s.sessionStore.Create(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create forked session: %w", err)
	}

	// Copy messages from original session
	newSession.Messages = make([]ports.Message, len(originalSession.Messages))
	copy(newSession.Messages, originalSession.Messages)

	// Copy metadata and add fork information
	newSession.Metadata = make(map[string]string)
	for k, v := range originalSession.Metadata {
		newSession.Metadata[k] = v
	}
	newSession.Metadata["forked_from"] = sessionID

	// Save the forked session
	if err := s.sessionStore.Save(ctx, newSession); err != nil {
		return nil, fmt.Errorf("failed to save forked session: %w", err)
	}

	return newSession, nil
}
