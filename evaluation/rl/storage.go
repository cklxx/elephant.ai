package rl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Manifest holds index metadata for stored JSONL files.
type Manifest struct {
	UpdatedAt time.Time                `json:"updated_at"`
	Tiers     map[QualityTier]TierInfo `json:"tiers"`
}

// TierInfo holds per-tier metadata.
type TierInfo struct {
	Files      []FileInfo `json:"files"`
	TotalCount int        `json:"total_count"`
	TotalBytes int64      `json:"total_bytes"`
}

// FileInfo describes a single JSONL file.
type FileInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

// Storage persists RLTrajectory records as JSONL files organized by tier and date.
type Storage struct {
	baseDir string
	mu      sync.Mutex
}

// NewStorage creates a new Storage at the given directory.
func NewStorage(baseDir string) (*Storage, error) {
	for _, tier := range ValidTiers {
		dir := filepath.Join(baseDir, string(tier))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create tier dir %s: %w", tier, err)
		}
	}
	return &Storage{baseDir: baseDir}, nil
}

// Append writes a trajectory to the appropriate tier/date JSONL file.
func (s *Storage) Append(traj *RLTrajectory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dateStr := traj.ExtractedAt.Format("2006-01-02")
	dir := filepath.Join(s.baseDir, string(traj.QualityTier))
	path := filepath.Join(dir, dateStr+".jsonl")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	data, err := json.Marshal(traj)
	if err != nil {
		return fmt.Errorf("marshal trajectory: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write trajectory: %w", err)
	}

	return nil
}

// AppendBatch writes multiple trajectories.
func (s *Storage) AppendBatch(trajectories []*RLTrajectory) error {
	for _, traj := range trajectories {
		if err := s.Append(traj); err != nil {
			return err
		}
	}
	return nil
}

// ReadTier reads all trajectories from a tier, optionally filtered by date range.
func (s *Storage) ReadTier(tier QualityTier, after, before time.Time) ([]*RLTrajectory, error) {
	dir := filepath.Join(s.baseDir, string(tier))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tier dir: %w", err)
	}

	var results []*RLTrajectory
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		dateStr := entry.Name()[:len(entry.Name())-6] // strip .jsonl
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if !after.IsZero() && fileDate.Before(after) {
			continue
		}
		if !before.IsZero() && fileDate.After(before) {
			continue
		}

		trajs, err := s.readFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		results = append(results, trajs...)
	}

	return results, nil
}

// Stats returns aggregate statistics across all tiers.
func (s *Storage) Stats() (*Manifest, error) {
	manifest := &Manifest{
		UpdatedAt: time.Now(),
		Tiers:     make(map[QualityTier]TierInfo),
	}

	for _, tier := range ValidTiers {
		dir := filepath.Join(s.baseDir, string(tier))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				manifest.Tiers[tier] = TierInfo{}
				continue
			}
			return nil, fmt.Errorf("read %s: %w", tier, err)
		}

		info := TierInfo{}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
				continue
			}
			fi, err := entry.Info()
			if err != nil {
				continue
			}
			count, _ := s.countLines(filepath.Join(dir, entry.Name()))
			info.Files = append(info.Files, FileInfo{
				Name:  entry.Name(),
				Count: count,
				Bytes: fi.Size(),
			})
			info.TotalCount += count
			info.TotalBytes += fi.Size()
		}
		sort.Slice(info.Files, func(i, j int) bool {
			return info.Files[i].Name > info.Files[j].Name // newest first
		})
		manifest.Tiers[tier] = info
	}

	return manifest, nil
}

func (s *Storage) readFile(path string) ([]*RLTrajectory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var results []*RLTrajectory
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line
	for scanner.Scan() {
		var traj RLTrajectory
		if err := json.Unmarshal(scanner.Bytes(), &traj); err != nil {
			continue // skip malformed lines
		}
		results = append(results, &traj)
	}
	return results, scanner.Err()
}

func (s *Storage) countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}
