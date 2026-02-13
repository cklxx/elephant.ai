package coordinator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	sessiontitle "alex/internal/app/agent/sessiontitle"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports"
	storage "alex/internal/domain/agent/ports/storage"
	materialports "alex/internal/domain/materials/ports"
	"alex/internal/shared/async"
	id "alex/internal/shared/utils/id"
)

func (c *AgentCoordinator) persistSessionTitle(ctx context.Context, sessionID string, title string) {
	if c == nil || c.sessionStore == nil {
		return
	}
	title = sessiontitle.NormalizeSessionTitle(title)
	if strings.TrimSpace(sessionID) == "" || title == "" {
		return
	}

	logger := c.loggerFor(ctx)
	async.Go(logger, "session-title-update", func() {
		updateCtx := context.Background()
		if logID := id.LogIDFromContext(ctx); logID != "" {
			updateCtx = id.WithLogID(updateCtx, logID)
		}
		updateCtx, cancel := context.WithTimeout(updateCtx, 2*time.Second)
		defer cancel()

		session, err := c.sessionStore.Get(updateCtx, sessionID)
		if err != nil {
			logger.Warn("Failed to load session for title update: %v", err)
			return
		}
		if session.Metadata == nil {
			session.Metadata = make(map[string]string)
		}
		if strings.TrimSpace(session.Metadata["title"]) != "" {
			return
		}
		session.Metadata["title"] = title
		if err := c.sessionStore.Save(updateCtx, session); err != nil {
			logger.Warn("Failed to persist session title: %v", err)
		}
	})
}

// SaveSessionAfterExecution saves session state after task completion
func (c *AgentCoordinator) SaveSessionAfterExecution(ctx context.Context, session *storage.Session, result *agent.TaskResult) error {
	logger := c.loggerFor(ctx)
	historyEnabled := appcontext.SessionHistoryEnabled(ctx)
	if historyEnabled && c.historyMgr != nil && session != nil && result != nil {
		previousHistory, _ := c.historyMgr.Replay(ctx, session.ID, 0)
		incoming := append(agent.CloneMessages(previousHistory), stripUserHistoryMessages(result.Messages)...)
		if err := c.historyMgr.AppendTurn(ctx, session.ID, incoming); err != nil && logger != nil {
			logger.Warn("Failed to append turn history: %v", err)
		}
	}

	c.sessionSaveMu.Lock()
	defer c.sessionSaveMu.Unlock()

	// Update session with results
	if historyEnabled {
		sanitizedMessages, attachmentStore := sanitizeMessagesForPersistence(result.Messages)
		if c.attachmentMigrator != nil && len(attachmentStore) > 0 {
			normalized, err := c.attachmentMigrator.Normalize(ctx, materialports.MigrationRequest{
				Attachments: attachmentStore,
				Origin:      "session_persist",
			})
			if err != nil && logger != nil {
				logger.Warn("Failed to migrate attachments for session persistence: %v", err)
			} else if normalized != nil {
				attachmentStore = normalized
			}
		}
		session.Messages = sanitizedMessages
		if len(attachmentStore) > 0 {
			session.Attachments = attachmentStore
		} else {
			session.Attachments = nil
		}
		if len(result.Important) > 0 {
			session.Important = ports.CloneImportantNotes(result.Important)
		} else {
			session.Important = nil
		}
	} else {
		session.Messages = nil
		session.Attachments = nil
		session.Important = nil
	}
	session.UpdatedAt = c.clock.Now()

	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	updateAwaitUserInputMetadata(session, result)
	if result.SessionID != "" {
		session.Metadata["session_id"] = result.SessionID
	}
	if result.RunID != "" {
		session.Metadata["last_task_id"] = result.RunID
	}
	if result.ParentRunID != "" {
		session.Metadata["last_parent_task_id"] = result.ParentRunID
	} else {
		delete(session.Metadata, "last_parent_task_id")
	}

	logger.Debug("Saving session...")
	if err := c.sessionStore.Save(ctx, session); err != nil {
		logger.Error("Failed to save session: %v", err)
		return fmt.Errorf("failed to save session: %w", err)
	}
	logger.Debug("Session saved successfully")

	return nil
}

// asyncSaveSession saves session asynchronously (non-blocking) with mutex protection.
// Used for per-iteration saves to make sessions visible in diagnostics during execution.
// Errors are logged but do not fail the iteration.
func (c *AgentCoordinator) asyncSaveSession(ctx context.Context, session *storage.Session) {
	snapshot := cloneSessionForSave(session)
	if snapshot == nil {
		return
	}

	go func(saved *storage.Session) {
		c.sessionSaveMu.Lock()
		defer c.sessionSaveMu.Unlock()

		logger := c.loggerFor(ctx)
		if err := c.sessionStore.Save(ctx, saved); err != nil {
			logger.Warn("Async session save failed (non-fatal): %v", err)
		} else {
			logger.Debug("Async session save completed (session_id=%s, messages=%d)", saved.ID, len(saved.Messages))
		}
	}(snapshot)
}

