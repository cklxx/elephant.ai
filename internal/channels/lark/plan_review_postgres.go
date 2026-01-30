package lark

import (
	"context"
	"fmt"
	"time"

	"alex/internal/jsonx"
	"alex/internal/logging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	planReviewPendingTable = "lark_plan_review_pending"
	defaultPlanReviewTTL   = 60 * time.Minute
)

// PlanReviewPostgresStore persists pending plan review state in Postgres.
type PlanReviewPostgresStore struct {
	pool   *pgxpool.Pool
	ttl    time.Duration
	logger logging.Logger
}

// NewPlanReviewPostgresStore constructs a Postgres-backed plan review store.
// TODO: For high QPS, consider moving this pending state to a KV store.
func NewPlanReviewPostgresStore(pool *pgxpool.Pool, ttl time.Duration) *PlanReviewPostgresStore {
	if ttl <= 0 {
		ttl = defaultPlanReviewTTL
	}
	return &PlanReviewPostgresStore{
		pool:   pool,
		ttl:    ttl,
		logger: logging.NewComponentLogger("LarkPlanReviewStore"),
	}
}

// EnsureSchema creates the pending table if it does not exist.
func (s *PlanReviewPostgresStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("plan review store not initialized")
	}
	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    user_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    overall_goal_ui TEXT NOT NULL,
    internal_plan JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, chat_id)
);`, planReviewPendingTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_expires ON %s (expires_at);`, planReviewPendingTable, planReviewPendingTable),
	}
	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// SavePending upserts a pending plan review record.
func (s *PlanReviewPostgresStore) SavePending(ctx context.Context, pending PlanReviewPending) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("plan review store not initialized")
	}
	if pending.UserID == "" || pending.ChatID == "" {
		return fmt.Errorf("user_id and chat_id required")
	}
	now := time.Now()
	if pending.CreatedAt.IsZero() {
		pending.CreatedAt = now
	}
	if pending.ExpiresAt.IsZero() {
		pending.ExpiresAt = now.Add(s.ttl)
	}
	var planJSON []byte
	if pending.InternalPlan != nil {
		data, err := jsonx.Marshal(pending.InternalPlan)
		if err != nil {
			return fmt.Errorf("marshal internal_plan: %w", err)
		}
		planJSON = data
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO `+planReviewPendingTable+` (user_id, chat_id, run_id, overall_goal_ui, internal_plan, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
ON CONFLICT (user_id, chat_id)
DO UPDATE SET run_id = EXCLUDED.run_id,
              overall_goal_ui = EXCLUDED.overall_goal_ui,
              internal_plan = EXCLUDED.internal_plan,
              created_at = EXCLUDED.created_at,
              expires_at = EXCLUDED.expires_at
`, pending.UserID, pending.ChatID, pending.RunID, pending.OverallGoalUI, planJSON, pending.CreatedAt, pending.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save pending: %w", err)
	}
	return nil
}

// GetPending fetches a pending plan review record if it exists and is not expired.
func (s *PlanReviewPostgresStore) GetPending(ctx context.Context, userID, chatID string) (PlanReviewPending, bool, error) {
	if s == nil || s.pool == nil {
		return PlanReviewPending{}, false, fmt.Errorf("plan review store not initialized")
	}
	if userID == "" || chatID == "" {
		return PlanReviewPending{}, false, nil
	}
	var record PlanReviewPending
	var internalPlanRaw []byte
	row := s.pool.QueryRow(ctx, `
SELECT user_id, chat_id, run_id, overall_goal_ui, internal_plan, created_at, expires_at
FROM `+planReviewPendingTable+`
WHERE user_id = $1 AND chat_id = $2
`, userID, chatID)
	if err := row.Scan(&record.UserID, &record.ChatID, &record.RunID, &record.OverallGoalUI, &internalPlanRaw, &record.CreatedAt, &record.ExpiresAt); err != nil {
		if err == pgx.ErrNoRows {
			return PlanReviewPending{}, false, nil
		}
		return PlanReviewPending{}, false, err
	}
	if !record.ExpiresAt.IsZero() && time.Now().After(record.ExpiresAt) {
		_ = s.ClearPending(ctx, userID, chatID)
		return PlanReviewPending{}, false, nil
	}
	if len(internalPlanRaw) > 0 {
		var plan any
		if err := jsonx.Unmarshal(internalPlanRaw, &plan); err != nil {
			s.logger.Warn("Failed to unmarshal internal_plan for user=%s chat=%s: %v", userID, chatID, err)
		} else {
			record.InternalPlan = plan
		}
	}
	return record, true, nil
}

// ClearPending deletes a pending plan review record.
func (s *PlanReviewPostgresStore) ClearPending(ctx context.Context, userID, chatID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("plan review store not initialized")
	}
	if userID == "" || chatID == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM `+planReviewPendingTable+` WHERE user_id = $1 AND chat_id = $2`, userID, chatID)
	return err
}
