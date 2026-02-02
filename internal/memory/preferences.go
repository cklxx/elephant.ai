package memory

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

	"github.com/segmentio/ksuid"
)

// PreferenceCategory categorizes a user preference.
type PreferenceCategory string

const (
	PreferenceCategoryLanguage       PreferenceCategory = "language"
	PreferenceCategoryFormat         PreferenceCategory = "format"
	PreferenceCategoryTool           PreferenceCategory = "tool"
	PreferenceCategoryStyle          PreferenceCategory = "style"
	PreferenceCategoryTimezone       PreferenceCategory = "timezone"
	PreferenceCategoryResponseLength PreferenceCategory = "response_length"
	PreferenceCategoryCommunication  PreferenceCategory = "communication"
)

// Preference represents a learned user preference extracted from interaction patterns.
type Preference struct {
	ID               string             `json:"id"`
	UserID           string             `json:"user_id"`
	Category         PreferenceCategory `json:"category"`
	Key              string             `json:"key"`
	Value            string             `json:"value"`
	Confidence       float64            `json:"confidence"`
	ObservationCount int                `json:"observation_count"`
	Source           string             `json:"source"`
	LastObserved     time.Time          `json:"last_observed"`
	CreatedAt        time.Time          `json:"created_at"`
}

// PreferenceQuery describes a search request against the preference store.
type PreferenceQuery struct {
	UserID        string               `json:"user_id"`
	Categories    []PreferenceCategory `json:"categories,omitempty"`
	Keys          []string             `json:"keys,omitempty"`
	MinConfidence float64              `json:"min_confidence,omitempty"`
	Limit         int                  `json:"limit,omitempty"`
}

// PreferenceStore abstracts persistence for user preference memory.
type PreferenceStore interface {
	SetPreference(ctx context.Context, pref Preference) (Preference, error)
	GetPreference(ctx context.Context, userID, category, key string) (Preference, error)
	SearchPreferences(ctx context.Context, query PreferenceQuery) ([]Preference, error)
	InferPreferences(ctx context.Context, userID string) ([]Preference, error)
	DeletePreference(ctx context.Context, id string) error
}

// FilePreferenceStore implements PreferenceStore by persisting preferences as
// JSON files, one file per user at {basePath}/{userID}/preferences.json.
type FilePreferenceStore struct {
	basePath string
	mu       sync.Mutex
	cache    map[string][]Preference // userID -> preferences
}

// NewFilePreferenceStore creates a file-backed preference store rooted at basePath.
func NewFilePreferenceStore(basePath string) *FilePreferenceStore {
	return &FilePreferenceStore{
		basePath: basePath,
		cache:    make(map[string][]Preference),
	}
}

// SetPreference creates or updates a preference. If a preference with the same
// UserID+Category+Key already exists, it increments ObservationCount, recalculates
// Confidence, and updates LastObserved and Value.
func (s *FilePreferenceStore) SetPreference(_ context.Context, pref Preference) (Preference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pref.UserID = strings.TrimSpace(pref.UserID)
	if pref.UserID == "" {
		return Preference{}, fmt.Errorf("preference user_id is required")
	}
	if pref.Category == "" {
		return Preference{}, fmt.Errorf("preference category is required")
	}
	if pref.Key == "" {
		return Preference{}, fmt.Errorf("preference key is required")
	}

	prefs, err := s.loadLocked(pref.UserID)
	if err != nil {
		return Preference{}, err
	}

	now := time.Now()
	idx := s.findPreference(prefs, pref.UserID, pref.Category, pref.Key)
	if idx >= 0 {
		// Upsert: increment observation count, recalculate confidence, update value.
		existing := &prefs[idx]
		existing.ObservationCount++
		existing.Confidence = computeConfidence(existing.ObservationCount)
		existing.LastObserved = now
		if pref.Value != "" {
			existing.Value = pref.Value
		}
		if pref.Source != "" {
			existing.Source = pref.Source
		}

		if err := s.saveLocked(pref.UserID, prefs); err != nil {
			return Preference{}, err
		}
		return *existing, nil
	}

	// Create new preference.
	if pref.ID == "" {
		pref.ID = ksuid.New().String()
	}
	if pref.ObservationCount == 0 {
		pref.ObservationCount = 1
	}
	if pref.Confidence == 0 {
		pref.Confidence = computeConfidence(pref.ObservationCount)
	}
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	if pref.LastObserved.IsZero() {
		pref.LastObserved = now
	}

	prefs = append(prefs, pref)
	if err := s.saveLocked(pref.UserID, prefs); err != nil {
		return Preference{}, err
	}
	return pref, nil
}

// GetPreference retrieves a specific preference by UserID, category, and key.
func (s *FilePreferenceStore) GetPreference(_ context.Context, userID, category, key string) (Preference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Preference{}, fmt.Errorf("preference user_id is required")
	}
	if category == "" {
		return Preference{}, fmt.Errorf("preference category is required")
	}
	if key == "" {
		return Preference{}, fmt.Errorf("preference key is required")
	}

	prefs, err := s.loadLocked(userID)
	if err != nil {
		return Preference{}, err
	}

	idx := s.findPreference(prefs, userID, PreferenceCategory(category), key)
	if idx < 0 {
		return Preference{}, fmt.Errorf("preference not found: %s/%s/%s", userID, category, key)
	}
	return prefs[idx], nil
}