func cloneSessionForSave(session *storage.Session) *storage.Session {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.Messages = agent.CloneMessages(session.Messages)
	if len(session.Todos) > 0 {
		cloned.Todos = append([]storage.Todo(nil), session.Todos...)
	} else {
		cloned.Todos = nil
	}
	if len(session.Metadata) > 0 {
		cloned.Metadata = make(map[string]string, len(session.Metadata))
		for key, value := range session.Metadata {
			cloned.Metadata[key] = value
		}
	} else {
		cloned.Metadata = nil
	}
	cloned.Attachments = ports.CloneAttachmentMap(session.Attachments)
	cloned.Important = ports.CloneImportantNotes(session.Important)
	cloned.UserPersona = ports.CloneUserPersonaProfile(session.UserPersona)
	return &cloned
}

func (c *AgentCoordinator) persistSessionSnapshot(
	ctx context.Context,
	env *agent.ExecutionEnvironment,
	fallbackRunID string,
	parentRunID string,
	stopReason string,
) {
	logger := c.loggerFor(ctx)
	if env == nil || env.State == nil || env.Session == nil {
		return
	}

	state := env.State
	result := &agent.TaskResult{
		Answer:      "",
		Messages:    state.Messages,
		Iterations:  state.Iterations,
		TokensUsed:  state.TokenCount,
		StopReason:  stopReason,
		SessionID:   state.SessionID,
		RunID:       state.RunID,
		ParentRunID: state.ParentRunID,
		Important:   ports.CloneImportantNotes(state.Important),
	}

	if result.SessionID == "" {
		result.SessionID = env.Session.ID
	}
	if result.RunID == "" {
		result.RunID = fallbackRunID
	}
	if result.ParentRunID == "" {
		result.ParentRunID = parentRunID
	}

	if err := c.SaveSessionAfterExecution(ctx, env.Session, result); err != nil {
		logger.Error("Failed to persist session after failure: %v", err)
	}
}

// ResetSession clears the session state and associated history snapshots.
func (c *AgentCoordinator) ResetSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	session, err := c.sessionStore.Get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			if c.historyMgr != nil {
				_ = c.historyMgr.ClearSession(ctx, sessionID)
			}
			return nil
		}
		return err
	}

	session.Messages = nil
	session.Metadata = nil
	session.Attachments = nil
	session.Important = nil
	session.Todos = nil
	session.UserPersona = nil
	session.UpdatedAt = c.clock.Now()

	if err := c.sessionStore.Save(ctx, session); err != nil {
		return err
	}
	if c.historyMgr != nil {
		if err := c.historyMgr.ClearSession(ctx, sessionID); err != nil {
			return err
		}
	}
	return nil
}

// GetSession retrieves or creates a session (public method)
func (c *AgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return c.getSession(ctx, id)
}

// EnsureSession returns an existing session or creates one with the provided ID.
func (c *AgentCoordinator) EnsureSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return c.sessionStore.Create(ctx)
	}
	session, err := c.sessionStore.Get(ctx, id)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, storage.ErrSessionNotFound) {
		return nil, err
	}

	now := c.clock.Now()
	session = &storage.Session{
		ID:        id,
		Messages:  []ports.Message{},
		Todos:     []storage.Todo{},
		Metadata:  map[string]string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := c.sessionStore.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (c *AgentCoordinator) getSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return c.sessionStore.Create(ctx)
	}
	return c.sessionStore.Get(ctx, id)
}

func (c *AgentCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return c.sessionStore.List(ctx, limit, offset)
}

func sanitizeAttachmentForPersistence(att ports.Attachment) ports.Attachment {
	uri := strings.TrimSpace(att.URI)
	if uri != "" && !strings.HasPrefix(strings.ToLower(uri), "data:") {
		att.Data = ""
	}
	return att
}

func sanitizeMessagesForPersistence(messages []ports.Message) ([]ports.Message, map[string]ports.Attachment) {
	if len(messages) == 0 {
		return nil, nil
	}

	sanitized := make([]ports.Message, 0, len(messages))
	attachments := make(map[string]ports.Attachment)

	for _, msg := range messages {
		if msg.Source == ports.MessageSourceUserHistory {
			continue
		}

		cloned := msg
		if len(msg.Attachments) > 0 {
			for key, att := range msg.Attachments {
				name := strings.TrimSpace(key)
				if name == "" {
					name = strings.TrimSpace(att.Name)
				}
				if name == "" {
					continue
				}
				if att.Name == "" {
					att.Name = name
				}
				attachments[name] = sanitizeAttachmentForPersistence(att)
			}
			cloned.Attachments = nil
		}
		sanitized = append(sanitized, cloned)
	}

	if len(sanitized) == 0 {
		return nil, nil
	}

	if len(attachments) == 0 {
		return sanitized, nil
	}
	return sanitized, attachments
}

func stripUserHistoryMessages(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	trimmed := make([]ports.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Source == ports.MessageSourceUserHistory {
			continue
		}
		trimmed = append(trimmed, msg)
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

func updateAwaitUserInputMetadata(session *storage.Session, result *agent.TaskResult) {
	if session == nil {
		return
	}
	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}

	stopReason := ""
	if result != nil {
		stopReason = strings.TrimSpace(result.StopReason)
	}
	if strings.EqualFold(stopReason, "await_user_input") {
		if question, ok := agent.ExtractAwaitUserInputQuestion(result.Messages); ok && strings.TrimSpace(question) != "" {
			session.Metadata["await_user_input"] = "true"
			session.Metadata["await_user_input_question"] = question
			return
		}
	}

	delete(session.Metadata, "await_user_input")
	delete(session.Metadata, "await_user_input_question")
}
