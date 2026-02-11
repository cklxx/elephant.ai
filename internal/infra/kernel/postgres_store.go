// Package kernel provides the Postgres-backed dispatch queue for the kernel agent loop.
package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dispatchTable = "kernel_dispatch_tasks"

// PostgresStore implements kerneldomain.Store backed by Postgres.
type PostgresStore struct {
	pool          *pgxpool.Pool
	leaseDuration time.Duration
	logger        logging.Logger
}

var _ kerneldomain.Store = (*PostgresStore)(nil)

const defaultLeaseDuration = 30 * time.Minute

// NewPostgresStore creates a new Postgres-backed dispatch store.
// leaseDuration controls how long a claimed dispatch is held before it can be
// reclaimed. If zero, defaults to 30 minutes.
func NewPostgresStore(pool *pgxpool.Pool, leaseDuration time.Duration) *PostgresStore {
	if leaseDuration <= 0 {
		leaseDuration = defaultLeaseDuration
	}
	return &PostgresStore{
		pool:          pool,
		leaseDuration: leaseDuration,
		logger:        logging.NewKernelLogger("KernelDispatchStore"),
	}
}

// EnsureSchema creates the dispatch table and indices if they do not exist.
func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("kernel dispatch store not initialized")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS ` + dispatchTable + ` (
    dispatch_id TEXT PRIMARY KEY,
    kernel_id   TEXT NOT NULL,
    cycle_id    TEXT NOT NULL,
    agent_id    TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 5,
    status      TEXT NOT NULL DEFAULT 'pending',
    lease_owner TEXT,
    lease_until TIMESTAMPTZ,
    task_id     TEXT,
    error       TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)`,
		`CREATE INDEX IF NOT EXISTS idx_kernel_dispatch_status
    ON ` + dispatchTable + ` (kernel_id, status, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_kernel_dispatch_lease
    ON ` + dispatchTable + ` (lease_until) WHERE lease_until IS NOT NULL`,
	}

	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure kernel dispatch schema: %w", err)
		}
	}
	return nil
}

// EnqueueDispatches inserts a batch of dispatch specs as pending rows within a
// single transaction. On partial INSERT failure the entire batch is rolled back
// to avoid orphaned rows.
func (s *PostgresStore) EnqueueDispatches(ctx context.Context, kernelID, cycleID string, specs []kerneldomain.DispatchSpec) ([]kerneldomain.Dispatch, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin enqueue tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // best-effort on defer

	now := time.Now().UTC()
	dispatches := make([]kerneldomain.Dispatch, 0, len(specs))

	for _, spec := range specs {
		dispatchID := id.NewRunID()
		metaJSON, err := json.Marshal(spec.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata for %s: %w", spec.AgentID, err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO `+dispatchTable+` (dispatch_id, kernel_id, cycle_id, agent_id, prompt, priority, status, metadata, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			dispatchID, kernelID, cycleID, spec.AgentID, spec.Prompt, spec.Priority,
			string(kerneldomain.DispatchPending), metaJSON, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("enqueue dispatch %s: %w", spec.AgentID, err)
		}

		dispatches = append(dispatches, kerneldomain.Dispatch{
			DispatchID: dispatchID,
			KernelID:   kernelID,
			CycleID:    cycleID,
			AgentID:    spec.AgentID,
			Prompt:     spec.Prompt,
			Priority:   spec.Priority,
			Status:     kerneldomain.DispatchPending,
			Metadata:   spec.Metadata,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit enqueue tx: %w", err)
	}
	return dispatches, nil
}

// ClaimDispatches atomically claims up to limit pending dispatches using FOR UPDATE SKIP LOCKED.
func (s *PostgresStore) ClaimDispatches(ctx context.Context, kernelID, workerID string, limit int) ([]kerneldomain.Dispatch, error) {
	leaseUntil := time.Now().UTC().Add(s.leaseDuration)

	rows, err := s.pool.Query(ctx,
		`UPDATE `+dispatchTable+` SET
			status = $1, lease_owner = $2, lease_until = $3, updated_at = now()
		WHERE dispatch_id IN (
			SELECT dispatch_id FROM `+dispatchTable+`
			WHERE kernel_id = $4 AND status = $5
			ORDER BY priority DESC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $6
		)
		RETURNING dispatch_id, kernel_id, cycle_id, agent_id, prompt, priority, status,
		          lease_owner, lease_until, task_id, error, metadata, created_at, updated_at`,
		string(kerneldomain.DispatchRunning), workerID, leaseUntil,
		kernelID, string(kerneldomain.DispatchPending), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim dispatches: %w", err)
	}
	defer rows.Close()

	return scanDispatches(rows)
}

// MarkDispatchRunning sets the dispatch status to running.
func (s *PostgresStore) MarkDispatchRunning(ctx context.Context, dispatchID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE `+dispatchTable+` SET status = $1, updated_at = now() WHERE dispatch_id = $2`,
		string(kerneldomain.DispatchRunning), dispatchID,
	)
	return err
}

