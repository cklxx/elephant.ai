package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"alex/internal/infra/filestore"
)

// FileJobStore persists jobs as individual JSON files inside a directory.
// Each job maps to {Dir}/{jobID}.json. All operations are thread-safe.
type FileJobStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileJobStore returns a store that writes jobs under dir. The directory
// is created on the first Save if it does not already exist.
func NewFileJobStore(dir string) *FileJobStore {
	return &FileJobStore{dir: dir}
}

// Save marshals job to JSON and writes it to {Dir}/{job.ID}.json.
// CreatedAt is set automatically on first save; UpdatedAt is always refreshed.
func (s *FileJobStore) Save(_ context.Context, job Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	// Preserve CreatedAt from existing file if this is an overwrite.
	existing, err := s.loadLocked(job.ID)
	if err == nil && existing != nil {
		if job.CreatedAt.IsZero() {
			job.CreatedAt = existing.CreatedAt
		}
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("jobstore: marshal failed: %w", err)
	}

	path := s.path(job.ID)
	if err := filestore.AtomicWrite(path, data, 0o644); err != nil {
		return fmt.Errorf("jobstore: write failed: %w", err)
	}
	return nil
}

// Load retrieves the job with the given ID. Returns an error wrapping
// ErrJobNotFound if the file does not exist.
func (s *FileJobStore) Load(_ context.Context, jobID string) (*Job, error) {
	if jobID == "" {
		return nil, fmt.Errorf("jobstore: job id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadLocked(jobID)
}

// loadLocked reads a single job file. Caller must hold at least s.mu.RLock().
func (s *FileJobStore) loadLocked(jobID string) (*Job, error) {
	data, err := os.ReadFile(s.path(jobID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("jobstore: %w: %s", ErrJobNotFound, jobID)
		}
		return nil, fmt.Errorf("jobstore: read failed: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("jobstore: unmarshal failed: %w", err)
	}
	return &job, nil
}

// List returns all persisted jobs, sorted by CreatedAt ascending.
func (s *FileJobStore) List(_ context.Context) ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // directory doesn't exist yet — no jobs
		}
		return nil, fmt.Errorf("jobstore: readdir failed: %w", err)
	}

	var jobs []Job
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		jobID := entry.Name()[:len(entry.Name())-len(".json")]
		job, err := s.loadLocked(jobID)
		if err != nil {
			continue // skip corrupt files
		}
		jobs = append(jobs, *job)
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})

	return jobs, nil
}

// Delete removes the job file for jobID. Returns an error wrapping
// ErrJobNotFound if the file does not exist.
func (s *FileJobStore) Delete(_ context.Context, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("jobstore: job id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path(jobID))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("jobstore: %w: %s", ErrJobNotFound, jobID)
		}
		return fmt.Errorf("jobstore: delete failed: %w", err)
	}
	return nil
}

// UpdateStatus transitions the job to the given status, refreshing UpdatedAt.
// Returns an error wrapping ErrJobNotFound if the job does not exist.
func (s *FileJobStore) UpdateStatus(_ context.Context, jobID string, status JobStatus) error {
	if jobID == "" {
		return fmt.Errorf("jobstore: job id is required")
	}
	if !status.IsValid() {
		return fmt.Errorf("jobstore: invalid status %q", status)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	job, err := s.loadLocked(jobID)
	if err != nil {
		return err
	}

	job.Status = status
	job.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("jobstore: marshal failed: %w", err)
	}

	if err := filestore.AtomicWrite(s.path(jobID), data, 0o644); err != nil {
		return fmt.Errorf("jobstore: write failed: %w", err)
	}
	return nil
}

// path returns the filesystem path for the given job ID.
func (s *FileJobStore) path(jobID string) string {
	return filepath.Join(s.dir, jobID+".json")
}
