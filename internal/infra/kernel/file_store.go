package kernel

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"alex/internal/domain/kernel"
	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

// fileStoreDoc is the on-disk JSON envelope.
type fileStoreDoc struct {
	Dispatches []kernel.Dispatch `json:"dispatches"`
}

// FileStore is a file-backed implementation of kernel.Store.
// It keeps an in-memory map guarded by a RWMutex and persists
// to a single JSON file using atomic temp-file + rename writes.
type FileStore struct {
	mu                sync.RWMutex
	dispatches        map[string]kernel.Dispatch
	filePath          string
	leaseDuration     time.Duration
	retentionDuration time.Duration
	now               func() time.Time // injectable for tests
}

// NewFileStore creates a new file-backed dispatch store.
// dir is the directory where dispatches.json will be stored.
// leaseDuration controls how long a claimed dispatch is leased to a worker.
// retentionDuration controls how long terminal dispatches are kept before pruning.
func NewFileStore(dir string, leaseDuration, retentionDuration time.Duration) *FileStore {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Minute
	}
	if retentionDuration <= 0 {
		retentionDuration = 14 * 24 * time.Hour
	}
	return &FileStore{
		dispatches:        make(map[string]kernel.Dispatch),
		filePath:          filepath.Join(dir, "dispatches.json"),
		leaseDuration:     leaseDuration,
		retentionDuration: retentionDuration,
		now:               time.Now,
	}
}

