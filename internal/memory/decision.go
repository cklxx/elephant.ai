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

// DecisionEntry captures a single decision record for a user.
type DecisionEntry struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	SessionID      string     `json:"session_id"`
	Decision       string     `json:"decision"`
	Rationale      string     `json:"rationale"`
	Context        string     `json:"context"`
	Alternatives   []string   `json:"alternatives,omitempty"`
	Outcome        string     `json:"outcome,omitempty"`
	OutcomeSuccess bool       `json:"outcome_success,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// DecisionQuery describes a search request for decisions.
type DecisionQuery struct {
	UserID         string    `json:"user_id"`
	Tags           []string  `json:"tags,omitempty"`
	Text           string    `json:"text,omitempty"`
	OnlyUnresolved bool      `json:"only_unresolved,omitempty"`
	Since          time.Time `json:"since,omitempty"`
	Limit          int       `json:"limit,omitempty"`
}

// DecisionStore abstracts persistence for decision memories.
type DecisionStore interface {
	RecordDecision(ctx context.Context, entry DecisionEntry) (DecisionEntry, error)
	ResolveDecision(ctx context.Context, id string, outcome string, success bool) error
	SearchDecisions(ctx context.Context, query DecisionQuery) ([]DecisionEntry, error)
	RecentDecisions(ctx context.Context, userID string, limit int) ([]DecisionEntry, error)
}

// FileDecisionStore implements DecisionStore by persisting decisions as JSON files per user.
type FileDecisionStore struct {
	basePath string
	mu       sync.Mutex
	cache    map[string][]DecisionEntry // userID -> decisions
}

// NewFileDecisionStore creates a file-backed decision store rooted at basePath.
func NewFileDecisionStore(basePath string) *FileDecisionStore {
	return &FileDecisionStore{
		basePath: basePath,
		cache:    make(map[string][]DecisionEntry),
	}
}

// RecordDecision saves a new decision and returns it with a generated ID.
func (s *FileDecisionStore) RecordDecision(_ context.Context, entry DecisionEntry) (DecisionEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.UserID = strings.TrimSpace(entry.UserID)
	if entry.UserID == "" {
		return entry, fmt.Errorf("user_id is required")
	}
	entry.Decision = strings.TrimSpace(entry.Decision)
	if entry.Decision == "" {
		return entry, fmt.Errorf("decision is required")
	}

	if entry.ID == "" {
		entry.ID = ksuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	decisions, err := s.loadLocked(entry.UserID)
	if err != nil {
		return entry, fmt.Errorf("load decisions: %w", err)
	}

	decisions = append(decisions, entry)
	s.cache[entry.UserID] = decisions

	if err := s.saveLocked(entry.UserID, decisions); err != nil {
		return entry, fmt.Errorf("save decisions: %w", err)
	}

	return entry, nil
}

// ResolveDecision records the outcome for a previously recorded decision.
func (s *FileDecisionStore) ResolveDecision(_ context.Context, id string, outcome string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("decision id is required")
	}

	for userID, decisions := range s.cache {
		for i := range decisions {
			if decisions[i].ID == id {
				now := time.Now()
				decisions[i].Outcome = outcome
				decisions[i].OutcomeSuccess = success
				decisions[i].ResolvedAt = &now
				s.cache[userID] = decisions
				return s.saveLocked(userID, decisions)
			}
		}
	}

	// Not in cache; scan all user directories on disk.
	userDirs, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("decision not found: %s", id)
		}
		return fmt.Errorf("read base dir: %w", err)
	}

	for _, dir := range userDirs {
		if !dir.IsDir() {
			continue
		}
		userID := dir.Name()
		if _, cached := s.cache[userID]; cached {
			continue // already checked above
		}
		decisions, err := s.loadLocked(userID)
		if err != nil {
			continue
		}
		for i := range decisions {
			if decisions[i].ID == id {
				now := time.Now()
				decisions[i].Outcome = outcome
				decisions[i].OutcomeSuccess = success
				decisions[i].ResolvedAt = &now
				s.cache[userID] = decisions
				return s.saveLocked(userID, decisions)
			}
		}
	}

	return fmt.Errorf("decision not found: %s", id)
}

// SearchDecisions finds decisions matching the given query.
func (s *FileDecisionStore) SearchDecisions(_ context.Context, query DecisionQuery) ([]DecisionEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query.UserID = strings.TrimSpace(query.UserID)
	if query.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	decisions, err := s.loadLocked(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("load decisions: %w", err)
	}

	var results []DecisionEntry
	textLower := strings.ToLower(strings.TrimSpace(query.Text))
	tagSet := makeTagSet(query.Tags)

	for _, d := range decisions {
		if query.OnlyUnresolved && d.ResolvedAt != nil {
			continue
		}
		if !query.Since.IsZero() && d.CreatedAt.Before(query.Since) {
			continue
		}
		if len(tagSet) > 0 && !matchesAnyTag(d.Tags, tagSet) {
			continue
		}
		if textLower != "" && !matchesText(d, textLower) {
			continue
		}
		results = append(results, d)
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	return results, nil
}

// RecentDecisions returns the most recent decisions for a user, sorted by CreatedAt descending.
func (s *FileDecisionStore) RecentDecisions(_ context.Context, userID string, limit int) ([]DecisionEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	decisions, err := s.loadLocked(userID)
	if err != nil {
		return nil, fmt.Errorf("load decisions: %w", err)
	}

	// Decisions are stored in insertion order; sort descending by CreatedAt.
	sorted := make([]DecisionEntry, len(decisions))
	copy(sorted, decisions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	if limit > 0 && limit < len(sorted) {
		sorted = sorted[:limit]
	}

	return sorted, nil
}

// loadLocked reads decisions from disk for a user. Must be called with s.mu held.
func (s *FileDecisionStore) loadLocked(userID string) ([]DecisionEntry, error) {
	if cached, ok := s.cache[userID]; ok {
		return cached, nil
	}

	filePath := s.decisionFilePath(userID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.cache[userID] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read decision file: %w", err)
	}

	var decisions []DecisionEntry
	if err := json.Unmarshal(data, &decisions); err != nil {
		return nil, fmt.Errorf("unmarshal decisions: %w", err)
	}

	s.cache[userID] = decisions
	return decisions, nil
}

// saveLocked writes decisions to disk for a user. Must be called with s.mu held.
func (s *FileDecisionStore) saveLocked(userID string, decisions []DecisionEntry) error {
	dirPath := filepath.Join(s.basePath, userID)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("create user dir: %w", err)
	}

	data, err := json.MarshalIndent(decisions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}

	filePath := s.decisionFilePath(userID)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write decision file: %w", err)
	}

	return nil
}

func (s *FileDecisionStore) decisionFilePath(userID string) string {
	return filepath.Join(s.basePath, userID, "decisions.json")
}

// makeTagSet builds a lowercase set from a tag slice.
func makeTagSet(tags []string) map[string]bool {
	if len(tags) == 0 {
		return nil
	}
	set := make(map[string]bool, len(tags))
	for _, tag := range tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		if t != "" {
			set[t] = true
		}
	}
	return set
}

// matchesAnyTag returns true if any of the entry's tags appear in the tag set.
func matchesAnyTag(entryTags []string, tagSet map[string]bool) bool {
	for _, tag := range entryTags {
		if tagSet[strings.ToLower(strings.TrimSpace(tag))] {
			return true
		}
	}
	return false
}

// matchesText returns true if the search text appears in the decision's Decision, Rationale, or Context fields.
func matchesText(entry DecisionEntry, textLower string) bool {
	if strings.Contains(strings.ToLower(entry.Decision), textLower) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Rationale), textLower) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Context), textLower) {
		return true
	}
	return false
}
