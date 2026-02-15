package state_store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"alex/internal/infra/filestore"
	"alex/internal/shared/json"
)

// FileStore persists snapshots as JSON documents on disk for local dev usage.
type FileStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStore creates a file-backed state store rooted at the provided directory.
func NewFileStore(baseDir string) *FileStore {
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "alex-session-snapshots")
	}
	_ = os.MkdirAll(baseDir, 0o755)
	return &FileStore{baseDir: baseDir}
}

func (s *FileStore) sessionDir(sessionID string) string {
	return filepath.Join(s.baseDir, sessionID)
}

// Init ensures the session directory exists.
func (s *FileStore) Init(_ context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	return os.MkdirAll(s.sessionDir(sessionID), 0o755)
}

// ClearSession removes any existing snapshots for the session.
func (s *FileStore) ClearSession(_ context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	if err := os.RemoveAll(s.sessionDir(sessionID)); err != nil {
		return fmt.Errorf("remove session snapshots: %w", err)
	}
	return nil
}

// SaveSnapshot writes the snapshot payload to disk.
func (s *FileStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	if err := s.Init(ctx, snapshot.SessionID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	payload := snapshot
	if payload.CreatedAt.IsZero() {
		payload.CreatedAt = time.Now()
	}
	data, err := jsonx.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	path := filepath.Join(s.sessionDir(snapshot.SessionID), s.filename(snapshot.TurnID))
	if err := filestore.AtomicWrite(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// LatestSnapshot returns the most recent snapshot if available.
func (s *FileStore) LatestSnapshot(ctx context.Context, sessionID string) (Snapshot, error) {
	metas, _, err := s.ListSnapshots(ctx, sessionID, "", 1)
	if err != nil {
		return Snapshot{}, err
	}
	if len(metas) == 0 {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return s.GetSnapshot(ctx, sessionID, metas[0].TurnID)
}

// GetSnapshot returns the snapshot for a given turn.
func (s *FileStore) GetSnapshot(ctx context.Context, sessionID string, turnID int) (Snapshot, error) {
	if sessionID == "" {
		return Snapshot{}, fmt.Errorf("session id required")
	}
	if ctx != nil && ctx.Err() != nil {
		return Snapshot{}, ctx.Err()
	}
	path := filepath.Join(s.sessionDir(sessionID), s.filename(turnID))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := jsonx.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	return snapshot, nil
}

// ListSnapshots returns metadata sorted by newest turn first.
func (s *FileStore) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]SnapshotMetadata, string, error) {
	if sessionID == "" {
		return nil, "", fmt.Errorf("session id required")
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, "", ctx.Err()
	}
	entries, err := os.ReadDir(s.sessionDir(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("list session snapshots: %w", err)
	}
	turnIDs := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if id, ok := s.parseFilename(entry.Name()); ok {
			turnIDs = append(turnIDs, id)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(turnIDs)))

	startIdx := 0
	if cursor != "" {
		if cursorID, err := strconv.Atoi(cursor); err == nil {
			for i, id := range turnIDs {
				if id < cursorID {
					startIdx = i
					break
				}
			}
		}
	}
	if startIdx >= len(turnIDs) {
		return nil, "", nil
	}

	if limit <= 0 {
		limit = 20
	}

	var metas []SnapshotMetadata
	end := startIdx + limit
	if end > len(turnIDs) {
		end = len(turnIDs)
	}
	for _, turnID := range turnIDs[startIdx:end] {
		if ctx != nil && ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		snapshot, err := s.GetSnapshot(ctx, sessionID, turnID)
		if err != nil {
			return nil, "", err
		}
		metas = append(metas, SnapshotMetadata{
			SessionID:  snapshot.SessionID,
			TurnID:     snapshot.TurnID,
			LLMTurnSeq: snapshot.LLMTurnSeq,
			Summary:    snapshot.Summary,
			CreatedAt:  snapshot.CreatedAt,
		})
	}
	var nextCursor string
	if end < len(turnIDs) {
		nextCursor = strconv.Itoa(turnIDs[end])
	}
	return metas, nextCursor, nil
}

func (s *FileStore) filename(turnID int) string {
	if turnID < 0 {
		turnID = 0
	}
	return fmt.Sprintf("turn_%06d.json", turnID)
}

func (s *FileStore) parseFilename(name string) (int, bool) {
	if !strings.HasPrefix(name, "turn_") || !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, "turn_"), ".json")
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false
	}
	return value, true
}
