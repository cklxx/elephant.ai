package lark

import (
	"context"
	"fmt"
	"time"

	"alex/internal/shared/logging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const taskRegistryTable = "lark_task_registry"

// TaskPostgresStore persists task records in Postgres.
type TaskPostgresStore struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

// NewTaskPostgresStore constructs a Postgres-backed task store.
func NewTaskPostgresStore(pool *pgxpool.Pool) *TaskPostgresStore {
	return &TaskPostgresStore{
		pool:   pool,
		logger: logging.NewComponentLogger("LarkTaskStore"),
	}
}

// EnsureSchema creates the task registry table if it does not exist.
func (s *TaskPostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    task_id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    agent_type TEXT NOT NULL DEFAULT 'internal',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    answer_preview TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    tokens_used INTEGER NOT NULL DEFAULT 0
);`, taskRegistryTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_chat_status ON %s (chat_id, status);`, taskRegistryTable, taskRegistryTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_created ON %s (created_at);`, taskRegistryTable, taskRegistryTable),
	}
	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure task schema: %w", err)
		}
	}
	return nil
}

// SaveTask inserts a new task record (upsert on task_id).
func (s *TaskPostgresStore) SaveTask(ctx context.Context, task TaskRecord) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	if task.TaskID == "" || task.ChatID == "" {
		return fmt.Errorf("task_id and chat_id required")
	}
	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if task.Status == "" {
		task.Status = "pending"
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO `+taskRegistryTable+` (task_id, chat_id, user_id, agent_type, description, status, created_at, updated_at, answer_preview, error, tokens_used)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (task_id)
DO UPDATE SET chat_id = EXCLUDED.chat_id,
              user_id = EXCLUDED.user_id,
              agent_type = EXCLUDED.agent_type,
              description = EXCLUDED.description,
              status = EXCLUDED.status,
              updated_at = EXCLUDED.updated_at,
              answer_preview = EXCLUDED.answer_preview,
              error = EXCLUDED.error,
              tokens_used = EXCLUDED.tokens_used
`, task.TaskID, task.ChatID, task.UserID, task.AgentType, task.Description,
		task.Status, task.CreatedAt, task.UpdatedAt, task.AnswerPreview, task.Error, task.TokensUsed)
	if err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// UpdateStatus updates the status (and optional fields) for a task.
func (s *TaskPostgresStore) UpdateStatus(ctx context.Context, taskID, status string, opts ...TaskUpdateOption) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	if taskID == "" {
		return fmt.Errorf("task_id required")
	}

	var o taskUpdateOptions
	for _, fn := range opts {
		fn(&o)
	}

	now := time.Now()
	var completedAt *time.Time
	if status == "completed" || status == "failed" || status == "cancelled" {
		completedAt = &now
	}

	_, err := s.pool.Exec(ctx, `
UPDATE `+taskRegistryTable+`
SET status = $2, updated_at = $3, completed_at = COALESCE($4, completed_at),
    answer_preview = CASE WHEN $5 = '' THEN answer_preview ELSE $5 END,
    error = CASE WHEN $6 = '' THEN error ELSE $6 END,
    tokens_used = CASE WHEN $7 = 0 THEN tokens_used ELSE $7 END
WHERE task_id = $1
`, taskID, status, now, completedAt, o.answerPreview, o.errorText, o.tokensUsed)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	return nil
}

// GetTask fetches a single task by ID.
func (s *TaskPostgresStore) GetTask(ctx context.Context, taskID string) (TaskRecord, bool, error) {
	if s == nil || s.pool == nil {
		return TaskRecord{}, false, fmt.Errorf("task store not initialized")
	}
	if taskID == "" {
		return TaskRecord{}, false, nil
	}

	var rec TaskRecord
	var completedAt *time.Time
	row := s.pool.QueryRow(ctx, `
SELECT task_id, chat_id, user_id, agent_type, description, status,
       created_at, updated_at, completed_at, answer_preview, error, tokens_used
FROM `+taskRegistryTable+`
WHERE task_id = $1
`, taskID)
	if err := row.Scan(&rec.TaskID, &rec.ChatID, &rec.UserID, &rec.AgentType,
		&rec.Description, &rec.Status, &rec.CreatedAt, &rec.UpdatedAt,
		&completedAt, &rec.AnswerPreview, &rec.Error, &rec.TokensUsed); err != nil {
		if err == pgx.ErrNoRows {
			return TaskRecord{}, false, nil
		}
		return TaskRecord{}, false, fmt.Errorf("get task: %w", err)
	}
	if completedAt != nil {
		rec.CompletedAt = *completedAt
	}
	return rec, true, nil
}

// ListByChat returns tasks for a chat, optionally filtered to active-only.
func (s *TaskPostgresStore) ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]TaskRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	if chatID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT task_id, chat_id, user_id, agent_type, description, status,
       created_at, updated_at, completed_at, answer_preview, error, tokens_used
FROM ` + taskRegistryTable + ` WHERE chat_id = $1`
	if activeOnly {
		query += ` AND status IN ('pending', 'running', 'waiting_input')`
	}
	query += ` ORDER BY created_at DESC LIMIT $2`

	rows, err := s.pool.Query(ctx, query, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []TaskRecord
	for rows.Next() {
		var rec TaskRecord
		var completedAt *time.Time
		if err := rows.Scan(&rec.TaskID, &rec.ChatID, &rec.UserID, &rec.AgentType,
			&rec.Description, &rec.Status, &rec.CreatedAt, &rec.UpdatedAt,
			&completedAt, &rec.AnswerPreview, &rec.Error, &rec.TokensUsed); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if completedAt != nil {
			rec.CompletedAt = *completedAt
		}
		tasks = append(tasks, rec)
	}
	return tasks, rows.Err()
}

// DeleteExpired removes task records older than the given time.
func (s *TaskPostgresStore) DeleteExpired(ctx context.Context, before time.Time) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM `+taskRegistryTable+` WHERE created_at < $1`, before)
	if err != nil {
		return fmt.Errorf("delete expired tasks: %w", err)
	}
	return nil
}

// MarkStaleRunning marks all running/pending tasks as failed (used on gateway restart).
func (s *TaskPostgresStore) MarkStaleRunning(ctx context.Context, reason string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
UPDATE `+taskRegistryTable+`
SET status = 'failed', error = $1, updated_at = $2, completed_at = $2
WHERE status IN ('pending', 'running', 'waiting_input')
`, reason, now)
	if err != nil {
		return fmt.Errorf("mark stale running: %w", err)
	}
	return nil
}
