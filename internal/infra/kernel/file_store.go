package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	domain "alex/internal/domain/kernel"
)

const (
	// defaultLeaseDuration aligns with the typical kernel schedule (30 min).
	// Dispatches still running after this period are considered stale.
	defaultLeaseDuration = 30 * time.Minute

	// defaultRetentionPeriod controls how long terminal dispatches are kept.
	defaultRetentionPeriod = 24 * time.Hour
)

// FileStore implements domain.Store backed by a JSON file.
type FileStore struct {
	mu              sync.RWMutex
	dispatches      map[string]domain.Dispatch
	filePath        string
	leaseDuration   time.Duration
	retentionPeriod time.Duration
	now             func() time.Time
}

// FileStoreConfig holds configurable parameters for the file store.
type FileStoreConfig struct {
	Dir             string
	LeaseDuration   time.Duration
	RetentionPeriod time.Duration
}

// NewFileStore creates a FileStore with the given configuration.
// Zero-value durations fall back to safe defaults.
func NewFileStore(cfg FileStoreConfig) *FileStore {
	lease := cfg.LeaseDuration
	if lease <= 0 {
		lease = defaultLeaseDuration
	}
	retention := cfg.RetentionPeriod
	if retention <= 0 {
		retention = defaultRetentionPeriod
	}
	return &FileStore{
		dispatches:      make(map[string]domain.Dispatch),
		filePath:        filepath.Join(cfg.Dir, "dispatches.json"),
		leaseDuration:   lease,
		retentionPeriod: retention,
		now:             time.Now,
	}
}

// Save persists or updates a dispatch.
func (s *FileStore) Save(ctx context.Context, d domain.Dispatch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	d.UpdatedAt = s.now()
	s.dispatches[d.DispatchID] = d
	return s.persistLocked()
}

// Get returns a dispatch by ID.
func (s *FileStore) Get(_ context.Context, dispatchID string) (domain.Dispatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.dispatches[dispatchID]
	if !ok {
		return domain.Dispatch{}, fmt.Errorf("dispatch %q not found", dispatchID)
	}
	return d, nil
}

// ListRecentByAgent returns the most recent perAgent dispatches per agent
// belonging to kernelID.
func (s *FileStore) ListRecentByAgent(_ context.Context, kernelID string, perAgent int) (map[string][]domain.Dispatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]domain.Dispatch)
	for _, d := range s.dispatches {
		if d.KernelID != kernelID {
			continue
		}
		result[d.AgentName] = append(result[d.AgentName], d)
	}

	// Sort newest-first and truncate per agent.
	for agent, dispatches := range result {
		sortDispatchesDesc(dispatches)
		if perAgent > 0 && len(dispatches) > perAgent {
			result[agent] = dispatches[:perAgent]
		}
	}
	return result, nil
}

// RecoverStaleRunning marks dispatches in "running" state older than the lease
// duration as failed.
func (s *FileStore) RecoverStaleRunning(_ context.Context, kernelID string) (int, error) {
	cutoff := s.now().Add(-s.leaseDuration)

	s.mu.Lock()
	defer s.mu.Unlock()

	var recovered int
	for id, d := range s.dispatches {
		if d.KernelID != kernelID {
			continue
		}
		if d.Status != domain.DispatchRunning {
			continue
		}
		if !d.UpdatedAt.Before(cutoff) {
			continue
		}
		d.Status = domain.DispatchFailed
		d.Error = fmt.Sprintf("recovered stale: no update for %s (lease %s)", s.now().Sub(d.UpdatedAt).Truncate(time.Second), s.leaseDuration)
		d.UpdatedAt = s.now()
		s.dispatches[id] = d
		recovered++
	}
	if recovered > 0 {
		if err := s.persistLocked(); err != nil {
			return 0, fmt.Errorf("persist after recovery: %w", err)
		}
	}
	return recovered, nil
}

// PurgeTerminalDispatches removes terminal dispatches older than the retention
// period.
func (s *FileStore) PurgeTerminalDispatches(_ context.Context, kernelID string) (int, error) {
	cutoff := s.now().Add(-s.retentionPeriod)

	s.mu.Lock()
	defer s.mu.Unlock()

	var purged int
	for id, d := range s.dispatches {
		if d.KernelID != kernelID {
			continue
		}
		if !d.Status.IsTerminal() {
			continue
		}
		if d.UpdatedAt.Before(cutoff) {
			delete(s.dispatches, id)
			purged++
		}
	}
	if purged > 0 {
		if err := s.persistLocked(); err != nil {
			return 0, fmt.Errorf("persist after purge: %w", err)
		}
	}
	return purged, nil
}

// Load reads dispatches from the JSON file. Safe to call before first use.
func (s *FileStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dispatch file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var records []domain.Dispatch
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("parse dispatch file: %w", err)
	}
	for _, d := range records {
		s.dispatches[d.DispatchID] = d
	}
	return nil
}

func (s *FileStore) persistLocked() error {
	records := make([]domain.Dispatch, 0, len(s.dispatches))
	for _, d := range s.dispatches {
		records = append(records, d)
	}
	sortDispatchesDesc(records)

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dispatches: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dispatch dir: %w", err)
	}
	return os.WriteFile(s.filePath, data, 0o644)
}

// sortDispatchesDesc sorts dispatches newest-first by UpdatedAt.
func sortDispatchesDesc(dispatches []domain.Dispatch) {
	for i := 1; i < len(dispatches); i++ {
		for j := i; j > 0 && dispatches[j].UpdatedAt.After(dispatches[j-1].UpdatedAt); j-- {
			dispatches[j], dispatches[j-1] = dispatches[j-1], dispatches[j]
		}
	}
}