// MarkDispatchDone sets the dispatch status to done with the resulting task ID.
func (s *PostgresStore) MarkDispatchDone(ctx context.Context, dispatchID, taskID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE `+dispatchTable+` SET status = $1, task_id = $2, updated_at = now() WHERE dispatch_id = $3`,
		string(kerneldomain.DispatchDone), taskID, dispatchID,
	)
	return err
}

// MarkDispatchFailed sets the dispatch status to failed with an error message.
func (s *PostgresStore) MarkDispatchFailed(ctx context.Context, dispatchID, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE `+dispatchTable+` SET status = $1, error = $2, updated_at = now() WHERE dispatch_id = $3`,
		string(kerneldomain.DispatchFailed), errMsg, dispatchID,
	)
	return err
}

// ListActiveDispatches returns all non-terminal dispatches for a kernel.
func (s *PostgresStore) ListActiveDispatches(ctx context.Context, kernelID string) ([]kerneldomain.Dispatch, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT dispatch_id, kernel_id, cycle_id, agent_id, prompt, priority, status,
		        lease_owner, lease_until, task_id, error, metadata, created_at, updated_at
		 FROM `+dispatchTable+`
		 WHERE kernel_id = $1 AND status IN ($2, $3)
		 ORDER BY created_at DESC`,
		kernelID, string(kerneldomain.DispatchPending), string(kerneldomain.DispatchRunning),
	)
	if err != nil {
		return nil, fmt.Errorf("list active dispatches: %w", err)
	}
	defer rows.Close()

	return scanDispatches(rows)
}

// ListRecentByAgent returns the most recent dispatch for each agent_id.
func (s *PostgresStore) ListRecentByAgent(ctx context.Context, kernelID string) (map[string]kerneldomain.Dispatch, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT ON (agent_id)
		        dispatch_id, kernel_id, cycle_id, agent_id, prompt, priority, status,
		        lease_owner, lease_until, task_id, error, metadata, created_at, updated_at
		 FROM `+dispatchTable+`
		 WHERE kernel_id = $1
		 ORDER BY agent_id, updated_at DESC`,
		kernelID,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent by agent: %w", err)
	}
	defer rows.Close()

	dispatches, err := scanDispatches(rows)
	if err != nil {
		return nil, err
	}

	result := make(map[string]kerneldomain.Dispatch, len(dispatches))
	for _, d := range dispatches {
		result[d.AgentID] = d
	}
	return result, nil
}

// pgxRows abstracts pgx row iteration for scanning.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanDispatches(rows pgxRows) ([]kerneldomain.Dispatch, error) {
	var dispatches []kerneldomain.Dispatch
	for rows.Next() {
		var d kerneldomain.Dispatch
		var metaJSON []byte
		var leaseOwner, taskID, errMsg *string
		var leaseUntil *time.Time

		if err := rows.Scan(
			&d.DispatchID, &d.KernelID, &d.CycleID, &d.AgentID,
			&d.Prompt, &d.Priority, &d.Status,
			&leaseOwner, &leaseUntil, &taskID, &errMsg,
			&metaJSON, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return dispatches, fmt.Errorf("scan dispatch: %w", err)
		}

		if leaseOwner != nil {
			d.LeaseOwner = *leaseOwner
		}
		if leaseUntil != nil {
			d.LeaseUntil = leaseUntil
		}
		if taskID != nil {
			d.TaskID = *taskID
		}
		if errMsg != nil {
			d.Error = *errMsg
		}
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			_ = json.Unmarshal(metaJSON, &d.Metadata)
		}

		dispatches = append(dispatches, d)
	}
	return dispatches, rows.Err()
}
