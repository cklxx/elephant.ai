// Package task provides the Postgres-backed implementation of the unified task store.
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	taskdomain "alex/internal/domain/task"
	"alex/internal/shared/logging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	tasksTable       = "tasks"
	transitionsTable = "task_transitions"
)

// PostgresStore implements taskdomain.Store backed by Postgres.
type PostgresStore struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

var _ taskdomain.Store = (*PostgresStore)(nil)

// NewPostgresStore creates a new Postgres-backed unified task store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		pool:   pool,
		logger: logging.NewComponentLogger("UnifiedTaskStore"),
	}
}

// EnsureSchema creates the tasks and task_transitions tables if they don't exist.
func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS ` + tasksTable + ` (
    task_id            TEXT PRIMARY KEY,
    session_id         TEXT NOT NULL DEFAULT '',
    parent_task_id     TEXT NOT NULL DEFAULT '',
    channel            TEXT NOT NULL DEFAULT '',
    chat_id            TEXT NOT NULL DEFAULT '',
    user_id            TEXT NOT NULL DEFAULT '',
    description        TEXT NOT NULL DEFAULT '',
    prompt             TEXT NOT NULL DEFAULT '',
    agent_type         TEXT NOT NULL DEFAULT 'internal',
    agent_preset       TEXT NOT NULL DEFAULT '',
    tool_preset        TEXT NOT NULL DEFAULT '',
    execution_mode     TEXT NOT NULL DEFAULT '',
    autonomy_level     TEXT NOT NULL DEFAULT '',
    workspace_mode     TEXT NOT NULL DEFAULT '',
    working_dir        TEXT NOT NULL DEFAULT '',
    config             JSONB,
    status             TEXT NOT NULL DEFAULT 'pending',
    termination_reason TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at         TIMESTAMPTZ,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at       TIMESTAMPTZ,
    current_iteration  INTEGER NOT NULL DEFAULT 0,
    total_iterations   INTEGER NOT NULL DEFAULT 0,
    tokens_used        INTEGER NOT NULL DEFAULT 0,
    cost_usd           DOUBLE PRECISION NOT NULL DEFAULT 0,
    answer_preview     TEXT NOT NULL DEFAULT '',
    result_json        JSONB,
    plan_json          JSONB,
    retry_attempt      INTEGER NOT NULL DEFAULT 0,
    parent_plan_task_id TEXT NOT NULL DEFAULT '',
    error              TEXT NOT NULL DEFAULT '',
	    depends_on         TEXT[] NOT NULL DEFAULT '{}',
	    bridge_meta        JSONB,
	    metadata           JSONB,
	    owner_id           TEXT NOT NULL DEFAULT '',
	    lease_until        TIMESTAMPTZ,
	    lease_updated_at   TIMESTAMPTZ
	)`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS lease_updated_at TIMESTAMPTZ`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS execution_mode TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS autonomy_level TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS plan_json JSONB`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS retry_attempt INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE ` + tasksTable + ` ADD COLUMN IF NOT EXISTS parent_plan_task_id TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_session ON ` + tasksTable + ` (session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_chat_status ON ` + tasksTable + ` (chat_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON ` + tasksTable + ` (status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_created ON ` + tasksTable + ` (created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_channel ON ` + tasksTable + ` (channel)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status_lease ON ` + tasksTable + ` (status, lease_until)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_owner ON ` + tasksTable + ` (owner_id)`,

		`CREATE TABLE IF NOT EXISTS ` + transitionsTable + ` (
    id            BIGSERIAL PRIMARY KEY,
    task_id       TEXT NOT NULL REFERENCES ` + tasksTable + `(task_id) ON DELETE CASCADE,
    from_status   TEXT NOT NULL DEFAULT '',
    to_status     TEXT NOT NULL DEFAULT '',
    reason        TEXT NOT NULL DEFAULT '',
    metadata_json JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
)`,
		`CREATE INDEX IF NOT EXISTS idx_task_transitions_task ON ` + transitionsTable + ` (task_id, created_at)`,
	}

	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure task schema: %w", err)
		}
	}
	return nil
}

