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
			// Anchor entries carry session metadata and todos if present.
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
			if todosRaw, ok := e.Payload["todos"]; ok {
				d, err := json.Marshal(todosRaw)
				if err == nil {
					var todos []storage.Todo
					if json.Unmarshal(d, &todos) == nil {
						sess.Todos = todos
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

	// Persist metadata and todos so Get can reconstruct them.
	if len(session.Metadata) > 0 || len(session.Todos) > 0 {
		payload := map[string]any{"label": "session_metadata"}
		if len(session.Metadata) > 0 {
			m := make(map[string]any, len(session.Metadata))
			for k, v := range session.Metadata {
				m[k] = v
			}
			payload["metadata"] = m
		}
		if len(session.Todos) > 0 {
			d, _ := json.Marshal(session.Todos)
			var raw any
			_ = json.Unmarshal(d, &raw)
			payload["todos"] = raw
		}
		metaEntry := coretape.TapeEntry{
			ID:      fmt.Sprintf("meta_%d", time.Now().UnixNano()),
			Kind:    coretape.KindAnchor,
			Payload: payload,
			Meta:    coretape.EntryMeta{SessionID: session.ID},
			Date:    time.Now(),
		}
		if err := a.store.Append(ctx, session.ID, metaEntry); err != nil {
			return fmt.Errorf("save session metadata: %w", err)
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
		payload["tool_calls"] = jsonRoundTrip(msg.ToolCalls)
	}
	if len(msg.ToolResults) > 0 {
		payload["tool_results"] = jsonRoundTrip(msg.ToolResults)
	}
	if msg.ToolCallID != "" {
		payload["tool_call_id"] = msg.ToolCallID
	}
	if len(msg.Attachments) > 0 {
		payload["attachments"] = jsonRoundTrip(msg.Attachments)
	}
	if len(msg.Thinking.Parts) > 0 {
		payload["thinking"] = jsonRoundTrip(msg.Thinking)
	}
	if len(msg.Metadata) > 0 {
		payload["metadata"] = jsonRoundTrip(msg.Metadata)
	}
	return coretape.NewMessageFromPayload(payload, coretape.EntryMeta{
		SessionID: sessionID,
	})
}

// jsonRoundTrip marshals v to JSON and back to map[string]any so the payload
// contains only primitive types that survive JSONL serialization.
func jsonRoundTrip(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(data, &out)
	return out
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

	jsonUnmarshalField(e.Payload, "tool_calls", &msg.ToolCalls)
	jsonUnmarshalField(e.Payload, "tool_results", &msg.ToolResults)
	jsonUnmarshalField(e.Payload, "attachments", &msg.Attachments)
	jsonUnmarshalField(e.Payload, "thinking", &msg.Thinking)
	jsonUnmarshalField(e.Payload, "metadata", &msg.Metadata)

	return msg, nil
}

// jsonUnmarshalField extracts a payload key via JSON round-trip into dest.
func jsonUnmarshalField(payload map[string]any, key string, dest any) {
	raw, ok := payload[key]
	if !ok {
		return
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, dest)
}
