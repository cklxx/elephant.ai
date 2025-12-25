package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists memories using the shared session database.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore constructs a Postgres-backed store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// EnsureSchema creates the table if it does not exist.
func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("memory store not initialized")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS user_memories (
    key TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    keywords TEXT[] NOT NULL DEFAULT '{}',
    slots JSONB NOT NULL DEFAULT '{}'::jsonb,
    terms TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_user_memories_user ON user_memories (user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_user_memories_terms ON user_memories USING GIN (terms);`,
	}
	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// Insert writes a new memory entry.
func (s *PostgresStore) Insert(ctx context.Context, entry Entry) (Entry, error) {
	if s == nil || s.pool == nil {
		return entry, fmt.Errorf("memory store not initialized")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.Slots == nil {
		entry.Slots = map[string]string{}
	}
	slotsJSON, err := json.Marshal(entry.Slots)
	if err != nil {
		return entry, fmt.Errorf("encode slots: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
INSERT INTO user_memories (key, user_id, content, keywords, slots, terms, created_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
ON CONFLICT (key) DO NOTHING
`, entry.Key, entry.UserID, entry.Content, entry.Keywords, slotsJSON, entry.Terms, entry.CreatedAt)
	return entry, err
}

// Search returns entries matching the supplied query terms for the user.
func (s *PostgresStore) Search(ctx context.Context, query Query) ([]Entry, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("memory store not initialized")
	}

	if len(query.Terms) == 0 {
		return nil, nil
	}

	conditions := []string{"user_id = $1"}
	args := []any{query.UserID}
	argPos := 2

	if len(query.Terms) > 0 {
		conditions = append(conditions, fmt.Sprintf("terms && $%d", argPos))
		args = append(args, query.Terms)
		argPos++
	}

	where := strings.Join(conditions, " AND ")
	querySQL := fmt.Sprintf(`
SELECT key, user_id, content, keywords, slots, terms, created_at
FROM user_memories
WHERE %s
ORDER BY created_at DESC
LIMIT $%d
`, where, argPos)
	args = append(args, query.Limit)

	rows, err := s.pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Entry
	for rows.Next() {
		var (
			entry     Entry
			slotsJSON []byte
		)
		if err := rows.Scan(&entry.Key, &entry.UserID, &entry.Content, &entry.Keywords, &slotsJSON, &entry.Terms, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if len(slotsJSON) > 0 {
			var slots map[string]string
			if err := json.Unmarshal(slotsJSON, &slots); err == nil {
				entry.Slots = slots
			}
		}
		results = append(results, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