// Create inserts a new task record.
func (s *PostgresStore) Create(ctx context.Context, task *taskdomain.Task) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	if task.TaskID == "" {
		return fmt.Errorf("task_id required")
	}

	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if task.Status == "" {
		task.Status = taskdomain.StatusPending
	}

	metadataJSON := marshalJSONOrNil(task.Metadata)
	bridgeMetaJSON := marshalJSONOrNil(task.BridgeMeta)

	dependsOn := task.DependsOn
	if dependsOn == nil {
		dependsOn = []string{}
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tasksTable+` (
    task_id, session_id, parent_task_id, channel, chat_id, user_id,
    description, prompt, agent_type, agent_preset, tool_preset, execution_mode, autonomy_level,
    workspace_mode, working_dir, config, status, termination_reason,
    created_at, started_at, updated_at, completed_at,
	    current_iteration, total_iterations, tokens_used, cost_usd,
	    answer_preview, result_json, plan_json, retry_attempt, parent_plan_task_id, error, depends_on, bridge_meta, metadata,
	    owner_id, lease_until, lease_updated_at
) VALUES (
	    $1, $2, $3, $4, $5, $6,
	    $7, $8, $9, $10, $11, $12, $13,
	    $14, $15, $16, $17, $18,
	    $19, $20, $21, $22,
	    $23, $24, $25, $26,
	    $27, $28, $29, $30, $31, $32, $33, $34, $35,
	    $36, $37, $38
)`,
		task.TaskID, task.SessionID, task.ParentTaskID, task.Channel, task.ChatID, task.UserID,
		task.Description, task.Prompt, task.AgentType, task.AgentPreset, task.ToolPreset, task.ExecutionMode, task.AutonomyLevel,
		task.WorkspaceMode, task.WorkingDir, nullableRawJSON(task.Config), string(task.Status), string(task.TerminationReason),
		task.CreatedAt, task.StartedAt, task.UpdatedAt, task.CompletedAt,
		task.CurrentIteration, task.TotalIterations, task.TokensUsed, task.CostUSD,
		task.AnswerPreview, nullableRawJSON(task.ResultJSON), nullableRawJSON(task.PlanJSON), task.RetryAttempt, task.ParentPlanTaskID, task.Error, dependsOn, bridgeMetaJSON, metadataJSON,
		"", nil, nil,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// Record initial transition (best-effort).
	s.recordTransition(ctx, task.TaskID, "", string(task.Status), "created", nil, task.CreatedAt)
	return nil
}