// EnsureSchema creates the storage directory and loads existing data from disk.
func (s *FileStore) EnsureSchema(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := filestore.EnsureParentDir(s.filePath); err != nil {
		return fmt.Errorf("create dispatch store dir: %w", err)
	}
	if err := s.load(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.pruneLocked(ctx, s.now(), true); err != nil {
		return err
	}
	return nil
}

// EnqueueDispatches inserts a batch of dispatch specs as pending dispatches.
func (s *FileStore) EnqueueDispatches(ctx context.Context, kernelID, cycleID string, specs []kernel.DispatchSpec) ([]kernel.Dispatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}

	now := s.now()
	created := make([]kernel.Dispatch, 0, len(specs))
	for _, spec := range specs {
		d := kernel.Dispatch{
			DispatchID: uuid.New().String(),
			KernelID:   kernelID,
			CycleID:    cycleID,
			AgentID:    spec.AgentID,
			Prompt:     spec.Prompt,
			Priority:   spec.Priority,
			Kind:       spec.Kind,
			Team:       cloneTeamSpec(spec.Team),
			Status:     kernel.DispatchPending,
			Metadata:   spec.Metadata,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		created = append(created, d)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range created {
		s.dispatches[d.DispatchID] = d
	}
	if err := s.persistLocked(); err != nil {
		// Roll back on persist failure.
		for _, d := range created {
			delete(s.dispatches, d.DispatchID)
		}
		return nil, err
	}
	return created, nil
}

func cloneTeamSpec(spec *kernel.TeamDispatchSpec) *kernel.TeamDispatchSpec {
	if spec == nil {
		return nil
	}
	cloned := *spec
	if spec.Prompts != nil {
		clonedPrompts := make(map[string]string, len(spec.Prompts))
		for role, prompt := range spec.Prompts {
			clonedPrompts[role] = prompt
		}
		cloned.Prompts = clonedPrompts
	}
	return &cloned
}

// ClaimDispatches atomically claims up to limit pending dispatches for workerID.
// Dispatches are selected by kernelID, sorted by priority DESC then created_at ASC.
// Expired leases (past lease_until) are treated as unclaimed and eligible for re-claim.
func (s *FileStore) ClaimDispatches(ctx context.Context, kernelID, workerID string, limit int) ([]kernel.Dispatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}

	now := s.now()
	leaseUntil := now.Add(s.leaseDuration)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Collect claimable dispatches: pending for this kernel, with no active lease.
	var candidates []kernel.Dispatch
	for _, d := range s.dispatches {
		if d.KernelID != kernelID || d.Status != kernel.DispatchPending {
			continue
		}
		// Skip if actively leased (lease_owner set and lease not expired).
		if d.LeaseOwner != "" && d.LeaseUntil != nil && d.LeaseUntil.After(now) {
			continue
		}
		candidates = append(candidates, d)
	}

	// Sort: priority DESC, then created_at ASC.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Claim each candidate.
	claimed := make([]kernel.Dispatch, 0, len(candidates))
	for _, d := range candidates {
		d.LeaseOwner = workerID
		d.LeaseUntil = &leaseUntil
		d.UpdatedAt = now
		s.dispatches[d.DispatchID] = d
		claimed = append(claimed, d)
	}

	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return claimed, nil
}

// MarkDispatchRunning transitions a dispatch to running status.
func (s *FileStore) MarkDispatchRunning(ctx context.Context, dispatchID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.dispatches[dispatchID]
	if !ok {
		return fmt.Errorf("dispatch %s not found", dispatchID)
	}
	d.Status = kernel.DispatchRunning
	d.UpdatedAt = s.now()
	s.dispatches[dispatchID] = d
	return s.persistLocked()
}

// MarkDispatchDone transitions a dispatch to done with the resulting taskID.
// K-03 fix: pruneLocked is called with persist=true so that the status update
// and any pruned records are written to disk in a single atomic operation.
// This prevents memory/disk divergence if the process crashes between prune
// and a subsequent persist call.
func (s *FileStore) MarkDispatchDone(ctx context.Context, dispatchID, taskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.dispatches[dispatchID]
	if !ok {
		return fmt.Errorf("dispatch %s not found", dispatchID)
	}
	d.Status = kernel.DispatchDone
	d.TaskID = taskID
	d.UpdatedAt = now
	s.dispatches[dispatchID] = d
	// persist=true: write status change and pruned records atomically in one
	// file operation, eliminating any window where in-memory prune is not
	// reflected on disk.
	if _, err := s.pruneLocked(ctx, now, true); err != nil {
		return err
	}
	return nil
}

// MarkDispatchFailed transitions a dispatch to failed with an error message.
// K-03 fix: same persist=true pattern as MarkDispatchDone — see above.
func (s *FileStore) MarkDispatchFailed(ctx context.Context, dispatchID, errMsg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.dispatches[dispatchID]
	if !ok {
		return fmt.Errorf("dispatch %s not found", dispatchID)
	}
	d.Status = kernel.DispatchFailed
	d.Error = errMsg
	d.UpdatedAt = now
	s.dispatches[dispatchID] = d
	// persist=true: write status change and pruned records atomically in one
	// file operation, eliminating any window where in-memory prune is not
	// reflected on disk.
	if _, err := s.pruneLocked(ctx, now, true); err != nil {
		return err
	}
	return nil
}

// ListActiveDispatches returns all non-terminal dispatches for a kernel.
// Terminal statuses are done, failed, and cancelled.
func (s *FileStore) ListActiveDispatches(ctx context.Context, kernelID string) ([]kernel.Dispatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []kernel.Dispatch
	for _, d := range s.dispatches {
		if d.KernelID != kernelID {
			continue
		}
		if isTerminalDispatchStatus(d.Status) {
			continue
		}
		out = append(out, d)
	}
	// Deterministic ordering: created_at ASC, then dispatch_id ASC.
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].DispatchID < out[j].DispatchID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// ListRecentByAgent returns the most recent dispatch for each agent_id
// within the given kernel. Recency is determined by updated_at first,
// then created_at, with dispatch_id as a deterministic final tiebreaker.
func (s *FileStore) ListRecentByAgent(ctx context.Context, kernelID string) (map[string]kernel.Dispatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]kernel.Dispatch)
	for _, d := range s.dispatches {
		if d.KernelID != kernelID {
			continue
		}
		existing, ok := result[d.AgentID]
		if !ok || isDispatchMoreRecent(d, existing) {
			result[d.AgentID] = d
		}
	}
	return result, nil
}

// --- internal helpers ---

