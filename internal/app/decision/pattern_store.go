package decision

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

// PatternStore is a file-backed JSON store for decision patterns with checksum validation.
type PatternStore struct {
	mu       sync.RWMutex
	patterns map[string]*Pattern
	filePath string
}

// NewPatternStore creates a PatternStore backed by the given file path.
func NewPatternStore(filePath string) (*PatternStore, error) {
	ps := &PatternStore{patterns: make(map[string]*Pattern), filePath: filePath}
	if filePath != "" {
		if err := ps.load(); err != nil {
			return nil, fmt.Errorf("pattern store load: %w", err)
		}
	}
	return ps, nil
}

// Save persists a pattern, computing its checksum.
func (ps *PatternStore) Save(p *Pattern) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	p.Checksum = computeChecksum(p)
	stored := *p
	ps.patterns[p.ID] = &stored
	return ps.persistLocked()
}

// Get returns a pattern by ID, or nil if not found.
func (ps *PatternStore) Get(id string) *Pattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.copyPattern(id)
}

// List returns all patterns sorted by creation time descending.
func (ps *PatternStore) List() []*Pattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.snapshotSorted()
}

// FindMatching returns patterns that match the given category and condition substring.
func (ps *PatternStore) FindMatching(category, condition string) []*Pattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var results []*Pattern
	for _, p := range ps.patterns {
		if p.Category == category && strings.Contains(p.Condition, condition) {
			cp := *p
			results = append(results, &cp)
		}
	}
	// Sort by confidence descending for deterministic ordering.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})
	return results
}

// Delete removes a pattern by ID.
func (ps *PatternStore) Delete(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, exists := ps.patterns[id]; !exists {
		return fmt.Errorf("pattern %q not found", id)
	}
	delete(ps.patterns, id)
	return ps.persistLocked()
}

// ValidateIntegrity returns IDs of patterns with invalid checksums.
func (ps *PatternStore) ValidateIntegrity() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var corrupt []string
	for _, p := range ps.patterns {
		if computeChecksum(p) != p.Checksum {
			corrupt = append(corrupt, p.ID)
		}
	}
	return corrupt
}

// RunIntegrityScan finds corrupt patterns and sets their confidence to zero.
func (ps *PatternStore) RunIntegrityScan() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	changed := false
	for _, p := range ps.patterns {
		if computeChecksum(p) != p.Checksum {
			p.Confidence = 0
			changed = true
		}
	}
	if changed {
		return ps.persistLocked()
	}
	return nil
}

func computeChecksum(p *Pattern) string {
	h := sha256.Sum256([]byte(p.Category + p.Condition + p.Action))
	return fmt.Sprintf("%x", h)
}

func (ps *PatternStore) copyPattern(id string) *Pattern {
	p, ok := ps.patterns[id]
	if !ok {
		return nil
	}
	cp := *p
	return &cp
}

func (ps *PatternStore) snapshotSorted() []*Pattern {
	result := make([]*Pattern, 0, len(ps.patterns))
	for _, p := range ps.patterns {
		cp := *p
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (ps *PatternStore) load() error {
	data, err := filestore.ReadFileOrEmpty(ps.filePath)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	var patterns []*Pattern
	if err := jsonx.Unmarshal(data, &patterns); err != nil {
		return fmt.Errorf("parse pattern store: %w", err)
	}
	for _, p := range patterns {
		ps.patterns[p.ID] = p
	}
	return nil
}

func (ps *PatternStore) persistLocked() error {
	if ps.filePath == "" {
		return nil
	}
	data, err := filestore.MarshalJSONIndent(ps.snapshotSorted())
	if err != nil {
		return fmt.Errorf("marshal pattern store: %w", err)
	}
	return filestore.AtomicWrite(ps.filePath, data, 0o600)
}