// Get retrieves a task by ID.
func (s *PostgresStore) Get(ctx context.Context, taskID string) (*taskdomain.Task, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}

	row := s.pool.QueryRow(ctx, selectAllColumns()+` FROM `+tasksTable+` WHERE task_id = $1`, taskID)
	t, err := scanTask(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("task %s: not found", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

// SetStatus updates the task status and writes a transition record in a single transaction.
func (s *PostgresStore) SetStatus(ctx context.Context, taskID string, status taskdomain.Status, opts ...taskdomain.TransitionOption) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	p := taskdomain.ApplyTransitionOptions(opts)
	now := time.Now()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Read current status under lock.
	var fromStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM `+tasksTable+` WHERE task_id = $1 FOR UPDATE`, taskID).Scan(&fromStatus)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("task %s: not found", taskID)
	}
	if err != nil {
		return fmt.Errorf("read current status: %w", err)
	}

	// Build dynamic UPDATE.
	setClauses := `status = $2, updated_at = $3`
	args := []any{taskID, string(status), now}
	idx := 4

	if status.IsTerminal() {
		setClauses += fmt.Sprintf(`, completed_at = COALESCE(completed_at, $%d)`, idx)
		args = append(args, now)
		idx++

		reason := terminationReasonForStatus(status)
		setClauses += fmt.Sprintf(`, termination_reason = CASE WHEN termination_reason = '' THEN $%d ELSE termination_reason END`, idx)
		args = append(args, string(reason))
		idx++

		setClauses += `, owner_id = '', lease_until = NULL, lease_updated_at = NULL`
	}

	if status == taskdomain.StatusRunning {
		setClauses += fmt.Sprintf(`, started_at = COALESCE(started_at, $%d)`, idx)
		args = append(args, now)
		idx++
	}

	if p.AnswerPreview != nil {
		setClauses += fmt.Sprintf(`, answer_preview = $%d`, idx)
		args = append(args, *p.AnswerPreview)
		idx++
	}
	if p.ErrorText != nil {
		setClauses += fmt.Sprintf(`, error = $%d`, idx)
		args = append(args, *p.ErrorText)
		idx++
	}
	if p.TokensUsed != nil {
		setClauses += fmt.Sprintf(`, tokens_used = $%d`, idx)
		args = append(args, *p.TokensUsed)
		idx++
	}
	if meta := stringifyMetadata(p.Metadata); len(meta) > 0 {
		metaJSON := marshalJSONOrNil(meta)
		if metaJSON != nil {
			setClauses += fmt.Sprintf(`, metadata = COALESCE(metadata, '{}'::jsonb) || $%d`, idx)
			args = append(args, metaJSON)
		}
	}

	_, err = tx.Exec(ctx, `UPDATE `+tasksTable+` SET `+setClauses+` WHERE task_id = $1`, args...)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	// Write transition record.
	transMetaJSON := marshalJSONOrNil(p.Metadata)
	_, err = tx.Exec(ctx, `
INSERT INTO `+transitionsTable+` (task_id, from_status, to_status, reason, metadata_json, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`,
		taskID, fromStatus, string(status), p.Reason, transMetaJSON, now)
	if err != nil {
		return fmt.Errorf("record transition: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateProgress updates iteration and token counts.
func (s *PostgresStore) UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int, costUSD float64) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET current_iteration = $2, tokens_used = $3, cost_usd = $4, updated_at = $5
WHERE task_id = $1`,
		taskID, iteration, tokensUsed, costUSD, time.Now())
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task %s: not found", taskID)
	}
	return nil
}

// SetResult stores the completion result.
func (s *PostgresStore) SetResult(ctx context.Context, taskID string, answer string, resultJSON json.RawMessage, tokensUsed int) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	now := time.Now()
	preview := answer
	if len(preview) > 500 {
		preview = preview[:500]
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET status = $2, termination_reason = $3, answer_preview = $4, result_json = $5,
    tokens_used = $6, total_iterations = current_iteration,
    completed_at = COALESCE(completed_at, $7), updated_at = $7,
    owner_id = '', lease_until = NULL, lease_updated_at = NULL
WHERE task_id = $1`,
		taskID, string(taskdomain.StatusCompleted), string(taskdomain.TerminationCompleted),
		preview, nullableRawJSON(resultJSON), tokensUsed, now)
	if err != nil {
		return fmt.Errorf("set result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task %s: not found", taskID)
	}

	s.recordTransition(ctx, taskID, "running", string(taskdomain.StatusCompleted), "result set", nil, now)
	return nil
}

// SetError records a task failure.
func (s *PostgresStore) SetError(ctx context.Context, taskID string, errText string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	now := time.Now()
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET status = $2, termination_reason = $3, error = $4,
    completed_at = COALESCE(completed_at, $5), updated_at = $5,
    owner_id = '', lease_until = NULL, lease_updated_at = NULL
WHERE task_id = $1`,
		taskID, string(taskdomain.StatusFailed), string(taskdomain.TerminationError), errText, now)
	if err != nil {
		return fmt.Errorf("set error: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task %s: not found", taskID)
	}

	s.recordTransition(ctx, taskID, "running", string(taskdomain.StatusFailed), errText, nil, now)
	return nil
}

