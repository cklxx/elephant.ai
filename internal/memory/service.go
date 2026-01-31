package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/ksuid"
)

// defaultRecallLimit bounds the number of memories returned for a single query.
const defaultRecallLimit = 5

// Entry captures a single memory record for a user.
type Entry struct {
	Key       string            `json:"key"`
	UserID    string            `json:"user_id"`
	Content   string            `json:"content"`
	Keywords  []string          `json:"keywords"`
	Slots     map[string]string `json:"slots"`
	Terms     []string          `json:"terms,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Query describes a recall request.
type Query struct {
	UserID   string            `json:"user_id"`
	Text     string            `json:"text,omitempty"`
	Keywords []string          `json:"keywords"`
	Slots    map[string]string `json:"slots"`
	Terms    []string          `json:"terms,omitempty"`
	Limit    int               `json:"limit"`
}

// Store abstracts persistence for memories.
type Store interface {
	EnsureSchema(ctx context.Context) error
	Insert(ctx context.Context, entry Entry) (Entry, error)
	Search(ctx context.Context, query Query) ([]Entry, error)
	Delete(ctx context.Context, keys []string) error
	Prune(ctx context.Context, policy RetentionPolicy) ([]string, error)
}

// Service provides higher-level memory operations such as tokenization.
type Service interface {
	Save(ctx context.Context, entry Entry) (Entry, error)
	Recall(ctx context.Context, query Query) ([]Entry, error)
}

type service struct {
	store     Store
	retention RetentionPolicy
}

// NewService constructs a memory service with the provided store.
func NewService(store Store) Service {
	return NewServiceWithRetention(store, RetentionPolicy{})
}

// NewServiceWithRetention constructs a memory service with a retention policy.
func NewServiceWithRetention(store Store, retention RetentionPolicy) Service {
	return &service{store: store, retention: retention}
}

// Save persists a memory entry after normalizing keywords, slots, and tokens.
func (s *service) Save(ctx context.Context, entry Entry) (Entry, error) {
	if s == nil || s.store == nil {
		return entry, fmt.Errorf("memory service not initialized")
	}

	entry.UserID = strings.TrimSpace(entry.UserID)
	if entry.UserID == "" {
		return entry, fmt.Errorf("user_id is required")
	}

	entry.Content = strings.TrimSpace(entry.Content)
	if entry.Content == "" {
		return entry, fmt.Errorf("content is required")
	}

	entry.Keywords = normalizeKeywords(entry.Keywords)
	entry.Slots = normalizeSlots(entry.Slots)
	entry.Terms = collectTerms(entry.Content, entry.Keywords, entry.Slots)
	if entry.Slots == nil {
		entry.Slots = map[string]string{}
	}
	if strings.TrimSpace(entry.Slots["type"]) == "" {
		entry.Slots["type"] = "manual"
	}

	if entry.Key == "" {
		entry.Key = ksuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	return s.store.Insert(ctx, entry)
}

// Recall fetches memories for the user using the supplied keywords/slots.
func (s *service) Recall(ctx context.Context, query Query) ([]Entry, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("memory service not initialized")
	}

	query.UserID = strings.TrimSpace(query.UserID)
	if query.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	query.Text = strings.TrimSpace(query.Text)
	query.Keywords = normalizeKeywords(query.Keywords)
	query.Slots = normalizeSlots(query.Slots)
	query.Terms = collectTerms(query.Text, query.Keywords, query.Slots)
	if query.Limit <= 0 {
		query.Limit = defaultRecallLimit
	}

	results, err := s.store.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.filterExpiredEntries(ctx, results), nil
}

func (s *service) filterExpiredEntries(ctx context.Context, entries []Entry) []Entry {
	if s == nil || s.store == nil || len(entries) == 0 {
		return entries
	}
	if !s.retention.HasRules() {
		return entries
	}

	now := time.Now()
	var filtered []Entry
	var expiredKeys []string
	for _, entry := range entries {
		if s.retention.IsExpired(entry, now) {
			if entry.Key != "" {
				expiredKeys = append(expiredKeys, entry.Key)
			}
			continue
		}
		filtered = append(filtered, entry)
	}

	if s.retention.PruneOnRecall && len(expiredKeys) > 0 {
		_ = s.store.Delete(ctx, expiredKeys)
	}

	return filtered
}

func normalizeKeywords(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeSlots(slots map[string]string) map[string]string {
	if len(slots) == 0 {
		return nil
	}

	normalized := make(map[string]string, len(slots))
	for key, value := range slots {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func collectTerms(content string, keywords []string, slots map[string]string) []string {
	var combined []string
	combined = append(combined, keywords...)
	if content != "" {
		combined = append(combined, tokenize(content)...)
	}
	for key, value := range slots {
		combined = append(combined, tokenize(key)...)
		combined = append(combined, tokenize(value)...)
	}

	seen := make(map[string]bool, len(combined))
	terms := make([]string, 0, len(combined))
	for _, term := range combined {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		terms = append(terms, normalized)
	}
	return terms
}
