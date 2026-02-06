package skills

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// FeedbackConfig configures skill feedback persistence.
type FeedbackConfig struct {
	Enabled   bool
	StorePath string
}

// FeedbackStats tracks skill usage and feedback.
type FeedbackStats struct {
	Name       string    `yaml:"name"`
	Activated  int       `yaml:"activated"`
	Helpful    int       `yaml:"helpful"`
	NotHelpful int       `yaml:"not_helpful"`
	LastUsed   time.Time `yaml:"last_used"`
}

// HelpfulRatio returns the helpful ratio for the skill.
func (s FeedbackStats) HelpfulRatio() float64 {
	total := s.Helpful + s.NotHelpful
	if total == 0 {
		return 0
	}
	return float64(s.Helpful) / float64(total)
}

// FeedbackStore persists skill feedback statistics.
type FeedbackStore struct {
	mu    sync.Mutex
	path  string
	stats map[string]FeedbackStats
}

// NewFeedbackStore initializes a feedback store or returns nil when disabled.
func NewFeedbackStore(cfg FeedbackConfig) *FeedbackStore {
	if !cfg.Enabled {
		return nil
	}
	path := strings.TrimSpace(cfg.StorePath)
	if path == "" {
		return nil
	}
	store := &FeedbackStore{
		path:  path,
		stats: make(map[string]FeedbackStats),
	}
	store.load()
	return store
}

// GetStats returns stored feedback stats for a skill.
func (s *FeedbackStore) GetStats(name string) (FeedbackStats, bool) {
	if s == nil {
		return FeedbackStats{}, false
	}
	key := NormalizeName(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, ok := s.stats[key]
	return stats, ok
}

// RecordActivation records a skill activation event.
func (s *FeedbackStore) RecordActivation(name string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := NormalizeName(name)
	stats := s.stats[key]
	if stats.Name == "" {
		stats.Name = name
	}
	stats.Activated++
	stats.LastUsed = time.Now()
	s.stats[key] = stats
	s.saveLocked()
}

// RecordActivations records multiple activations at once.
func (s *FeedbackStore) RecordActivations(matches []MatchResult) {
	if s == nil {
		return
	}
	for _, match := range matches {
		s.RecordActivation(match.Skill.Name)
	}
}

// RecordFeedback records explicit helpfulness feedback.
func (s *FeedbackStore) RecordFeedback(name string, helpful bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := NormalizeName(name)
	stats := s.stats[key]
	if stats.Name == "" {
		stats.Name = name
	}
	if helpful {
		stats.Helpful++
	} else {
		stats.NotHelpful++
	}
	stats.LastUsed = time.Now()
	s.stats[key] = stats
	s.saveLocked()
}

func (s *FeedbackStore) load() {
	if s == nil || s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var stored struct {
		Stats []FeedbackStats `yaml:"stats"`
	}
	if err := yaml.Unmarshal(data, &stored); err != nil {
		return
	}
	for _, stat := range stored.Stats {
		if stat.Name == "" {
			continue
		}
		s.stats[NormalizeName(stat.Name)] = stat
	}
}

func (s *FeedbackStore) saveLocked() {
	if s == nil || s.path == "" {
		return
	}
	stats := make([]FeedbackStats, 0, len(s.stats))
	for _, stat := range s.stats {
		stats = append(stats, stat)
	}
	payload, err := yaml.Marshal(struct {
		Stats []FeedbackStats `yaml:"stats"`
	}{Stats: stats})
	if err != nil {
		return
	}
	dir := filepath.Dir(s.path)
	if dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	_ = os.WriteFile(s.path, payload, 0o644)
}
