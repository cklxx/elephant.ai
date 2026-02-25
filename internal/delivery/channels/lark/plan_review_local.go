package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

const defaultPlanReviewTTL = 60 * time.Minute

type planReviewStoreDoc struct {
	Items []PlanReviewPending `json:"items"`
}

// PlanReviewLocalStore is a local (memory/file) PlanReviewStore.
// When filePath is empty the store is in-memory only.
type PlanReviewLocalStore struct {
	coll *filestore.Collection[string, PlanReviewPending]
	ttl  time.Duration
}

// NewPlanReviewMemoryStore creates an in-memory plan review store.
func NewPlanReviewMemoryStore(ttl time.Duration) *PlanReviewLocalStore {
	return newPlanReviewLocalStore("", ttl)
}

// NewPlanReviewFileStore creates a file-backed plan review store under dir/plan_review_pending.json.
func NewPlanReviewFileStore(dir string, ttl time.Duration) (*PlanReviewLocalStore, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("plan review file store dir is required")
	}
	if err := filestore.EnsureDir(trimmedDir); err != nil {
		return nil, fmt.Errorf("create plan review file store dir: %w", err)
	}
	store := newPlanReviewLocalStore(trimmedDir+"/plan_review_pending.json", ttl)
	if err := store.coll.Load(); err != nil {
		return nil, err
	}
	return store, nil
}

func newPlanReviewLocalStore(filePath string, ttl time.Duration) *PlanReviewLocalStore {
	if ttl <= 0 {
		ttl = defaultPlanReviewTTL
	}
	coll := filestore.NewCollection[string, PlanReviewPending](filestore.CollectionConfig{
		FilePath: filePath,
		Perm:     0o600,
		Name:     "plan_review",
	})
	coll.SetMarshalDoc(func(m map[string]PlanReviewPending) ([]byte, error) {
		doc := planReviewStoreDoc{Items: make([]PlanReviewPending, 0, len(m))}
		for _, p := range m {
			doc.Items = append(doc.Items, p)
		}
		return filestore.MarshalJSONIndent(doc)
	})
	coll.SetUnmarshalDoc(func(data []byte) (map[string]PlanReviewPending, error) {
		var doc planReviewStoreDoc
		if err := jsonx.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("decode plan review store: %w", err)
		}
		now := time.Now()
		m := make(map[string]PlanReviewPending, len(doc.Items))
		for _, p := range doc.Items {
			key := planReviewKey(p.UserID, p.ChatID)
			if key == "::" {
				continue
			}
			// Evict expired on load.
			if !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt) {
				continue
			}
			m[key] = p
		}
		return m, nil
	})
	return &PlanReviewLocalStore{coll: coll, ttl: ttl}
}

func planReviewKey(userID, chatID string) string {
	return strings.TrimSpace(userID) + "::" + strings.TrimSpace(chatID)
}

// EnsureSchema validates file store readiness. Memory mode is no-op.
func (s *PlanReviewLocalStore) EnsureSchema(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("plan review store not initialized")
	}
	return s.coll.EnsureDir()
}

// SavePending writes pending plan review state.
func (s *PlanReviewLocalStore) SavePending(ctx context.Context, pending PlanReviewPending) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("plan review store not initialized")
	}
	pending.UserID = strings.TrimSpace(pending.UserID)
	pending.ChatID = strings.TrimSpace(pending.ChatID)
	if pending.UserID == "" || pending.ChatID == "" {
		return fmt.Errorf("user_id and chat_id required")
	}
	now := s.coll.Now()
	if pending.CreatedAt.IsZero() {
		pending.CreatedAt = now
	}
	if pending.ExpiresAt.IsZero() {
		pending.ExpiresAt = now.Add(s.ttl)
	}

	key := planReviewKey(pending.UserID, pending.ChatID)
	return s.coll.Mutate(func(items map[string]PlanReviewPending) error {
		items[key] = pending
		filestore.EvictByTTL(items, now, 0, func(v PlanReviewPending) time.Time {
			if v.ExpiresAt.IsZero() {
				return now // never evict items without expiry
			}
			return v.ExpiresAt
		})
		return nil
	})
}

// GetPending fetches pending plan review state if present and not expired.
func (s *PlanReviewLocalStore) GetPending(ctx context.Context, userID, chatID string) (PlanReviewPending, bool, error) {
	if ctx != nil && ctx.Err() != nil {
		return PlanReviewPending{}, false, ctx.Err()
	}
	if s == nil {
		return PlanReviewPending{}, false, fmt.Errorf("plan review store not initialized")
	}
	userID = strings.TrimSpace(userID)
	chatID = strings.TrimSpace(chatID)
	if userID == "" || chatID == "" {
		return PlanReviewPending{}, false, nil
	}

	now := s.coll.Now()
	key := planReviewKey(userID, chatID)

	pending, ok := s.coll.Get(key)
	if !ok {
		return PlanReviewPending{}, false, nil
	}
	if !pending.ExpiresAt.IsZero() && now.After(pending.ExpiresAt) {
		_ = s.coll.Delete(key)
		return PlanReviewPending{}, false, nil
	}
	return pending, true, nil
}

// ClearPending deletes a pending plan review entry.
func (s *PlanReviewLocalStore) ClearPending(ctx context.Context, userID, chatID string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("plan review store not initialized")
	}
	userID = strings.TrimSpace(userID)
	chatID = strings.TrimSpace(chatID)
	if userID == "" || chatID == "" {
		return nil
	}
	return s.coll.Delete(planReviewKey(userID, chatID))
}

// SetNow overrides the time function for testing.
func (s *PlanReviewLocalStore) SetNow(fn func() time.Time) {
	s.coll.Now = fn
}

var _ PlanReviewStore = (*PlanReviewLocalStore)(nil)
