package memory

import (
	"context"
	"strings"
	"sync"
	"time"
)

// InMemoryStore implements Store for tests and local demos.
type InMemoryStore struct {
	mu      sync.RWMutex
	records map[string][]Entry // userID -> entries
}

// NewInMemoryStore constructs an in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		records: make(map[string][]Entry),
	}
}

// EnsureSchema is a no-op for in-memory storage.
func (s *InMemoryStore) EnsureSchema(_ context.Context) error {
	return nil
}

// Insert appends a memory entry for the user.
func (s *InMemoryStore) Insert(_ context.Context, entry Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[entry.UserID] = append([]Entry{entry}, s.records[entry.UserID]...)
	return entry, nil
}

// Search returns entries that overlap with the query terms.
func (s *InMemoryStore) Search(_ context.Context, query Query) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := s.records[query.UserID]
	if len(candidates) == 0 {
		return nil, nil
	}

	termSet := make(map[string]bool, len(query.Terms))
	for _, term := range query.Terms {
		termSet[strings.ToLower(term)] = true
	}
	hasTerms := len(termSet) > 0 || len(query.Keywords) > 0

	var results []Entry
	for _, entry := range candidates {
		if !matchSlots(entry.Slots, query.Slots) {
			continue
		}
		if hasTerms {
			if !matchesTerms(entry.Terms, termSet) && !containsAny(entry.Content, query.Keywords) {
				continue
			}
		}
		results = append(results, entry)
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	return results, nil
}

// Delete removes entries by key across all users.
func (s *InMemoryStore) Delete(_ context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keySet[key] = struct{}{}
	}
	if len(keySet) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for userID, entries := range s.records {
		filtered := entries[:0]
		for _, entry := range entries {
			if _, ok := keySet[entry.Key]; ok {
				continue
			}
			filtered = append(filtered, entry)
		}
		if len(filtered) == 0 {
			delete(s.records, userID)
		} else {
			s.records[userID] = filtered
		}
	}
	return nil
}

// Prune removes expired entries based on the retention policy.
func (s *InMemoryStore) Prune(_ context.Context, policy RetentionPolicy) ([]string, error) {
	if !policy.HasRules() {
		return nil, nil
	}
	now := time.Now()
	var deleted []string

	s.mu.Lock()
	defer s.mu.Unlock()

	for userID, entries := range s.records {
		filtered := entries[:0]
		for _, entry := range entries {
			if policy.IsExpired(entry, now) {
				if entry.Key != "" {
					deleted = append(deleted, entry.Key)
				}
				continue
			}
			filtered = append(filtered, entry)
		}
		if len(filtered) == 0 {
			delete(s.records, userID)
		} else {
			s.records[userID] = filtered
		}
	}

	return deleted, nil
}

func matchesTerms(entryTerms []string, query map[string]bool) bool {
	for _, term := range entryTerms {
		if query[strings.ToLower(term)] {
			return true
		}
	}
	return false
}

func matchSlots(entrySlots map[string]string, querySlots map[string]string) bool {
	if len(querySlots) == 0 {
		return true
	}
	for key, value := range querySlots {
		if strings.TrimSpace(entrySlots[key]) != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}

func containsAny(content string, keywords []string) bool {
	if content == "" || len(keywords) == 0 {
		return false
	}
	for _, kw := range keywords {
		trimmed := strings.TrimSpace(kw)
		if trimmed == "" {
			continue
		}
		if strings.Contains(content, trimmed) {
			return true
		}
	}
	return false
}
