package postgresstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/logging"
	id "alex/internal/utils/id"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sessionTable = "agent_sessions"
)

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Store implements a Postgres-backed session store.
type Store struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

// New constructs a Postgres-backed session store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:   pool,
		logger: logging.NewComponentLogger("SessionPostgresStore"),
	}
}

// EnsureSchema creates the session table if it does not exist.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("session store not initialized")
	}

	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id TEXT PRIMARY KEY,
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    todos JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    attachments JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_updated_at ON %s (updated_at DESC);
`, sessionTable, sessionTable)

	_, err := s.pool.Exec(ctx, query)
	return err
}

// Create creates a new session row.
func (s *Store) Create(ctx context.Context) (*ports.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("session store not initialized")
	}

	for attempt := 0; attempt < 3; attempt++ {
		sessionID := id.NewSessionID()
		if !isSafeSessionID(sessionID) {
			return nil, fmt.Errorf("invalid session ID")
		}

		now := time.Now()
		session := &ports.Session{
			ID:        sessionID,
			Messages:  []ports.Message{},
			Todos:     []ports.Todo{},
			Metadata:  make(map[string]string),
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.insert(ctx, session, false); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				continue
			}
			return nil, err
		}

		return session, nil
	}

	return nil, fmt.Errorf("failed to allocate unique session ID")
}

// Get retrieves a session by ID.
func (s *Store) Get(ctx context.Context, sessionID string) (*ports.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !isSafeSessionID(sessionID) {
		return nil, fmt.Errorf("invalid session ID")
	}
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("session store not initialized")
	}

	query := fmt.Sprintf(`
SELECT id, messages, todos, metadata, attachments, created_at, updated_at
FROM %s
WHERE id = $1
`, sessionTable)

	var (
		messagesJSON    []byte
		todosJSON       []byte
		metadataJSON    []byte
		attachmentsJSON []byte
		session         ports.Session
	)

	err := s.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&messagesJSON,
		&todosJSON,
		&metadataJSON,
		&attachmentsJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}

	if len(messagesJSON) > 0 {
		if err := json.Unmarshal(messagesJSON, &session.Messages); err != nil {
			return nil, fmt.Errorf("decode messages: %w", err)
		}
	}
	if len(todosJSON) > 0 {
		if err := json.Unmarshal(todosJSON, &session.Todos); err != nil {
			return nil, fmt.Errorf("decode todos: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &session.Metadata); err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
	}
	if len(attachmentsJSON) > 0 {
		var attachments map[string]ports.Attachment
		if err := json.Unmarshal(attachmentsJSON, &attachments); err != nil {
			return nil, fmt.Errorf("decode attachments: %w", err)
		}
		session.Attachments = sanitizeAttachmentMap(attachments)
	}

	return &session, nil
}

// Save upserts a session record.
func (s *Store) Save(ctx context.Context, session *ports.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	if !isSafeSessionID(session.ID) {
		return fmt.Errorf("invalid session ID")
	}
	if s == nil || s.pool == nil {
		return fmt.Errorf("session store not initialized")
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	session.UpdatedAt = time.Now()

	return s.insert(ctx, session, true)
}

// List returns all session IDs.
func (s *Store) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("session store not initialized")
	}

	query := fmt.Sprintf(`
SELECT id
FROM %s
ORDER BY updated_at DESC
`, sessionTable)

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// Delete removes a session.
func (s *Store) Delete(ctx context.Context, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !isSafeSessionID(sessionID) {
		return fmt.Errorf("invalid session ID")
	}
	if s == nil || s.pool == nil {
		return fmt.Errorf("session store not initialized")
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, sessionTable)
	_, err := s.pool.Exec(ctx, query, sessionID)
	return err
}

func (s *Store) insert(ctx context.Context, session *ports.Session, upsert bool) error {
	messagesValue := session.Messages
	if messagesValue == nil {
		messagesValue = []ports.Message{}
	}
	todosValue := session.Todos
	if todosValue == nil {
		todosValue = []ports.Todo{}
	}
	metadataValue := session.Metadata
	if metadataValue == nil {
		metadataValue = map[string]string{}
	}

	messages, err := toJSONBytes(messagesValue)
	if err != nil {
		return fmt.Errorf("encode messages: %w", err)
	}
	todos, err := toJSONBytes(todosValue)
	if err != nil {
		return fmt.Errorf("encode todos: %w", err)
	}
	metadata, err := toJSONBytes(metadataValue)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	attachments := sanitizeAttachmentMap(session.Attachments)
	var attachmentsParam any
	if len(attachments) > 0 {
		attachmentsJSON, err := toJSONBytes(attachments)
		if err != nil {
			return fmt.Errorf("encode attachments: %w", err)
		}
		attachmentsParam = attachmentsJSON
	}

	query := fmt.Sprintf(`
INSERT INTO %s (id, messages, todos, metadata, attachments, created_at, updated_at)
VALUES ($1, $2::jsonb, $3::jsonb, $4::jsonb, $5::jsonb, $6, $7)
`, sessionTable)
	if upsert {
		query += "ON CONFLICT (id) DO UPDATE SET messages = EXCLUDED.messages, todos = EXCLUDED.todos, metadata = EXCLUDED.metadata, attachments = EXCLUDED.attachments, updated_at = EXCLUDED.updated_at"
	}

	_, err = s.pool.Exec(ctx, query,
		session.ID,
		messages,
		todos,
		metadata,
		attachmentsParam,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		logging.OrNop(s.logger).Error("Failed to persist session %s: %v", session.ID, err)
		return err
	}
	return nil
}

func toJSONBytes(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func isSafeSessionID(id string) bool {
	return sessionIDPattern.MatchString(id)
}

func sanitizeAttachmentMap(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	sanitized := make(map[string]ports.Attachment, len(values))
	for key, att := range values {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			continue
		}
		att.Name = name
		att.URI = uri
		att.Data = ""
		sanitized[name] = att
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}
