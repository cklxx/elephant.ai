package postgresstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/logging"
	id "alex/internal/utils/id"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sessionTable = "agent_sessions"

	defaultSessionCacheSize = 256
)

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Store implements a Postgres-backed session store.
type Store struct {
	pool      *pgxpool.Pool
	logger    logging.Logger
	cache     *lru.Cache[string, sessionCacheEntry]
	cacheSize int
}

type sessionCacheEntry struct {
	session   *ports.Session
	updatedAt time.Time
}

// StoreOption configures the session store.
type StoreOption func(*Store)

// WithCacheSize sets the LRU cache size. Set to 0 to disable caching.
func WithCacheSize(size int) StoreOption {
	return func(s *Store) {
		if size < 0 {
			return
		}
		s.cacheSize = size
	}
}

// New constructs a Postgres-backed session store.
func New(pool *pgxpool.Pool, opts ...StoreOption) *Store {
	store := &Store{
		pool:      pool,
		logger:    logging.NewComponentLogger("SessionPostgresStore"),
		cacheSize: defaultSessionCacheSize,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	if store.cacheSize <= 0 {
		return store
	}
	cache, err := lru.New[string, sessionCacheEntry](store.cacheSize)
	if err != nil {
		store.logger.Warn("Failed to initialize session cache: %v", err)
		return store
	}
	store.cache = cache
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

		s.storeCachedSession(session)
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

	if cached, ok, err := s.loadCachedSession(ctx, sessionID); err != nil {
		return nil, err
	} else if ok {
		return cached, nil
	}

	session, err := s.fetchSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	s.storeCachedSession(session)
	return cloneSession(session), nil
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

	if err := s.insert(ctx, session, true); err != nil {
		return err
	}
	s.storeCachedSession(session)
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
	if err == nil {
		s.deleteCachedSession(sessionID)
	}
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

func (s *Store) loadCachedSession(ctx context.Context, sessionID string) (*ports.Session, bool, error) {
	if s.cache == nil {
		return nil, false, nil
	}
	entry, ok := s.cache.Get(sessionID)
	if !ok || entry.session == nil {
		return nil, false, nil
	}

	updatedAt, err := s.fetchUpdatedAt(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.cache.Remove(sessionID)
			return nil, false, fmt.Errorf("session not found")
		}
		return nil, false, err
	}
	if updatedAt.Equal(entry.updatedAt) {
		return cloneSession(entry.session), true, nil
	}
	return nil, false, nil
}

func (s *Store) storeCachedSession(session *ports.Session) {
	if s.cache == nil || session == nil {
		return
	}
	cloned := cloneSession(session)
	if cloned == nil {
		return
	}
	s.cache.Add(session.ID, sessionCacheEntry{
		session:   cloned,
		updatedAt: session.UpdatedAt,
	})
}

func (s *Store) deleteCachedSession(sessionID string) {
	if s.cache == nil {
		return
	}
	s.cache.Remove(sessionID)
}

func (s *Store) fetchUpdatedAt(ctx context.Context, sessionID string) (time.Time, error) {
	query := fmt.Sprintf(`SELECT updated_at FROM %s WHERE id = $1`, sessionTable)
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, query, sessionID).Scan(&updatedAt)
	return updatedAt, err
}

func (s *Store) fetchSession(ctx context.Context, sessionID string) (*ports.Session, error) {
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
		session         ports.Session
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
	if len(importantJSON) > 0 {
		if err := json.Unmarshal(importantJSON, &session.Important); err != nil {
			return nil, fmt.Errorf("decode important: %w", err)
		}
	}
	if len(personaJSON) > 0 {
		var persona ports.UserPersonaProfile
		if err := json.Unmarshal(personaJSON, &persona); err != nil {
			return nil, fmt.Errorf("decode user persona: %w", err)
		}
		session.UserPersona = &persona
	}

	return &session, nil
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

func cloneSession(session *ports.Session) *ports.Session {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.Messages = ports.CloneMessages(session.Messages)
	if len(session.Todos) > 0 {
		cloned.Todos = append([]ports.Todo(nil), session.Todos...)
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
