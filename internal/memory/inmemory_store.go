package memory

import (
	"context"
	"strings"
	"sync"
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
	if len(termSet) == 0 {
		return nil, nil
	}

	var results []Entry
	for _, entry := range candidates {
		if matchesTerms(entry.Terms, termSet) {
			results = append(results, entry)
		}
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	return results, nil
}

func matchesTerms(entryTerms []string, query map[string]bool) bool {
	for _, term := range entryTerms {
		if query[strings.ToLower(term)] {
			return true
		}
	}
	return false
}
