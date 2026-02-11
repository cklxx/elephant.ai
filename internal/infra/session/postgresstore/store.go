package postgresstore

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/json"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sessionTable = "agent_sessions"

	defaultMaxSessionMessages = 1000 // Limit session messages to prevent JSONB size overflow (256MB limit)
)

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Store implements a Postgres-backed session store.
// LRU cache removed - sessions are always read from Postgres.
// For long-running processes (Lark), sessions stay in Coordinator memory.
// For short-lived requests (Web), direct DB reads are acceptable.
type Store struct {
	pool        *pgxpool.Pool
	logger      logging.Logger
	maxMessages int // Maximum number of messages to retain in session (0 = unlimited)
}

// StoreOption configures the session store.
type StoreOption func(*Store)

// WithMaxMessages sets the maximum number of messages to retain in a session.
// Set to 0 to disable the limit (not recommended - may hit Postgres JSONB 256MB limit).
// Default is 1000 messages.
func WithMaxMessages(max int) StoreOption {
	return func(s *Store) {
		if max < 0 {
			return
		}
		s.maxMessages = max
	}
}

// New constructs a Postgres-backed session store without LRU cache.
func New(pool *pgxpool.Pool, opts ...StoreOption) *Store {
	store := &Store{
		pool:        pool,
		logger:      logging.NewComponentLogger("SessionPostgresStore"),
		maxMessages: defaultMaxSessionMessages,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	return store
}

// EnsureSchema creates the session table if it does not exist.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("session store not initialized")
	}

	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    id TEXT PRIMARY KEY,
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    todos JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    attachments JSONB,
    important JSONB,
    user_persona JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);`, sessionTable),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS important JSONB;`, sessionTable),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS user_persona JSONB;`, sessionTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_agent_sessions_updated_at ON %s (updated_at DESC);`, sessionTable),
	}

	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// Create creates a new session row.
func (s *Store) Create(ctx context.Context) (*storage.Session, error) {
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
		session := &storage.Session{
			ID:        sessionID,
			Messages:  []ports.Message{},
			Todos:     []storage.Todo{},
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

// Get retrieves a session by ID directly from Postgres.
func (s *Store) Get(ctx context.Context, sessionID string) (*storage.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !isSafeSessionID(sessionID) {
		return nil, fmt.Errorf("invalid session ID")
	}
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("session store not initialized")
	}

	session, err := s.fetchSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return cloneSession(session), nil
}

// Save upserts a session record.
func (s *Store) Save(ctx context.Context, session *storage.Session) error {
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

	// Truncate messages in-place to prevent memory accumulation.
	// This modifies the session object directly to ensure all references
	// (including cached copies) see the truncated version.
	if s.maxMessages > 0 && len(session.Messages) > s.maxMessages {
		originalCount := len(session.Messages)
		session.Messages = session.Messages[len(session.Messages)-s.maxMessages:]
		logging.OrNop(s.logger).Info(
			"Truncated session %s messages in-memory from %d to %d (limit: %d)",
			session.ID, originalCount, len(session.Messages), s.maxMessages,
		)
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	session.UpdatedAt = time.Now()

	if err := s.insert(ctx, session, true); err != nil {
		// If insertion failed due to JSONB size limit (54000: program_limit_exceeded),
		// aggressively truncate messages and retry once.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "54000" && len(session.Messages) > 100 {
			logging.OrNop(s.logger).Warn(
				"Session %s save failed due to size limit (SQLSTATE 54000), aggressively truncating to 100 messages and retrying",
				session.ID,
			)
			// Keep only the most recent 100 messages as emergency fallback
			session.Messages = session.Messages[len(session.Messages)-100:]
			if retryErr := s.insert(ctx, session, true); retryErr != nil {
				return fmt.Errorf("save failed after aggressive truncation: %w", retryErr)
			}
			return nil
		}
		return err
	}
	return nil
}

// List returns session IDs with optional limit/offset pagination.
func (s *Store) List(ctx context.Context, limit int, offset int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("session store not initialized")
	}

	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
SELECT id
FROM %s
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2
`, sessionTable)

	rows, err := s.pool.Query(ctx, query, limit, offset)
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

func (s *Store) insert(ctx context.Context, session *storage.Session, upsert bool) error {
	messagesValue := session.Messages
	if messagesValue == nil {
		messagesValue = []ports.Message{}
	}

	// Truncate messages if they exceed the limit to prevent JSONB size overflow.
	// Postgres JSONB has a hard limit of 268435455 bytes (~256MB).
	// Keeping only recent messages prevents hitting this limit while retaining context.
	if s.maxMessages > 0 && len(messagesValue) > s.maxMessages {
		originalCount := len(messagesValue)
		// Keep the most recent messages
		messagesValue = messagesValue[len(messagesValue)-s.maxMessages:]
		logging.OrNop(s.logger).Info(
			"Truncated session %s messages from %d to %d (limit: %d)",
			session.ID, originalCount, len(messagesValue), s.maxMessages,
		)
	}

	todosValue := session.Todos
	if todosValue == nil {
		todosValue = []storage.Todo{}
	}
	metadataValue := session.Metadata
	if metadataValue == nil {
		metadataValue = map[string]string{}
	}
	importantValue := session.Important
	if importantValue == nil {
		importantValue = map[string]ports.ImportantNote{}
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
	important, err := toJSONBytes(importantValue)
	if err != nil {
		return fmt.Errorf("encode important: %w", err)
	}
	var personaParam any
	if session.UserPersona != nil {
		personaJSON, err := toJSONBytes(session.UserPersona)
		if err != nil {
			return fmt.Errorf("encode user persona: %w", err)
		}
		personaParam = personaJSON
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
INSERT INTO %s (id, messages, todos, metadata, attachments, important, user_persona, created_at, updated_at)
VALUES ($1, $2::jsonb, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9)
`, sessionTable)
	if upsert {
		query += "ON CONFLICT (id) DO UPDATE SET messages = EXCLUDED.messages, todos = EXCLUDED.todos, metadata = EXCLUDED.metadata, attachments = EXCLUDED.attachments, important = EXCLUDED.important, user_persona = EXCLUDED.user_persona, updated_at = EXCLUDED.updated_at"
	}

	_, err = s.pool.Exec(ctx, query,
		session.ID,
		messages,
		todos,
		metadata,
		attachmentsParam,
		important,
		personaParam,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		logging.OrNop(s.logger).Error("Failed to persist session %s: %v", session.ID, err)
		return err
	}
	return nil
}