// SetBridgeMeta persists bridge checkpoint data.
func (s *PostgresStore) SetBridgeMeta(ctx context.Context, taskID string, meta taskdomain.BridgeMeta) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal bridge_meta: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET bridge_meta = $2, updated_at = $3
WHERE task_id = $1`,
		taskID, metaJSON, time.Now())
	if err != nil {
		return fmt.Errorf("set bridge_meta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task %s: not found", taskID)
	}
	return nil
}

// TryClaimTask attempts to claim ownership for a task. The claim succeeds when
// the task is unowned, already owned by ownerID, or the prior lease expired.
func (s *PostgresStore) TryClaimTask(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	if s == nil || s.pool == nil {
		return false, fmt.Errorf("task store not initialized")
	}
	if taskID == "" {
		return false, fmt.Errorf("task_id required")
	}
	if ownerID == "" {
		return false, fmt.Errorf("owner_id required")
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET owner_id = $2, lease_until = $3, lease_updated_at = now(), updated_at = now()
WHERE task_id = $1
  AND status IN ('pending', 'running', 'waiting_input')
  AND (
    owner_id = '' OR owner_id = $2 OR lease_until IS NULL OR lease_until < now()
  )`,
		taskID, ownerID, leaseUntil)
	if err != nil {
		return false, fmt.Errorf("try claim task: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ClaimResumableTasks atomically claims tasks in statuses for resumption and
// returns the claimed rows.
func (s *PostgresStore) ClaimResumableTasks(ctx context.Context, ownerID string, leaseUntil time.Time, limit int, statuses ...taskdomain.Status) ([]*taskdomain.Task, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id required")
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	statusStrs := make([]string, len(statuses))
	for i, st := range statuses {
		statusStrs[i] = string(st)
	}

	rows, err := s.pool.Query(ctx, `
WITH candidates AS (
    SELECT task_id
    FROM `+tasksTable+`
    WHERE status = ANY($1)
      AND (owner_id = '' OR owner_id = $2 OR lease_until IS NULL OR lease_until < now())
    ORDER BY created_at DESC
    LIMIT $4
    FOR UPDATE SKIP LOCKED
)
UPDATE `+tasksTable+` AS t
SET owner_id = $2, lease_until = $3, lease_updated_at = now(), updated_at = now()
FROM candidates AS c
WHERE t.task_id = c.task_id
RETURNING `+allColumns(),
		statusStrs, ownerID, leaseUntil, limit)
	if err != nil {
		return nil, fmt.Errorf("claim resumable tasks: %w", err)
	}
	return collectTasks(rows)
}

// RenewTaskLease extends lease_until for an owned task.
func (s *PostgresStore) RenewTaskLease(ctx context.Context, taskID, ownerID string, leaseUntil time.Time) (bool, error) {
	if s == nil || s.pool == nil {
		return false, fmt.Errorf("task store not initialized")
	}
	if taskID == "" {
		return false, fmt.Errorf("task_id required")
	}
	if ownerID == "" {
		return false, fmt.Errorf("owner_id required")
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET lease_until = $3, lease_updated_at = now(), updated_at = now()
WHERE task_id = $1 AND owner_id = $2`,
		taskID, ownerID, leaseUntil)
	if err != nil {
		return false, fmt.Errorf("renew task lease: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ReleaseTaskLease clears owner/lease metadata for a task.
func (s *PostgresStore) ReleaseTaskLease(ctx context.Context, taskID, ownerID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	if taskID == "" || ownerID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET owner_id = '', lease_until = NULL, lease_updated_at = NULL, updated_at = now()
WHERE task_id = $1 AND owner_id = $2`, taskID, ownerID)
	if err != nil {
		return fmt.Errorf("release task lease: %w", err)
	}
	return nil
}

// ListBySession returns tasks for a session, newest first.
func (s *PostgresStore) ListBySession(ctx context.Context, sessionID string, limit int) ([]*taskdomain.Task, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, selectAllColumns()+`
FROM `+tasksTable+` WHERE session_id = $1
ORDER BY created_at DESC LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list by session: %w", err)
	}
	return collectTasks(rows)
}

// ListByChat returns tasks for a chat, optionally filtered to active-only.
func (s *PostgresStore) ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]*taskdomain.Task, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	if chatID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	query := selectAllColumns() + ` FROM ` + tasksTable + ` WHERE chat_id = $1`
	if activeOnly {
		query += ` AND status IN ('pending', 'running', 'waiting_input')`
	}
	query += ` ORDER BY created_at DESC LIMIT $2`

	rows, err := s.pool.Query(ctx, query, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("list by chat: %w", err)
	}
	return collectTasks(rows)
}

// ListByStatus returns tasks matching any of the given statuses.
func (s *PostgresStore) ListByStatus(ctx context.Context, statuses ...taskdomain.Status) ([]*taskdomain.Task, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}
	if len(statuses) == 0 {
		return nil, nil
	}

	statusStrs := make([]string, len(statuses))
	for i, st := range statuses {
		statusStrs[i] = string(st)
	}

	rows, err := s.pool.Query(ctx, selectAllColumns()+`
FROM `+tasksTable+` WHERE status = ANY($1)
ORDER BY created_at DESC`, statusStrs)
	if err != nil {
		return nil, fmt.Errorf("list by status: %w", err)
	}
	return collectTasks(rows)
}

// ListActive returns all non-terminal tasks.
func (s *PostgresStore) ListActive(ctx context.Context) ([]*taskdomain.Task, error) {
	return s.ListByStatus(ctx, taskdomain.StatusPending, taskdomain.StatusRunning, taskdomain.StatusWaitingInput)
}

// List returns paginated tasks, newest first.
func (s *PostgresStore) List(ctx context.Context, limit int, offset int) ([]*taskdomain.Task, int, error) {
	if s == nil || s.pool == nil {
		return nil, 0, fmt.Errorf("task store not initialized")
	}
	if limit <= 0 {
		limit = 50
	}

	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM `+tasksTable).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	rows, err := s.pool.Query(ctx, selectAllColumns()+`
FROM `+tasksTable+` ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}

	tasks, err := collectTasks(rows)
	if err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// Delete removes a task.
func (s *PostgresStore) Delete(ctx context.Context, taskID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM `+tasksTable+` WHERE task_id = $1`, taskID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task %s: not found", taskID)
	}
	return nil
}

// Transitions returns the audit trail for a task.
func (s *PostgresStore) Transitions(ctx context.Context, taskID string) ([]taskdomain.Transition, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("task store not initialized")
	}

	rows, err := s.pool.Query(ctx, `
SELECT id, task_id, from_status, to_status, reason, metadata_json, created_at
FROM `+transitionsTable+` WHERE task_id = $1 ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list transitions: %w", err)
	}
	defer rows.Close()

	var transitions []taskdomain.Transition
	for rows.Next() {
		var t taskdomain.Transition
		var fromStr, toStr string
		var metaJSON []byte
		if err := rows.Scan(&t.ID, &t.TaskID, &fromStr, &toStr, &t.Reason, &metaJSON, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan transition: %w", err)
		}
		t.FromStatus = taskdomain.Status(fromStr)
		t.ToStatus = taskdomain.Status(toStr)
		if metaJSON != nil {
			t.MetadataJSON = metaJSON
		}
		transitions = append(transitions, t)
	}
	return transitions, rows.Err()
}

// MarkStaleRunning marks all running/pending tasks as failed.
func (s *PostgresStore) MarkStaleRunning(ctx context.Context, reason string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}

	now := time.Now()
	_, err := s.pool.Exec(ctx, `
UPDATE `+tasksTable+`
SET status = 'failed', termination_reason = 'error', error = $1,
    updated_at = $2, completed_at = COALESCE(completed_at, $2),
    owner_id = '', lease_until = NULL, lease_updated_at = NULL
WHERE status IN ('pending', 'running', 'waiting_input')`,
		reason, now)
	if err != nil {
		return fmt.Errorf("mark stale running: %w", err)
	}
	return nil
}

// DeleteExpired removes tasks completed before the given time.
func (s *PostgresStore) DeleteExpired(ctx context.Context, before time.Time) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("task store not initialized")
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM `+tasksTable+` WHERE created_at < $1`, before)
	if err != nil {
		return fmt.Errorf("delete expired: %w", err)
	}
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func selectAllColumns() string {
	return `SELECT ` + allColumns()
}

func allColumns() string {
	return `task_id, session_id, parent_task_id, channel, chat_id, user_id,
	       description, prompt, agent_type, agent_preset, tool_preset, execution_mode, autonomy_level,
	       workspace_mode, working_dir, config, status, termination_reason,
	       created_at, started_at, updated_at, completed_at,
	       current_iteration, total_iterations, tokens_used, cost_usd,
	       answer_preview, result_json, plan_json, retry_attempt, parent_plan_task_id, error, depends_on, bridge_meta, metadata`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (*taskdomain.Task, error) {
	var t taskdomain.Task
	var (
		configJSON     []byte
		resultJSON     []byte
		planJSON       []byte
		bridgeMetaJSON []byte
		metadataJSON   []byte
		status         string
		termReason     string
	)

	err := row.Scan(
		&t.TaskID, &t.SessionID, &t.ParentTaskID, &t.Channel, &t.ChatID, &t.UserID,
		&t.Description, &t.Prompt, &t.AgentType, &t.AgentPreset, &t.ToolPreset, &t.ExecutionMode, &t.AutonomyLevel,
		&t.WorkspaceMode, &t.WorkingDir, &configJSON, &status, &termReason,
		&t.CreatedAt, &t.StartedAt, &t.UpdatedAt, &t.CompletedAt,
		&t.CurrentIteration, &t.TotalIterations, &t.TokensUsed, &t.CostUSD,
		&t.AnswerPreview, &resultJSON, &planJSON, &t.RetryAttempt, &t.ParentPlanTaskID, &t.Error, &t.DependsOn, &bridgeMetaJSON, &metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	t.Status = taskdomain.Status(status)
	t.TerminationReason = taskdomain.TerminationReason(termReason)

	if configJSON != nil {
		t.Config = configJSON
	}
	if resultJSON != nil {
		t.ResultJSON = resultJSON
	}
	if planJSON != nil {
		t.PlanJSON = planJSON
	}
	if bridgeMetaJSON != nil {
		var bm taskdomain.BridgeMeta
		if json.Unmarshal(bridgeMetaJSON, &bm) == nil {
			t.BridgeMeta = &bm
		}
	}
	if metadataJSON != nil {
		var md map[string]string
		if json.Unmarshal(metadataJSON, &md) == nil {
			t.Metadata = md
		}
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}

	return &t, nil
}

func collectTasks(rows pgx.Rows) ([]*taskdomain.Task, error) {
	defer rows.Close()

	var tasks []*taskdomain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func marshalJSONOrNil(v any) []byte {
	if v == nil {
		return nil
	}
	switch m := v.(type) {
	case map[string]string:
		if len(m) == 0 {
			return nil
		}
	case map[string]any:
		if len(m) == 0 {
			return nil
		}
	case *taskdomain.BridgeMeta:
		if m == nil {
			return nil
		}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func stringifyMetadata(meta map[string]any) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func nullableRawJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

func terminationReasonForStatus(status taskdomain.Status) taskdomain.TerminationReason {
	switch status {
	case taskdomain.StatusCompleted:
		return taskdomain.TerminationCompleted
	case taskdomain.StatusCancelled:
		return taskdomain.TerminationCancelled
	case taskdomain.StatusFailed:
		return taskdomain.TerminationError
	default:
		return taskdomain.TerminationNone
	}
}

func (s *PostgresStore) recordTransition(ctx context.Context, taskID, from, to, reason string, meta []byte, at time.Time) {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+transitionsTable+` (task_id, from_status, to_status, reason, metadata_json, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`,
		taskID, from, to, reason, meta, at)
	if err != nil {
		s.logger.Warn("failed to record transition %s→%s for %s: %v", from, to, taskID, err)
	}
}
