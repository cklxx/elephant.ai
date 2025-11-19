package state_store

import (
	"context"
	"sort"
	"strconv"
	"sync"
)

// InMemoryStore is a lightweight Store implementation for tests.
type InMemoryStore struct {
	mu        sync.RWMutex
	snapshots map[string]map[int]Snapshot
}

// NewInMemoryStore constructs an in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{snapshots: make(map[string]map[int]Snapshot)}
}

func (s *InMemoryStore) Init(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.snapshots[sessionID]; !ok {
		s.snapshots[sessionID] = make(map[int]Snapshot)
	}
	return nil
}

func (s *InMemoryStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	if err := s.Init(ctx, snapshot.SessionID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.SessionID][snapshot.TurnID] = snapshot
	return nil
}

func (s *InMemoryStore) LatestSnapshot(_ context.Context, sessionID string) (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	turns := s.turnsLocked(sessionID)
	if len(turns) == 0 {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return s.snapshots[sessionID][turns[len(turns)-1]], nil
}

func (s *InMemoryStore) GetSnapshot(_ context.Context, sessionID string, turnID int) (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sessionID == "" {
		return Snapshot{}, ErrSnapshotNotFound
	}
	turns, ok := s.snapshots[sessionID]
	if !ok {
		return Snapshot{}, ErrSnapshotNotFound
	}
	snap, ok := turns[turnID]
	if !ok {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return snap, nil
}

func (s *InMemoryStore) ListSnapshots(_ context.Context, sessionID string, cursor string, limit int) ([]SnapshotMetadata, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	turns := s.turnsLocked(sessionID)
	if len(turns) == 0 {
		return nil, "", nil
	}
	startIdx := 0
	if cursor != "" {
		if cursorID, err := strconv.Atoi(cursor); err == nil {
			for idx, turn := range turns {
				if turn > cursorID {
					startIdx = idx + 1
				}
			}
		}
	}
	if startIdx >= len(turns) {
		return nil, "", nil
	}
	if limit <= 0 {
		limit = 20
	}
	end := startIdx + limit
	if end > len(turns) {
		end = len(turns)
	}
	var metas []SnapshotMetadata
	for _, turnID := range turns[startIdx:end] {
		snap := s.snapshots[sessionID][turnID]
		metas = append(metas, SnapshotMetadata{
			SessionID:  snap.SessionID,
			TurnID:     snap.TurnID,
			LLMTurnSeq: snap.LLMTurnSeq,
			Summary:    snap.Summary,
			CreatedAt:  snap.CreatedAt,
		})
	}
	var nextCursor string
	if end < len(turns) {
		nextCursor = strconv.Itoa(turns[end])
	}
	return metas, nextCursor, nil
}

func (s *InMemoryStore) turnsLocked(sessionID string) []int {
	turns, ok := s.snapshots[sessionID]
	if !ok {
		return nil
	}
	ids := make([]int, 0, len(turns))
	for turn := range turns {
		ids = append(ids, turn)
	}
	sort.Ints(ids)
	return ids
}
