package state_store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"alex/internal/shared/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SnapshotKindState = "snapshot"
	SnapshotKindTurn  = "turn"
)

// PostgresStore persists snapshots in Postgres.
type PostgresStore struct {
	pool *pgxpool.Pool
	kind string
}

// NewPostgresStore creates a Postgres-backed snapshot store.
func NewPostgresStore(pool *pgxpool.Pool, kind string) *PostgresStore {
	if kind == "" {
		kind = SnapshotKindState
	}
	return &PostgresStore{pool: pool, kind: kind}
}

// EnsureSchema creates the snapshots table if it does not exist.
func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("snapshot store not initialized")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS agent_session_snapshots (
    session_id TEXT NOT NULL,
    turn_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    llm_turn_seq INTEGER NOT NULL DEFAULT 0,
    summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    PRIMARY KEY (session_id, turn_id, kind)
);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_snapshots_session_kind ON agent_session_snapshots (session_id, kind, turn_id DESC);`,
	}

	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// Init is a no-op for the Postgres store.
func (s *PostgresStore) Init(_ context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	return nil
}

// ClearSession removes persisted snapshots for the session.
func (s *PostgresStore) ClearSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	if s == nil || s.pool == nil {
		return fmt.Errorf("snapshot store not initialized")
	}

	_, err := s.pool.Exec(ctx, `DELETE FROM agent_session_snapshots WHERE session_id = $1 AND kind = $2`, sessionID, s.kind)
	return err
}

// SaveSnapshot upserts a snapshot record.
func (s *PostgresStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	if snapshot.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	if s == nil || s.pool == nil {
		return fmt.Errorf("snapshot store not initialized")
	}

	payload := snapshot
	if payload.CreatedAt.IsZero() {
		payload.CreatedAt = time.Now()
	}
	data, err := jsonx.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
INSERT INTO agent_session_snapshots (session_id, turn_id, kind, llm_turn_seq, summary, created_at, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
ON CONFLICT (session_id, turn_id, kind)
DO UPDATE SET llm_turn_seq = EXCLUDED.llm_turn_seq,
              summary = EXCLUDED.summary,
              created_at = EXCLUDED.created_at,
              payload = EXCLUDED.payload
`, snapshot.SessionID, snapshot.TurnID, s.kind, snapshot.LLMTurnSeq, snapshot.Summary, payload.CreatedAt, data)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	return nil
}

// LatestSnapshot returns the most recent snapshot.
func (s *PostgresStore) LatestSnapshot(ctx context.Context, sessionID string) (Snapshot, error) {
	metas, _, err := s.ListSnapshots(ctx, sessionID, "", 1)
	if err != nil {
		return Snapshot{}, err
	}
	if len(metas) == 0 {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return s.GetSnapshot(ctx, sessionID, metas[0].TurnID)
}

// GetSnapshot retrieves a snapshot by turn ID.
func (s *PostgresStore) GetSnapshot(ctx context.Context, sessionID string, turnID int) (Snapshot, error) {
	if sessionID == "" {
		return Snapshot{}, fmt.Errorf("session id required")
	}
	if s == nil || s.pool == nil {
		return Snapshot{}, fmt.Errorf("snapshot store not initialized")
	}

	var payload []byte
	err := s.pool.QueryRow(ctx, `
SELECT payload
FROM agent_session_snapshots
WHERE session_id = $1 AND turn_id = $2 AND kind = $3
`, sessionID, turnID, s.kind).Scan(&payload)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	if len(payload) == 0 {
		return Snapshot{}, ErrSnapshotNotFound
	}

	var snap Snapshot
	if err := jsonx.Unmarshal(payload, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	return snap, nil
}

// ListSnapshots returns metadata sorted by newest turn first.
func (s *PostgresStore) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]SnapshotMetadata, string, error) {
	if sessionID == "" {
		return nil, "", fmt.Errorf("session id required")
	}
	if s == nil || s.pool == nil {
		return nil, "", fmt.Errorf("snapshot store not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	cursorID := 0
	if cursor != "" {
		if parsed, err := strconv.Atoi(cursor); err == nil {
			cursorID = parsed
		}
	}

	args := []any{sessionID, s.kind}
	query := `
SELECT session_id, turn_id, llm_turn_seq, summary, created_at
FROM agent_session_snapshots
WHERE session_id = $1 AND kind = $2`

	if cursorID > 0 {
		query += " AND turn_id < $3"
		args = append(args, cursorID)
	}

	fetchLimit := limit + 1
	args = append(args, fetchLimit)
	query += fmt.Sprintf(" ORDER BY turn_id DESC LIMIT $%d", len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list session snapshots: %w", err)
	}
	defer rows.Close()

	metas := make([]SnapshotMetadata, 0, fetchLimit)
	for rows.Next() {
		var meta SnapshotMetadata
		if err := rows.Scan(&meta.SessionID, &meta.TurnID, &meta.LLMTurnSeq, &meta.Summary, &meta.CreatedAt); err != nil {
			return nil, "", err
		}
		metas = append(metas, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(metas) > limit {
		nextCursor = strconv.Itoa(metas[limit].TurnID)
		metas = metas[:limit]
	}

	return metas, nextCursor, nil
}
