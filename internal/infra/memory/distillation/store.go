package distillation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

// Store is a file-backed JSON store for distillation data.
type Store struct {
	mu       sync.RWMutex
	basePath string
}

// NewStore creates a Store rooted at basePath.
func NewStore(basePath string) *Store {
	return &Store{basePath: basePath}
}

// SaveDailyExtraction persists a day's extraction to disk.
func (s *Store) SaveDailyExtraction(_ context.Context, ext *DailyExtraction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON(s.dailyPath(ext.Date), ext)
}

// LoadDailyExtraction reads a single day's extraction.
func (s *Store) LoadDailyExtraction(_ context.Context, date string) (*DailyExtraction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ext DailyExtraction
	if err := s.readJSON(s.dailyPath(date), &ext); err != nil {
		return nil, err
	}
	return &ext, nil
}

// ListDailyExtractions returns extractions within [from, to], sorted by date.
func (s *Store) ListDailyExtractions(_ context.Context, from, to time.Time) ([]DailyExtraction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := s.listDir(s.dailyDir())
	if err != nil {
		return nil, err
	}
	return s.filterDailyEntries(entries, from, to)
}

func (s *Store) filterDailyEntries(entries []os.DirEntry, from, to time.Time) ([]DailyExtraction, error) {
	var results []DailyExtraction
	for _, e := range entries {
		date := filenameToDate(e.Name())
		t, err := time.Parse("2006-01-02", date)
		if err != nil || t.Before(from) || t.After(to) {
			continue
		}
		var ext DailyExtraction
		if err := s.readJSON(filepath.Join(s.dailyDir(), e.Name()), &ext); err != nil {
			continue
		}
		results = append(results, ext)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Date < results[j].Date })
	return results, nil
}

// SaveWeeklyPatterns persists weekly patterns.
func (s *Store) SaveWeeklyPatterns(_ context.Context, weekStart string, patterns []WeeklyPattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON(s.weeklyPath(weekStart), patterns)
}

// LoadWeeklyPatterns reads weekly patterns for a given week start date.
func (s *Store) LoadWeeklyPatterns(_ context.Context, weekStart string) ([]WeeklyPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var patterns []WeeklyPattern
	if err := s.readJSON(s.weeklyPath(weekStart), &patterns); err != nil {
		return nil, err
	}
	return patterns, nil
}

// SavePersonalityModel persists a user's personality model.
func (s *Store) SavePersonalityModel(_ context.Context, model *PersonalityModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON(s.personalityPath(model.UserID), model)
}

// LoadPersonalityModel reads a user's personality model.
func (s *Store) LoadPersonalityModel(_ context.Context, userID string) (*PersonalityModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var model PersonalityModel
	if err := s.readJSON(s.personalityPath(userID), &model); err != nil {
		return nil, err
	}
	return &model, nil
}

func (s *Store) dailyDir() string                  { return filepath.Join(s.basePath, "daily") }
func (s *Store) dailyPath(date string) string       { return filepath.Join(s.dailyDir(), date+".json") }
func (s *Store) weeklyPath(weekStart string) string  { return filepath.Join(s.basePath, "weekly", weekStart+".json") }
func (s *Store) personalityPath(userID string) string { return filepath.Join(s.basePath, "personality", userID+".json") }

func (s *Store) writeJSON(path string, v any) error {
	data, err := filestore.MarshalJSONIndent(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return filestore.AtomicWrite(path, data, 0o600)
}

func (s *Store) readJSON(path string, v any) error {
	data, err := filestore.ReadFileOrEmpty(path)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("not found: %s", path)
	}
	return jsonx.Unmarshal(data, v)
}

func (s *Store) listDir(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return entries, err
}

func filenameToDate(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}
