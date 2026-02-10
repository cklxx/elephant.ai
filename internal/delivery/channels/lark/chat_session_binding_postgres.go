package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const chatSessionBindingsTable = "lark_chat_session_bindings"

// ChatSessionBindingPostgresStore persists chat->session bindings in Postgres.
type ChatSessionBindingPostgresStore struct {
	pool *pgxpool.Pool
}

// NewChatSessionBindingPostgresStore creates a Postgres-backed chat session
// binding store.
func NewChatSessionBindingPostgresStore(pool *pgxpool.Pool) *ChatSessionBindingPostgresStore {
	return &ChatSessionBindingPostgresStore{pool: pool}
}

// EnsureSchema creates required schema for chat session bindings.
func (s *ChatSessionBindingPostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("chat session binding store not initialized")
	}
	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    channel TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel, chat_id)
);`, chatSessionBindingsTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_updated_at ON %s (updated_at);`, chatSessionBindingsTable, chatSessionBindingsTable),
	}
	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure chat session binding schema: %w", err)
		}
	}
	return nil
}

// SaveBinding upserts the chat->session binding.
func (s *ChatSessionBindingPostgresStore) SaveBinding(ctx context.Context, binding ChatSessionBinding) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("chat session binding store not initialized")
	}
	channel := strings.TrimSpace(binding.Channel)
	chatID := strings.TrimSpace(binding.ChatID)
	sessionID := strings.TrimSpace(binding.SessionID)
	if channel == "" || chatID == "" || sessionID == "" {
		return fmt.Errorf("channel, chat_id and session_id are required")
	}
	updatedAt := binding.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+chatSessionBindingsTable+` (channel, chat_id, session_id, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (channel, chat_id)
DO UPDATE SET session_id = EXCLUDED.session_id,
              updated_at = EXCLUDED.updated_at
`, channel, chatID, sessionID, updatedAt)
	if err != nil {
		return fmt.Errorf("save chat session binding: %w", err)
	}
	return nil
}

// GetBinding returns the active session binding for a chat.
func (s *ChatSessionBindingPostgresStore) GetBinding(ctx context.Context, channel, chatID string) (ChatSessionBinding, bool, error) {
	if s == nil || s.pool == nil {
		return ChatSessionBinding{}, false, fmt.Errorf("chat session binding store not initialized")
	}
	channel = strings.TrimSpace(channel)
	chatID = strings.TrimSpace(chatID)
	if channel == "" || chatID == "" {
		return ChatSessionBinding{}, false, nil
	}
	var binding ChatSessionBinding
	row := s.pool.QueryRow(ctx, `
SELECT channel, chat_id, session_id, updated_at
FROM `+chatSessionBindingsTable+`
WHERE channel = $1 AND chat_id = $2
`, channel, chatID)
	if err := row.Scan(&binding.Channel, &binding.ChatID, &binding.SessionID, &binding.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return ChatSessionBinding{}, false, nil
		}
		return ChatSessionBinding{}, false, fmt.Errorf("get chat session binding: %w", err)
	}
	return binding, true, nil
}

// DeleteBinding removes the chat->session binding.
func (s *ChatSessionBindingPostgresStore) DeleteBinding(ctx context.Context, channel, chatID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("chat session binding store not initialized")
	}
	channel = strings.TrimSpace(channel)
	chatID = strings.TrimSpace(chatID)
	if channel == "" || chatID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM `+chatSessionBindingsTable+` WHERE channel = $1 AND chat_id = $2`, channel, chatID)
	if err != nil {
		return fmt.Errorf("delete chat session binding: %w", err)
	}
	return nil
}