func (s *Store) fetchSession(ctx context.Context, sessionID string) (*storage.Session, error) {
	query := fmt.Sprintf(`
SELECT id, messages, todos, metadata, attachments, important, user_persona, created_at, updated_at
FROM %s
WHERE id = $1
`, sessionTable)

	var (
		messagesJSON    []byte
		todosJSON       []byte
		metadataJSON    []byte
		attachmentsJSON []byte
		importantJSON   []byte
		personaJSON     []byte
		session         storage.Session
	)

	err := s.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&messagesJSON,
		&todosJSON,
		&metadataJSON,
		&attachmentsJSON,
		&importantJSON,
		&personaJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrSessionNotFound
		}
		return nil, err
	}

	if len(messagesJSON) > 0 {
		if err := jsonx.Unmarshal(messagesJSON, &session.Messages); err != nil {
			return nil, fmt.Errorf("decode messages: %w", err)
		}
	}
	if len(todosJSON) > 0 {
		if err := jsonx.Unmarshal(todosJSON, &session.Todos); err != nil {
			return nil, fmt.Errorf("decode todos: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		if err := jsonx.Unmarshal(metadataJSON, &session.Metadata); err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
	}
	if len(attachmentsJSON) > 0 {
		var attachments map[string]ports.Attachment
		if err := jsonx.Unmarshal(attachmentsJSON, &attachments); err != nil {
			return nil, fmt.Errorf("decode attachments: %w", err)
		}
		session.Attachments = sanitizeAttachmentMap(attachments)
	}
	if len(importantJSON) > 0 {
		if err := jsonx.Unmarshal(importantJSON, &session.Important); err != nil {
			return nil, fmt.Errorf("decode important: %w", err)
		}
	}
	if len(personaJSON) > 0 {
		var persona ports.UserPersonaProfile
		if err := jsonx.Unmarshal(personaJSON, &persona); err != nil {
			return nil, fmt.Errorf("decode user persona: %w", err)
		}
		session.UserPersona = &persona
	}

	return &session, nil
}

func toJSONBytes(value any) ([]byte, error) {
	data, err := jsonx.Marshal(value)
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

func cloneSession(session *storage.Session) *storage.Session {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.Messages = agent.CloneMessages(session.Messages)
	if len(session.Todos) > 0 {
		cloned.Todos = append([]storage.Todo(nil), session.Todos...)
	}
	if len(session.Metadata) > 0 {
		cloned.Metadata = maps.Clone(session.Metadata)
	} else {
		cloned.Metadata = nil
	}
	cloned.Attachments = ports.CloneAttachmentMap(session.Attachments)
	cloned.Important = ports.CloneImportantNotes(session.Important)
	cloned.UserPersona = ports.CloneUserPersonaProfile(session.UserPersona)
	return &cloned
}
