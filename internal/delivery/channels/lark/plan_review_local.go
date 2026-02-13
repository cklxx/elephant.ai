package lark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	jsonx "alex/internal/shared/json"
)

const defaultPlanReviewTTL = 60 * time.Minute

type planReviewStoreDoc struct {
	Items []PlanReviewPending `json:"items"`
}

// PlanReviewLocalStore is a local (memory/file) PlanReviewStore.
// When filePath is empty the store is in-memory only.
type PlanReviewLocalStore struct {
	mu       sync.RWMutex
	filePath string
	ttl      time.Duration
	now      func() time.Time
	items    map[string]PlanReviewPending
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
	if err := os.MkdirAll(trimmedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create plan review file store dir: %w", err)
	}
	store := newPlanReviewLocalStore(filepath.Join(trimmedDir, "plan_review_pending.json"), ttl)
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func newPlanReviewLocalStore(filePath string, ttl time.Duration) *PlanReviewLocalStore {
	if ttl <= 0 {
		ttl = defaultPlanReviewTTL
	}
	return &PlanReviewLocalStore{
		filePath: filePath,
		ttl:      ttl,
		now:      time.Now,
		items:    make(map[string]PlanReviewPending),
	}
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
	if s.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("ensure plan review directory: %w", err)
	}
	return nil
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
	now := s.now()
	if pending.CreatedAt.IsZero() {
		pending.CreatedAt = now
	}
	if pending.ExpiresAt.IsZero() {
		pending.ExpiresAt = now.Add(s.ttl)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[planReviewKey(pending.UserID, pending.ChatID)] = pending
	s.evictExpiredLocked(now)
	return s.persistLocked()
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

	now := s.now()
	key := planReviewKey(userID, chatID)

	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.items[key]
	if !ok {
		return PlanReviewPending{}, false, nil
	}
	if !pending.ExpiresAt.IsZero() && now.After(pending.ExpiresAt) {
		delete(s.items, key)
		_ = s.persistLocked()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, planReviewKey(userID, chatID))
	return s.persistLocked()
}

func (s *PlanReviewLocalStore) evictExpiredLocked(now time.Time) {
	for key, pending := range s.items {
		if pending.ExpiresAt.IsZero() {
			continue
		}
		if now.After(pending.ExpiresAt) {
			delete(s.items, key)
		}
	}
}

func (s *PlanReviewLocalStore) load() error {
	if s.filePath == "" {
		return nil
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read plan review store: %w", err)
	}
	var doc planReviewStoreDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode plan review store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pending := range doc.Items {
		key := planReviewKey(pending.UserID, pending.ChatID)
		if key == "::" {
			continue
		}
		s.items[key] = pending
	}
	s.evictExpiredLocked(s.now())
	return nil
}

func (s *PlanReviewLocalStore) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	doc := planReviewStoreDoc{
		Items: make([]PlanReviewPending, 0, len(s.items)),
	}
	for _, pending := range s.items {
		doc.Items = append(doc.Items, pending)
	}
	data, err := jsonx.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode plan review store: %w", err)
	}
	data = append(data, '\n')
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write plan review temp file: %w", err)
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit plan review file: %w", err)
	}
	return nil
}

var _ PlanReviewStore = (*PlanReviewLocalStore)(nil)
