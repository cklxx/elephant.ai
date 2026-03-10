// Package decision provides a file-backed store for recording and querying
// team decisions. Each decision captures who decided, when, what alternatives
// were considered, and the outcome. Decisions can be tagged and searched by
// topic, date range, or participant. The store also generates Markdown
// summaries suitable for 1:1 prep briefs.
package decision

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

// Decision records a single team decision with its context.
type Decision struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	DecidedBy    string            `json:"decided_by"`
	Participants []string          `json:"participants,omitempty"`
	Alternatives []string          `json:"alternatives,omitempty"`
	Outcome      string            `json:"outcome"`
	Tags         []string          `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// SearchFilter specifies criteria for querying decisions.
type SearchFilter struct {
	Topic       string    // substring match against title, description, or tags
	Tags        []string  // all listed tags must be present
	Participant string    // must appear in decided_by or participants
	After       time.Time // decisions created after this time
	Before      time.Time // decisions created before this time
}

const persistVersion = 1

type persistedData struct {
	Version   int         `json:"version"`
	Decisions []*Decision `json:"decisions"`
}

// Store is a thread-safe, file-backed decision store.
type Store struct {
	mu       sync.RWMutex
	items    map[string]*Decision
	filePath string
}

// NewStore creates a Store backed by the given file path.
// If filePath is empty, the store runs in memory-only mode.
// Existing data is loaded from disk on creation.
func NewStore(filePath string) (*Store, error) {
	s := &Store{
		items:    make(map[string]*Decision),
		filePath: filePath,
	}
	if filePath != "" {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("decision store load: %w", err)
		}
	}
	return s, nil
}

// Add records a new decision. The ID must be unique.
func (s *Store) Add(d *Decision) error {
	if d.ID == "" {
		return fmt.Errorf("decision ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[d.ID]; exists {
		return fmt.Errorf("decision %q already exists", d.ID)
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	d.UpdatedAt = d.CreatedAt
	stored := *d
	s.items[d.ID] = &stored
	return s.persistLocked()
}

// Get returns a decision by ID, or nil if not found.
func (s *Store) Get(id string) *Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.items[id]
	if !ok {
		return nil
	}
	copy := *d
	return &copy
}

// Update replaces an existing decision. Returns an error if not found.
func (s *Store) Update(d *Decision) error {
	if d.ID == "" {
		return fmt.Errorf("decision ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[d.ID]; !exists {
		return fmt.Errorf("decision %q not found", d.ID)
	}
	d.UpdatedAt = time.Now()
	stored := *d
	s.items[d.ID] = &stored
	return s.persistLocked()
}

// Delete removes a decision by ID. Returns an error if not found.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[id]; !exists {
		return fmt.Errorf("decision %q not found", id)
	}
	delete(s.items, id)
	return s.persistLocked()
}

// List returns all decisions sorted by creation time descending (newest first).
func (s *Store) List() []*Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotSorted()
}

// Search returns decisions matching the filter, sorted newest first.
func (s *Store) Search(f SearchFilter) []*Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Decision
	for _, d := range s.items {
		if matchesFilter(d, f) {
			copy := *d
			results = append(results, &copy)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results
}

// FormatSummary generates a Markdown summary of decisions within the given
// lookback window, suitable for inclusion in a 1:1 prep brief.
func (s *Store) FormatSummary(lookback time.Duration, participant string) string {
	cutoff := time.Now().Add(-lookback)
	filter := SearchFilter{After: cutoff}
	if participant != "" {
		filter.Participant = participant
	}
	decisions := s.Search(filter)

	var out strings.Builder
	out.WriteString("### Recent Decisions\n\n")
	if len(decisions) == 0 {
		out.WriteString("No decisions recorded in this period.\n")
		return out.String()
	}
	for _, d := range decisions {
		out.WriteString(fmt.Sprintf("- **%s** (%s)", d.Title, d.CreatedAt.Format("2006-01-02")))
		if d.Outcome != "" {
			out.WriteString(fmt.Sprintf(" — %s", truncate(d.Outcome, 120)))
		}
		out.WriteString("\n")
		if len(d.Tags) > 0 {
			out.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(d.Tags, ", ")))
		}
	}
	return out.String()
}

// --- internal helpers ---

func (s *Store) snapshotSorted() []*Decision {
	result := make([]*Decision, 0, len(s.items))
	for _, d := range s.items {
		copy := *d
		result = append(result, &copy)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *Store) load() error {
	data, err := filestore.ReadFileOrEmpty(s.filePath)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	var pd persistedData
	if err := jsonx.Unmarshal(data, &pd); err != nil {
		return fmt.Errorf("parse decision store: %w", err)
	}
	for _, d := range pd.Decisions {
		s.items[d.ID] = d
	}
	return nil
}

func (s *Store) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	pd := persistedData{
		Version:   persistVersion,
		Decisions: s.snapshotSorted(),
	}
	data, err := filestore.MarshalJSONIndent(pd)
	if err != nil {
		return fmt.Errorf("marshal decision store: %w", err)
	}
	return filestore.AtomicWrite(s.filePath, data, 0o600)
}

func matchesFilter(d *Decision, f SearchFilter) bool {
	if !f.After.IsZero() && !d.CreatedAt.After(f.After) {
		return false
	}
	if !f.Before.IsZero() && !d.CreatedAt.Before(f.Before) {
		return false
	}
	if f.Participant != "" {
		found := strings.EqualFold(d.DecidedBy, f.Participant)
		if !found {
			for _, p := range d.Participants {
				if strings.EqualFold(p, f.Participant) {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	if f.Topic != "" {
		topic := strings.ToLower(f.Topic)
		if !strings.Contains(strings.ToLower(d.Title), topic) &&
			!strings.Contains(strings.ToLower(d.Description), topic) &&
			!containsTag(d.Tags, topic) {
			return false
		}
	}
	if len(f.Tags) > 0 {
		tagSet := make(map[string]bool, len(d.Tags))
		for _, t := range d.Tags {
			tagSet[strings.ToLower(t)] = true
		}
		for _, required := range f.Tags {
			if !tagSet[strings.ToLower(required)] {
				return false
			}
		}
	}
	return true
}

func containsTag(tags []string, topic string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), topic) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
