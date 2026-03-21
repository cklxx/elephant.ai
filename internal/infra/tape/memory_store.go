package tape

import (
	"context"
	"fmt"
	"sort"
	"sync"

	coretape "alex/internal/core/tape"
)

// MemoryStore is an in-memory TapeStore implementation for tests.
type MemoryStore struct {
	mu    sync.RWMutex
	tapes map[string][]coretape.TapeEntry
}

// NewMemoryStore returns a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tapes: make(map[string][]coretape.TapeEntry),
	}
}

// Append adds an entry to the named tape.
func (s *MemoryStore) Append(_ context.Context, tapeName string, entry coretape.TapeEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tapes[tapeName] = append(s.tapes[tapeName], entry)
	return nil
}

// Query returns entries from the named tape matching the query.
func (s *MemoryStore) Query(_ context.Context, tapeName string, q coretape.TapeQuery) ([]coretape.TapeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, ok := s.tapes[tapeName]
	if !ok {
		return nil, nil
	}

	return applyQuery(entries, q)
}

// List returns all known tape names sorted alphabetically.
func (s *MemoryStore) List(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.tapes))
	for name := range s.tapes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Delete removes a tape and all its entries.
func (s *MemoryStore) Delete(_ context.Context, tapeName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tapes, tapeName)
	return nil
}

// applyQuery filters entries according to the TapeQuery. Shared by both
// MemoryStore and FileStore.
func applyQuery(entries []coretape.TapeEntry, q coretape.TapeQuery) ([]coretape.TapeEntry, error) {
	start := 0

	// afterAnchor: find the anchor entry by ID, return entries after it.
	if anchor := q.GetAfterAnchor(); anchor != "" {
		found := false
		for i, e := range entries {
			if e.ID == anchor {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("anchor %q not found", anchor)
		}
	}

	// afterLabel: reverse-scan for the last anchor/compression entry whose
	// payload "label" matches, then return entries after it.
	if label := q.GetAfterLabel(); label != "" {
		for i := len(entries) - 1; i >= start; i-- {
			e := entries[i]
			if e.Kind != coretape.KindAnchor && e.Kind != coretape.KindCompression {
				continue
			}
			if l, _ := e.Payload["label"].(string); l == label {
				start = i + 1
				break
			}
		}
	}

	var result []coretape.TapeEntry
	kinds := q.GetKinds()
	sessionID := q.GetSessionID()
	runID := q.GetRunID()
	fromDate := q.GetFromDate()
	toDate := q.GetToDate()
	beforeSeq := q.GetBeforeSeq()
	afterSeq := q.GetAfterSeq()

	for i := start; i < len(entries); i++ {
		e := entries[i]

		if len(kinds) > 0 {
			match := false
			for _, k := range kinds {
				if e.Kind == k {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		if sessionID != "" && e.Meta.SessionID != sessionID {
			continue
		}
		if runID != "" && e.Meta.RunID != runID {
			continue
		}
		if !fromDate.IsZero() && e.Date.Before(fromDate) {
			continue
		}
		if !toDate.IsZero() && e.Date.After(toDate) {
			continue
		}
		if beforeSeq != 0 && e.Meta.Seq >= beforeSeq {
			continue
		}
		if afterSeq != 0 && e.Meta.Seq <= afterSeq {
			continue
		}

		result = append(result, e)

		if limit := q.GetLimit(); limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}
