package tape

import (
	"context"
	"fmt"
	"sync"

	coretape "alex/internal/core/tape"
)

// ForkStore provides context-based fork isolation on top of a parent TapeStore.
// Writes go to an in-memory overlay while reads merge parent and fork entries.
type ForkStore struct {
	parent coretape.TapeStore

	mu    sync.RWMutex
	forks map[string][]coretape.TapeEntry // tapeName -> forked entries
}

// NewForkStore wraps parent with fork isolation.
func NewForkStore(parent coretape.TapeStore) *ForkStore {
	return &ForkStore{
		parent: parent,
		forks:  make(map[string][]coretape.TapeEntry),
	}
}

// Fork registers a tape name for fork isolation. Future appends to this tape
// go to the overlay instead of the parent.
func (s *ForkStore) Fork(tapeName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.forks[tapeName]; !ok {
		s.forks[tapeName] = nil
	}
}

// Append writes to the fork overlay if the tape has been forked, otherwise
// delegates to the parent.
func (s *ForkStore) Append(ctx context.Context, tapeName string, entry coretape.TapeEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, forked := s.forks[tapeName]; forked {
		s.forks[tapeName] = append(s.forks[tapeName], entry)
		return nil
	}
	return s.parent.Append(ctx, tapeName, entry)
}

// Query merges parent entries with fork entries and applies the query filters.
func (s *ForkStore) Query(ctx context.Context, tapeName string, q coretape.TapeQuery) ([]coretape.TapeEntry, error) {
	s.mu.RLock()
	forkEntries, forked := s.forks[tapeName]
	s.mu.RUnlock()

	if !forked {
		return s.parent.Query(ctx, tapeName, q)
	}

	// Read all parent entries (no filter) then merge.
	parentEntries, err := s.parent.Query(ctx, tapeName, coretape.Query())
	if err != nil {
		return nil, fmt.Errorf("fork query parent: %w", err)
	}

	merged := make([]coretape.TapeEntry, 0, len(parentEntries)+len(forkEntries))
	merged = append(merged, parentEntries...)
	merged = append(merged, forkEntries...)

	return applyQuery(merged, q)
}

// List returns the union of parent tape names and forked tape names.
func (s *ForkStore) List(ctx context.Context) ([]string, error) {
	parentNames, err := s.parent.List(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{}, len(parentNames))
	for _, n := range parentNames {
		seen[n] = struct{}{}
	}
	for n := range s.forks {
		if _, ok := seen[n]; !ok {
			parentNames = append(parentNames, n)
		}
	}
	return parentNames, nil
}

// Delete delegates to the parent and removes any fork state.
func (s *ForkStore) Delete(ctx context.Context, tapeName string) error {
	s.mu.Lock()
	delete(s.forks, tapeName)
	s.mu.Unlock()
	return s.parent.Delete(ctx, tapeName)
}

// Merge flushes all fork entries back to the parent store and clears the fork.
func (s *ForkStore) Merge(ctx context.Context) error {
	s.mu.Lock()
	snapshot := make(map[string][]coretape.TapeEntry, len(s.forks))
	for name, entries := range s.forks {
		snapshot[name] = entries
	}
	s.forks = make(map[string][]coretape.TapeEntry)
	s.mu.Unlock()

	for name, entries := range snapshot {
		for _, e := range entries {
			if err := s.parent.Append(ctx, name, e); err != nil {
				return fmt.Errorf("merge tape %q: %w", name, err)
			}
		}
	}
	return nil
}

// Discard drops all fork entries without merging.
func (s *ForkStore) Discard() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forks = make(map[string][]coretape.TapeEntry)
}