func (s *FileStore) load() error {
	data, err := filestore.ReadFileOrEmpty(s.filePath)
	if err != nil {
		return fmt.Errorf("read dispatch store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var doc fileStoreDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode dispatch store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range doc.Dispatches {
		if d.DispatchID == "" {
			continue
		}
		s.dispatches[d.DispatchID] = d
	}
	return nil
}

// persistLocked writes the current state to disk atomically.
// Caller must hold s.mu (at least write lock).
func (s *FileStore) persistLocked() error {
	doc := fileStoreDoc{
		Dispatches: make([]kernel.Dispatch, 0, len(s.dispatches)),
	}
	for _, d := range s.dispatches {
		doc.Dispatches = append(doc.Dispatches, d)
	}
	// Deterministic output: sort by created_at ASC.
	sort.Slice(doc.Dispatches, func(i, j int) bool {
		return doc.Dispatches[i].CreatedAt.Before(doc.Dispatches[j].CreatedAt)
	})

	data, err := jsonx.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode dispatch store: %w", err)
	}
	data = append(data, '\n')

	if err := filestore.AtomicWrite(s.filePath, data, 0o600); err != nil {
		return fmt.Errorf("write dispatch store: %w", err)
	}
	return nil
}

func pruneDeadline(d kernel.Dispatch) time.Time {
	if d.UpdatedAt.After(d.CreatedAt) {
		return d.UpdatedAt
	}
	return d.CreatedAt
}

func (s *FileStore) pruneLocked(ctx context.Context, now time.Time, persist bool) (int, error) {
	if s.retentionDuration <= 0 {
		return 0, nil
	}
	cutoff := now.Add(-s.retentionDuration)
	removed := 0
	for id, d := range s.dispatches {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if !isTerminalDispatchStatus(d.Status) {
			continue
		}
		if pruneDeadline(d).After(cutoff) {
			continue
		}
		delete(s.dispatches, id)
		removed++
	}
	if removed > 0 && persist {
		if err := s.persistLocked(); err != nil {
			return 0, err
		}
	}
	return removed, nil
}

func isDispatchMoreRecent(candidate, existing kernel.Dispatch) bool {
	if candidate.UpdatedAt.After(existing.UpdatedAt) {
		return true
	}
	if candidate.UpdatedAt.Before(existing.UpdatedAt) {
		return false
	}
	if candidate.CreatedAt.After(existing.CreatedAt) {
		return true
	}
	if candidate.CreatedAt.Before(existing.CreatedAt) {
		return false
	}
	return candidate.DispatchID > existing.DispatchID
}

func isTerminalDispatchStatus(s kernel.DispatchStatus) bool {
	switch s {
	case kernel.DispatchDone, kernel.DispatchFailed, kernel.DispatchCancelled:
		return true
	default:
		return false
	}
}

// RecoverStaleRunning marks dispatches stuck in "running" longer than
// leaseDuration as "failed". This prevents permanently blocked agents
// when a previous cycle's executor crashed without completing.
func (s *FileStore) RecoverStaleRunning(ctx context.Context, kernelID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	now := s.now()
	cutoff := now.Add(-s.leaseDuration)

	s.mu.Lock()
	defer s.mu.Unlock()

	var recovered int
	for id, d := range s.dispatches {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if d.KernelID != kernelID {
			continue
		}
		if d.Status != kernel.DispatchRunning {
			continue
		}
		if d.UpdatedAt.After(cutoff) {
			continue
		}
		d.Status = kernel.DispatchFailed
		d.Error = fmt.Sprintf("recovered: stale running since %s (lease=%s)", d.UpdatedAt.Format("2006-01-02T15:04:05Z"), s.leaseDuration)
		d.UpdatedAt = now
		s.dispatches[id] = d
		recovered++
	}
	if recovered > 0 {
		if _, err := s.pruneLocked(ctx, now, false); err != nil {
			return 0, err
		}
		if err := s.persistLocked(); err != nil {
			return 0, fmt.Errorf("persist after stale recovery: %w", err)
		}
	}
	return recovered, nil
}

// Compile-time interface check.
var _ kernel.Store = (*FileStore)(nil)
