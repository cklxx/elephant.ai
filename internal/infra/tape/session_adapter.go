package tape

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	coretape "alex/internal/core/tape"
	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/storage"
)

// SessionAdapter adapts a TapeStore to satisfy storage.SessionStore for
// dual-write during migration.
type SessionAdapter struct {
	store coretape.TapeStore
}

// NewSessionAdapter returns a SessionAdapter wrapping the given TapeStore.
func NewSessionAdapter(store coretape.TapeStore) *SessionAdapter {
	return &SessionAdapter{store: store}
}

// Create creates a new session by writing an anchor entry to a new tape.
func (a *SessionAdapter) Create(ctx context.Context) (*storage.Session, error) {
	now := time.Now()
	sess := storage.NewSession(generateSessionID(), now)

	anchor := coretape.NewAnchor("session_created", coretape.EntryMeta{
		SessionID: sess.ID,
	})
	if err := a.store.Append(ctx, sess.ID, anchor); err != nil {
		return nil, fmt.Errorf("create session tape: %w", err)
	}
	return sess, nil
}

// Get reconstructs a storage.Session from tape entries.
func (a *SessionAdapter) Get(ctx context.Context, id string) (*storage.Session, error) {
	entries, err := a.store.Query(ctx, id, coretape.Query())
	if err != nil {
		return nil, fmt.Errorf("get session tape: %w", err)
	}
	if len(entries) == 0 {
		return nil, storage.ErrSessionNotFound
	}

	sess := storage.NewSession(id, entries[0].Date)

	for _, e := range entries {
		sess.UpdatedAt = e.Date

		switch e.Kind {
		case coretape.KindMessage:
			msg, err := entryToMessage(e)
			if err != nil {
				return nil, err
			}
			sess.Messages = append(sess.Messages, msg)

		case coretape.KindAnchor:
			// Anchor entries carry session metadata if present.
			if meta, ok := e.Payload["metadata"]; ok {
				if m, ok := meta.(map[string]any); ok {
					if sess.Metadata == nil {
						sess.Metadata = make(map[string]string)
					}
					for k, v := range m {
						if s, ok := v.(string); ok {
							sess.Metadata[k] = s
						}
					}
				}
			}
		}
	}

	return sess, nil
}

// Save appends message entries to the tape for any new messages.
func (a *SessionAdapter) Save(ctx context.Context, session *storage.Session) error {
	// Query existing entries to know how many messages are already persisted.
	existing, err := a.store.Query(ctx, session.ID, coretape.Query().Kinds(coretape.KindMessage))
	if err != nil {
		return fmt.Errorf("save session query: %w", err)
	}

	// If the tape doesn't exist yet, create an anchor first.
	allEntries, err := a.store.Query(ctx, session.ID, coretape.Query())
	if err != nil {
		return fmt.Errorf("save session check: %w", err)
	}
	if len(allEntries) == 0 {
		anchor := coretape.NewAnchor("session_created", coretape.EntryMeta{
			SessionID: session.ID,
		})
		if err := a.store.Append(ctx, session.ID, anchor); err != nil {
			return fmt.Errorf("save session anchor: %w", err)
		}
	}

	// Append only new messages.
	for i := len(existing); i < len(session.Messages); i++ {
		entry := messageToEntry(session.Messages[i], session.ID)
		if err := a.store.Append(ctx, session.ID, entry); err != nil {
			return fmt.Errorf("save session message %d: %w", i, err)
		}
	}
	return nil
}

// List returns tape names (session IDs) with optional pagination.
func (a *SessionAdapter) List(ctx context.Context, limit int, offset int) ([]string, error) {
	names, err := a.store.List(ctx)
	if err != nil {
		return nil, err
	}

	// Apply offset.
	if offset > 0 {
		if offset >= len(names) {
			return nil, nil
		}
		names = names[offset:]
	}

	// Apply limit.
	if limit > 0 && limit < len(names) {
		names = names[:limit]
	}
	return names, nil
}

// Delete removes a session tape.
func (a *SessionAdapter) Delete(ctx context.Context, id string) error {
	return a.store.Delete(ctx, id)
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func messageToEntry(msg ports.Message, sessionID string) coretape.TapeEntry {
	payload := map[string]any{
		"role":    msg.Role,
		"content": msg.Content,
	}
	if msg.Source != "" {
		payload["source"] = string(msg.Source)
	}
	if len(msg.ToolCalls) > 0 {
		data, _ := json.Marshal(msg.ToolCalls)
		var raw any
		_ = json.Unmarshal(data, &raw)
		payload["tool_calls"] = raw
	}
	if len(msg.ToolResults) > 0 {
		data, _ := json.Marshal(msg.ToolResults)
		var raw any
		_ = json.Unmarshal(data, &raw)
		payload["tool_results"] = raw
	}
	if msg.ToolCallID != "" {
		payload["tool_call_id"] = msg.ToolCallID
	}
	if len(msg.Attachments) > 0 {
		data, _ := json.Marshal(msg.Attachments)
		var raw any
		_ = json.Unmarshal(data, &raw)
		payload["attachments"] = raw
	}

	return coretape.NewMessage(msg.Role, msg.Content, coretape.EntryMeta{
		SessionID: sessionID,
	})
}

func entryToMessage(e coretape.TapeEntry) (ports.Message, error) {
	role, _ := e.Payload["role"].(string)
	content, _ := e.Payload["content"].(string)
	source, _ := e.Payload["source"].(string)

	msg := ports.Message{
		Role:    role,
		Content: content,
		Source:  ports.MessageSource(source),
	}

	if tcID, ok := e.Payload["tool_call_id"].(string); ok {
		msg.ToolCallID = tcID
	}

	// Reconstruct tool calls if present.
	if raw, ok := e.Payload["tool_calls"]; ok {
		data, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(data, &msg.ToolCalls)
		}
	}
	if raw, ok := e.Payload["tool_results"]; ok {
		data, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(data, &msg.ToolResults)
		}
	}
	if raw, ok := e.Payload["attachments"]; ok {
		data, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(data, &msg.Attachments)
		}
	}

	return msg, nil
}