// SearchPreferences finds preferences matching the query criteria.
// Results are sorted by Confidence descending.
func (s *FilePreferenceStore) SearchPreferences(_ context.Context, query PreferenceQuery) ([]Preference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query.UserID = strings.TrimSpace(query.UserID)
	if query.UserID == "" {
		return nil, fmt.Errorf("preference query user_id is required")
	}

	prefs, err := s.loadLocked(query.UserID)
	if err != nil {
		return nil, err
	}

	categorySet := make(map[PreferenceCategory]bool, len(query.Categories))
	for _, c := range query.Categories {
		categorySet[c] = true
	}
	keySet := make(map[string]bool, len(query.Keys))
	for _, k := range query.Keys {
		keySet[strings.ToLower(k)] = true
	}

	var results []Preference
	for _, p := range prefs {
		if len(categorySet) > 0 && !categorySet[p.Category] {
			continue
		}
		if len(keySet) > 0 && !keySet[strings.ToLower(p.Key)] {
			continue
		}
		if query.MinConfidence > 0 && p.Confidence < query.MinConfidence {
			continue
		}
		results = append(results, p)
	}

	// Sort by Confidence descending (highest confidence first).
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}
	return results, nil
}

// InferPreferences returns all preferences for a user with confidence > 0.5,
// sorted by confidence descending.
func (s *FilePreferenceStore) InferPreferences(_ context.Context, userID string) ([]Preference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("preference user_id is required")
	}

	prefs, err := s.loadLocked(userID)
	if err != nil {
		return nil, err
	}

	var results []Preference
	for _, p := range prefs {
		if p.Confidence > 0.5 {
			results = append(results, p)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	return results, nil
}

// DeletePreference removes the preference identified by id.
func (s *FilePreferenceStore) DeletePreference(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("preference id is required")
	}

	userID, idx, prefs, err := s.findPreferenceByID(id)
	if err != nil {
		return err
	}

	prefs = append(prefs[:idx], prefs[idx+1:]...)
	return s.saveLocked(userID, prefs)
}

// MergePreferences merges two preference lists, preferring higher confidence.
// When both lists contain a preference with the same Category+Key, the one
// with higher confidence wins. Preferences unique to either list are included.
func MergePreferences(existing, observed []Preference) []Preference {
	type prefKey struct {
		category PreferenceCategory
		key      string
	}

	merged := make(map[prefKey]Preference, len(existing)+len(observed))

	for _, p := range existing {
		k := prefKey{p.Category, p.Key}
		merged[k] = p
	}

	for _, p := range observed {
		k := prefKey{p.Category, p.Key}
		if current, ok := merged[k]; ok {
			if p.Confidence > current.Confidence {
				merged[k] = p
			}
		} else {
			merged[k] = p
		}
	}

	results := make([]Preference, 0, len(merged))
	for _, p := range merged {
		results = append(results, p)
	}

	// Sort by Confidence descending for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Confidence != results[j].Confidence {
			return results[i].Confidence > results[j].Confidence
		}
		// Tie-break by Key for deterministic ordering.
		return results[i].Key < results[j].Key
	})

	return results
}

// --- internal helpers ---

// computeConfidence calculates confidence as min(1.0, 0.3 + 0.1*observationCount).
func computeConfidence(observationCount int) float64 {
	c := 0.3 + 0.1*float64(observationCount)
	if c > 1.0 {
		return 1.0
	}
	return c
}

// loadLocked reads preferences from disk for a user. Must be called with s.mu held.
func (s *FilePreferenceStore) loadLocked(userID string) ([]Preference, error) {
	if cached, ok := s.cache[userID]; ok {
		return cached, nil
	}

	filePath := s.preferenceFilePath(userID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.cache[userID] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read preference file: %w", err)
	}

	var prefs []Preference
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("unmarshal preferences: %w", err)
	}

	s.cache[userID] = prefs
	return prefs, nil
}

// saveLocked writes preferences to disk for a user. Must be called with s.mu held.
func (s *FilePreferenceStore) saveLocked(userID string, prefs []Preference) error {
	dirPath := filepath.Join(s.basePath, userID)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("create preference dir: %w", err)
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}

	filePath := s.preferenceFilePath(userID)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write preference file: %w", err)
	}

	s.cache[userID] = prefs
	return nil
}

func (s *FilePreferenceStore) preferenceFilePath(userID string) string {
	return filepath.Join(s.basePath, userID, "preferences.json")
}

// findPreference locates a preference in the slice by UserID+Category+Key.
// Returns the index, or -1 if not found.
func (s *FilePreferenceStore) findPreference(prefs []Preference, userID string, category PreferenceCategory, key string) int {
	keyLower := strings.ToLower(key)
	for i, p := range prefs {
		if p.UserID == userID &&
			p.Category == category &&
			strings.ToLower(p.Key) == keyLower {
			return i
		}
	}
	return -1
}

// findPreferenceByID scans all user directories to find a preference by its ID.
// Returns the userID, index within the prefs slice, the full slice, or an error.
func (s *FilePreferenceStore) findPreferenceByID(id string) (string, int, []Preference, error) {
	// First check cached users.
	for userID, prefs := range s.cache {
		for i, p := range prefs {
			if p.ID == id {
				return userID, i, prefs, nil
			}
		}
	}

	// Scan all user directories on disk.
	dirEntries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, nil, fmt.Errorf("preference not found: %s", id)
		}
		return "", 0, nil, fmt.Errorf("read base path: %w", err)
	}

	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		userID := de.Name()
		if _, cached := s.cache[userID]; cached {
			continue // already checked above
		}
		prefs, err := s.loadLocked(userID)
		if err != nil {
			continue
		}
		for i, p := range prefs {
			if p.ID == id {
				return userID, i, prefs, nil
			}
		}
	}

	return "", 0, nil, fmt.Errorf("preference not found: %s", id)
}
