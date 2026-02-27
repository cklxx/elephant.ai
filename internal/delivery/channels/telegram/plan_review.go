package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"

	"github.com/mymmrac/telego"
)

// PlanReviewPending records a pending plan review keyed by chatID.
type PlanReviewPending struct {
	ChatID        int64     `json:"chat_id"`
	UserID        int64     `json:"user_id"`
	RunID         string    `json:"run_id"`
	OverallGoalUI string    `json:"overall_goal_ui"`
	InternalPlan  any       `json:"internal_plan"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// PlanReviewStore persists pending plan review state.
type PlanReviewStore interface {
	EnsureSchema(ctx context.Context) error
	SavePending(ctx context.Context, pending PlanReviewPending) error
	GetPending(ctx context.Context, chatID int64) (PlanReviewPending, bool, error)
	ClearPending(ctx context.Context, chatID int64) error
}

// PlanReviewLocalStore is a local (memory/file) PlanReviewStore.
type PlanReviewLocalStore struct {
	coll *filestore.Collection[int64, PlanReviewPending]
	ttl  time.Duration
}

const defaultPlanReviewTTL = 60 * time.Minute

// NewPlanReviewMemoryStore creates an in-memory plan review store.
func NewPlanReviewMemoryStore(ttl time.Duration) *PlanReviewLocalStore {
	return newPlanReviewLocalStore("", ttl)
}

// NewPlanReviewFileStore creates a file-backed plan review store.
func NewPlanReviewFileStore(dir string, ttl time.Duration) (*PlanReviewLocalStore, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("plan review file store dir is required")
	}
	if err := filestore.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("create plan review dir: %w", err)
	}
	store := newPlanReviewLocalStore(dir+"/tg_plan_review.json", ttl)
	if err := store.coll.Load(); err != nil {
		return nil, err
	}
	return store, nil
}

func newPlanReviewLocalStore(filePath string, ttl time.Duration) *PlanReviewLocalStore {
	if ttl <= 0 {
		ttl = defaultPlanReviewTTL
	}
	coll := filestore.NewCollection[int64, PlanReviewPending](filestore.CollectionConfig{
		FilePath: filePath,
		Perm:     0o600,
		Name:     "tg_plan_review",
	})
	type doc struct {
		Items []PlanReviewPending `json:"items"`
	}
	coll.SetMarshalDoc(func(m map[int64]PlanReviewPending) ([]byte, error) {
		d := doc{Items: make([]PlanReviewPending, 0, len(m))}
		for _, p := range m {
			d.Items = append(d.Items, p)
		}
		return filestore.MarshalJSONIndent(d)
	})
	coll.SetUnmarshalDoc(func(data []byte) (map[int64]PlanReviewPending, error) {
		var d doc
		if err := jsonx.Unmarshal(data, &d); err != nil {
			return nil, fmt.Errorf("decode plan review store: %w", err)
		}
		now := time.Now()
		m := make(map[int64]PlanReviewPending, len(d.Items))
		for _, p := range d.Items {
			if p.ChatID == 0 {
				continue
			}
			if !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt) {
				continue
			}
			m[p.ChatID] = p
		}
		return m, nil
	})
	return &PlanReviewLocalStore{coll: coll, ttl: ttl}
}

func (s *PlanReviewLocalStore) EnsureSchema(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.coll.EnsureDir()
}

func (s *PlanReviewLocalStore) SavePending(ctx context.Context, pending PlanReviewPending) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	now := s.coll.Now()
	if pending.CreatedAt.IsZero() {
		pending.CreatedAt = now
	}
	if pending.ExpiresAt.IsZero() {
		pending.ExpiresAt = now.Add(s.ttl)
	}
	return s.coll.Mutate(func(items map[int64]PlanReviewPending) error {
		items[pending.ChatID] = pending
		return nil
	})
}

func (s *PlanReviewLocalStore) GetPending(_ context.Context, chatID int64) (PlanReviewPending, bool, error) {
	pending, ok := s.coll.Get(chatID)
	if !ok {
		return PlanReviewPending{}, false, nil
	}
	now := s.coll.Now()
	if !pending.ExpiresAt.IsZero() && now.After(pending.ExpiresAt) {
		_ = s.coll.Delete(chatID)
		return PlanReviewPending{}, false, nil
	}
	return pending, true, nil
}

func (s *PlanReviewLocalStore) ClearPending(_ context.Context, chatID int64) error {
	return s.coll.Delete(chatID)
}

var _ PlanReviewStore = (*PlanReviewLocalStore)(nil)

// handlePlanReviewCallback processes inline keyboard button presses for plan review.
func (g *Gateway) handlePlanReviewCallback(ctx context.Context, cq *telego.CallbackQuery) {
	if g.planReview == nil || cq == nil {
		return
	}

	data := strings.TrimSpace(cq.Data)
	if !strings.HasPrefix(data, "plan:") {
		return
	}

	chatID := cq.Message.GetChat().ID
	action := strings.TrimPrefix(data, "plan:")

	pending, ok, err := g.planReview.GetPending(ctx, chatID)
	if err != nil || !ok {
		g.answerCallback(ctx, cq.ID, "没有待审核的计划。")
		return
	}

	switch action {
	case "approve":
		_ = g.planReview.ClearPending(ctx, chatID)
		g.answerCallback(ctx, cq.ID, "计划已批准 ✓")
		g.sendReply(ctx, chatID, 0, fmt.Sprintf("计划已批准: %s", pending.OverallGoalUI))
	case "reject":
		_ = g.planReview.ClearPending(ctx, chatID)
		g.answerCallback(ctx, cq.ID, "计划已拒绝 ✗")
		g.sendReply(ctx, chatID, 0, "计划已拒绝。")
	default:
		g.answerCallback(ctx, cq.ID, "未知操作")
	}
}

// answerCallback answers a callback query with optional text.
func (g *Gateway) answerCallback(ctx context.Context, callbackID, text string) {
	if g.bot == nil {
		return
	}
	params := &telego.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	}
	_ = g.bot.AnswerCallbackQuery(ctx, params)
}
