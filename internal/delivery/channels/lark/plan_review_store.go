package lark

import (
	"context"
	"time"
)

// PlanReviewPending records a pending plan review keyed by user_id + chat_id.
type PlanReviewPending struct {
	UserID        string
	ChatID        string
	RunID         string
	OverallGoalUI string
	InternalPlan  any
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// PlanReviewStore persists pending plan review state for Lark.
type PlanReviewStore interface {
	EnsureSchema(ctx context.Context) error
	SavePending(ctx context.Context, pending PlanReviewPending) error
	GetPending(ctx context.Context, userID, chatID string) (PlanReviewPending, bool, error)
	ClearPending(ctx context.Context, userID, chatID string) error
}
