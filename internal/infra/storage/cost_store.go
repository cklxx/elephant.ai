package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	agentstorage "alex/internal/domain/agent/ports/storage"
)

// fileCostStore implements CostStore using file-based storage
type fileCostStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileCostStore creates a new file-based cost store.
func NewFileCostStore(baseDir string) (*fileCostStore, error) {
	// Expand home directory
	if strings.HasPrefix(baseDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		baseDir = filepath.Join(home, baseDir[2:])
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}

	return &fileCostStore{
		baseDir: baseDir,
	}, nil
}

// SaveUsage saves a usage record
func (s *fileCostStore) SaveUsage(ctx context.Context, record agentstorage.UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Organize by date: YYYY-MM-DD/records.jsonl
	dateStr := record.Timestamp.Format("2006-01-02")
	dateDir := filepath.Join(s.baseDir, dateStr)

	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("create date dir: %w", err)
	}

	// Append to daily records file (JSONL format)
	recordsFile := filepath.Join(dateDir, "records.jsonl")
	f, err := os.OpenFile(recordsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open records file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Write record as JSON line
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write record: %w", err)
	}

	// Also create session index
	if err := s.updateSessionIndex(record); err != nil {
		return fmt.Errorf("update session index: %w", err)
	}

	return nil
}

// GetBySession retrieves all usage records for a session
func (s *fileCostStore) GetBySession(ctx context.Context, sessionID string) ([]agentstorage.UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Read session index to find relevant dates
	indexFile := filepath.Join(s.baseDir, "_index", fmt.Sprintf("%s.json", sessionID))
	indexData, err := os.ReadFile(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []agentstorage.UsageRecord{}, nil
		}
		return nil, fmt.Errorf("read session index: %w", err)
	}

	var dates []string
	if err := json.Unmarshal(indexData, &dates); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}

	// Collect records from all dates
	var records []agentstorage.UsageRecord
	for _, date := range dates {
		dateRecords, err := s.readDateRecords(date)
		if err != nil {
			continue // Skip if date file is missing
		}

		// Filter by session ID
		for _, record := range dateRecords {
			if record.SessionID == sessionID {
				records = append(records, record)
			}
		}
	}

	return records, nil
}

// GetByDateRange retrieves records within a date range
func (s *fileCostStore) GetByDateRange(ctx context.Context, start, end time.Time) ([]agentstorage.UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var records []agentstorage.UsageRecord

	// Iterate through dates
	current := start
	for current.Before(end) || current.Equal(end) {
		dateStr := current.Format("2006-01-02")
		dateRecords, err := s.readDateRecords(dateStr)
		if err != nil {
			// Date directory doesn't exist, skip
			current = current.Add(24 * time.Hour)
			continue
		}

		// Filter by exact time range
		for _, record := range dateRecords {
			if (record.Timestamp.After(start) || record.Timestamp.Equal(start)) &&
				(record.Timestamp.Before(end) || record.Timestamp.Equal(end)) {
				records = append(records, record)
			}
		}

		current = current.Add(24 * time.Hour)
	}

	return records, nil
}

// GetByModel retrieves records for a specific model
func (s *fileCostStore) GetByModel(ctx context.Context, model string) ([]agentstorage.UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Read all records and filter by model
	allRecords, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []agentstorage.UsageRecord
	for _, record := range allRecords {
		if record.Model == model {
			filtered = append(filtered, record)
		}
	}

	return filtered, nil
}

// ListAll retrieves all usage records
func (s *fileCostStore) ListAll(ctx context.Context) ([]agentstorage.UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var records []agentstorage.UsageRecord

	// Read all date directories
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read base dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "_index" {
			continue
		}

		dateRecords, err := s.readDateRecords(entry.Name())
		if err != nil {
			continue
		}

		records = append(records, dateRecords...)
	}

	// Sort by timestamp
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records, nil
}

// readDateRecords reads all records for a specific date
func (s *fileCostStore) readDateRecords(dateStr string) ([]agentstorage.UsageRecord, error) {
	recordsFile := filepath.Join(s.baseDir, dateStr, "records.jsonl")
	data, err := os.ReadFile(recordsFile)
	if err != nil {
		return nil, err
	}

	var records []agentstorage.UsageRecord
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var record agentstorage.UsageRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue // Skip malformed records
		}

		records = append(records, record)
	}

	return records, nil
}

// updateSessionIndex updates the session-to-dates index
func (s *fileCostStore) updateSessionIndex(record agentstorage.UsageRecord) error {
	indexDir := filepath.Join(s.baseDir, "_index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return err
	}

	indexFile := filepath.Join(indexDir, fmt.Sprintf("%s.json", record.SessionID))
	dateStr := record.Timestamp.Format("2006-01-02")

	// Read existing dates
	var dates []string
	if data, err := os.ReadFile(indexFile); err == nil {
		_ = json.Unmarshal(data, &dates) // Ignore unmarshal errors - will create new index
	}

	// Add new date if not present
	found := false
	for _, d := range dates {
		if d == dateStr {
			found = true
			break
		}
	}

	if !found {
		dates = append(dates, dateStr)
		sort.Strings(dates)

		data, err := json.Marshal(dates)
		if err != nil {
			return err
		}

		if err := os.WriteFile(indexFile, data, 0644); err != nil {
			return err
		}
	}

	return nil
}
