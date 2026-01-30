package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/logging"
	sessionstate "alex/internal/session/state_store"
	id "alex/internal/utils/id"
)

// SessionService handles session CRUD and share token operations.
// Extracted from ServerCoordinator.
type SessionService struct {
	agentCoordinator AgentExecutor
	sessionStore     storage.SessionStore
	stateStore       sessionstate.Store
	historyStore     sessionstate.Store
	broadcaster      *EventBroadcaster
	logger           logging.Logger
}

// NewSessionService creates a new session service.
func NewSessionService(
	agentCoordinator AgentExecutor,
	sessionStore storage.SessionStore,
	broadcaster *EventBroadcaster,
	opts ...SessionServiceOption,
) *SessionService {
	svc := &SessionService{
		agentCoordinator: agentCoordinator,
		sessionStore:     sessionStore,
		broadcaster:      broadcaster,
		logger:           logging.NewComponentLogger("SessionService"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// SessionServiceOption configures optional behavior.
type SessionServiceOption func(*SessionService)

// WithSessionStateStore wires the state store.
func WithSessionStateStore(store sessionstate.Store) SessionServiceOption {
	return func(svc *SessionService) {
		svc.stateStore = store
	}
}

// WithSessionHistoryStore wires the history store for cleanup on delete.
func WithSessionHistoryStore(store sessionstate.Store) SessionServiceOption {
	return func(svc *SessionService) {
		svc.historyStore = store
	}
}

// GetSession retrieves a session by ID.
func (svc *SessionService) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return svc.sessionStore.Get(ctx, id)
}

// UpdateSessionPersona updates the user persona profile for a session.
func (svc *SessionService) UpdateSessionPersona(ctx context.Context, sessionID string, persona *ports.UserPersonaProfile) (*storage.Session, error) {
	if persona == nil {
		return nil, fmt.Errorf("user persona is required")
	}
	session, err := svc.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	session.UserPersona = persona
	if err := svc.sessionStore.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// ListSessions returns session IDs with optional pagination.
func (svc *SessionService) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return svc.sessionStore.List(ctx, limit, offset)
}

// CreateSession creates a new session record without executing a task.
func (svc *SessionService) CreateSession(ctx context.Context) (*storage.Session, error) {
	if svc.agentCoordinator == nil {
		return nil, fmt.Errorf("agent coordinator not initialized")
	}
	logger := logging.FromContext(ctx, svc.logger)
	session, err := svc.agentCoordinator.GetSession(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	if svc.stateStore != nil {
		if err := svc.stateStore.Init(ctx, session.ID); err != nil {
			logger.Warn("[SessionService] Failed to initialize state store for session %s: %v", session.ID, err)
		}
	}
	return session, nil
}

// DeleteSession removes a session and associated data.
func (svc *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	var combined error
	if err := svc.sessionStore.Delete(ctx, sessionID); err != nil {
		combined = errors.Join(combined, err)
	}
	if svc.stateStore != nil {
		if err := svc.stateStore.ClearSession(ctx, sessionID); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	if svc.historyStore != nil {
		if err := svc.historyStore.ClearSession(ctx, sessionID); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	if svc.broadcaster != nil {
		svc.broadcaster.ClearEventHistory(sessionID)
	}
	return combined
}

// ForkSession creates a new session as a fork of an existing one.
func (svc *SessionService) ForkSession(ctx context.Context, sessionID string) (*storage.Session, error) {
	originalSession, err := svc.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("original session not found: %w", err)
	}

	newSession, err := svc.sessionStore.Create(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create forked session: %w", err)
	}

	newSession.Messages = make([]ports.Message, len(originalSession.Messages))
	copy(newSession.Messages, originalSession.Messages)

	newSession.Metadata = make(map[string]string)
	for k, v := range originalSession.Metadata {
		newSession.Metadata[k] = v
	}
	newSession.Metadata["forked_from"] = sessionID

	if err := svc.sessionStore.Save(ctx, newSession); err != nil {
		return nil, fmt.Errorf("failed to save forked session: %w", err)
	}

	return newSession, nil
}

// EnsureSessionShareToken returns an existing share token or creates one.
func (svc *SessionService) EnsureSessionShareToken(ctx context.Context, sessionID string, reset bool) (string, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return "", fmt.Errorf("session id required")
	}

	session, err := svc.sessionStore.Get(ctx, trimmedID)
	if err != nil {
		return "", err
	}

	metadata := session.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
		session.Metadata = metadata
	}

	if !reset {
		if existing := strings.TrimSpace(metadata[shareTokenMetadataKey]); existing != "" {
			return existing, nil
		}
	}

	token := fmt.Sprintf("share-%s", id.NewKSUID())
	if token == "share-" {
		return "", fmt.Errorf("failed to generate share token")
	}

	metadata[shareTokenMetadataKey] = token
	metadata[shareEnabledMetadataKey] = "true"

	if err := svc.sessionStore.Save(ctx, session); err != nil {
		return "", err
	}

	return token, nil
}

// ValidateShareToken returns the session if the token matches.
func (svc *SessionService) ValidateShareToken(ctx context.Context, sessionID string, token string) (*storage.Session, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return nil, fmt.Errorf("session id required")
	}

	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return nil, ErrShareTokenInvalid
	}

	session, err := svc.sessionStore.Get(ctx, trimmedID)
	if err != nil {
		return nil, err
	}

	expected := ""
	if session.Metadata != nil {
		expected = strings.TrimSpace(session.Metadata[shareTokenMetadataKey])
	}
	if expected == "" || expected != trimmedToken {
		return nil, ErrShareTokenInvalid
	}

	return session, nil
}
