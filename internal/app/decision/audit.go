package decision

import (
	"fmt"
	"sync"
	"time"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

const undoWindow = 24 * time.Hour

// AuditEntry records an auto-acted or escalated decision.
type AuditEntry struct {
	ID         string    `json:"id"`
	PatternID  string    `json:"pattern_id"`
	Category   string    `json:"category"`
	Action     string    `json:"action"`
	Confidence float64   `json:"confidence"`
	AutoActed  bool      `json:"auto_acted"`
	Corrected  bool      `json:"corrected,omitempty"`
	Correction string    `json:"correction,omitempty"`
	UndoBy     time.Time `json:"undo_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuditMetrics summarizes decision audit performance.
type AuditMetrics struct {
	TotalDecisions int
	AutoActed      int
	Escalated      int
	Corrections    int
	AccuracyRate   float64
}

// AuditLog is a file-backed audit trail for decision actions.
type AuditLog struct {
	mu       sync.RWMutex
	entries  []AuditEntry
	filePath string
	nowFn    func() time.Time
}

// NewAuditLog creates an AuditLog backed by the given file path.
func NewAuditLog(filePath string, nowFn func() time.Time) (*AuditLog, error) {
	a := &AuditLog{filePath: filePath, nowFn: nowFn}
	if filePath != "" {
		if err := a.load(); err != nil {
			return nil, fmt.Errorf("audit log load: %w", err)
		}
	}
	return a, nil
}

// Record appends an audit entry and persists it.
func (a *AuditLog) Record(entry AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.entries = append(a.entries, entry)
	return a.persistLocked()
}

// Get returns an audit entry by ID, or nil if not found.
func (a *AuditLog) Get(id string) *AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for i := range a.entries {
		if a.entries[i].ID == id {
			cp := a.entries[i]
			return &cp
		}
	}
	return nil
}

// ListRecent returns the most recent entries, up to limit.
func (a *AuditLog) ListRecent(limit int) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	n := len(a.entries)
	if limit > n {
		limit = n
	}
	result := make([]AuditEntry, limit)
	copy(result, a.entries[n-limit:])
	return result
}

// Undo marks an entry as corrected if within the undo window.
func (a *AuditLog) Undo(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.entries {
		if a.entries[i].ID != id {
			continue
		}
		if a.nowFn().After(a.entries[i].UndoBy) {
			return fmt.Errorf("undo window expired for %q", id)
		}
		a.entries[i].Corrected = true
		return a.persistLocked()
	}
	return fmt.Errorf("audit entry %q not found", id)
}

// Metrics computes summary statistics over all entries.
func (a *AuditLog) Metrics() AuditMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	m := AuditMetrics{TotalDecisions: len(a.entries)}
	for _, e := range a.entries {
		if e.AutoActed {
			m.AutoActed++
		} else {
			m.Escalated++
		}
		if e.Corrected {
			m.Corrections++
		}
	}
	if m.AutoActed > 0 {
		m.AccuracyRate = float64(m.AutoActed-m.Corrections) / float64(m.AutoActed)
	}
	return m
}

func (a *AuditLog) load() error {
	data, err := filestore.ReadFileOrEmpty(a.filePath)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	return jsonx.Unmarshal(data, &a.entries)
}

func (a *AuditLog) persistLocked() error {
	if a.filePath == "" {
		return nil
	}
	data, err := filestore.MarshalJSONIndent(a.entries)
	if err != nil {
		return fmt.Errorf("marshal audit log: %w", err)
	}
	return filestore.AtomicWrite(a.filePath, data, 0o600)
}
