package leader

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// DecisionRecord captures a single stall-handling decision and its outcome.
type DecisionRecord struct {
	Attempt   int
	Action    string // "INJECT", "FAIL", "ESCALATE"
	Argument  string
	Timestamp time.Time

	// Outcome is filled asynchronously after the decision is applied.
	// Possible values: "recovered", "still_stalled", "" (unknown/pending).
	Outcome   string
	OutcomeAt time.Time
}

// DecisionHistory tracks stall-handling decisions for a single runtime session.
// It is safe for concurrent use.
type DecisionHistory struct {
	mu      sync.RWMutex
	records []DecisionRecord
}

// Add appends a decision record.
func (h *DecisionHistory) Add(r DecisionRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
}

// Len returns the number of recorded decisions.
func (h *DecisionHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.records)
}

// Last returns the most recent n records (oldest first).
func (h *DecisionHistory) Last(n int) []DecisionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n <= 0 || len(h.records) == 0 {
		return nil
	}
	start := len(h.records) - n
	if start < 0 {
		start = 0
	}
	out := make([]DecisionRecord, len(h.records)-start)
	copy(out, h.records[start:])
	return out
}

// RecordOutcome updates the outcome of the most recent decision.
func (h *DecisionHistory) RecordOutcome(outcome string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		return
	}
	last := &h.records[len(h.records)-1]
	if last.Outcome == "" {
		last.Outcome = outcome
		last.OutcomeAt = time.Now()
	}
}

// RecordOutcomeForAttempt updates the outcome for a specific attempt if it is still pending.
func (h *DecisionHistory) RecordOutcomeForAttempt(attempt int, outcome string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.records) - 1; i >= 0; i-- {
		record := &h.records[i]
		if record.Attempt != attempt {
			continue
		}
		if record.Outcome == "" {
			record.Outcome = outcome
			record.OutcomeAt = time.Now()
		}
		return
	}
}

// InjectSuccessRate returns the success rate for evaluated INJECT decisions.
// Pending INJECT outcomes are ignored.
func (h *DecisionHistory) InjectSuccessRate() float64 {
	successes, total := h.injectStats()
	if total == 0 {
		return 0
	}
	return float64(successes) / float64(total)
}

// InjectEvaluatedCount returns the number of INJECT decisions with a final outcome.
func (h *DecisionHistory) InjectEvaluatedCount() int {
	_, total := h.injectStats()
	return total
}

// SummaryForPrompt formats the last n decisions as a text block suitable for
// inclusion in a stall prompt. Returns empty string if no history exists.
func (h *DecisionHistory) SummaryForPrompt(n int) string {
	records := h.Last(n)
	if len(records) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Previous decisions for this session:\n")
	for _, r := range records {
		outcome := r.Outcome
		if outcome == "" {
			outcome = "pending"
		}
		b.WriteString(fmt.Sprintf("  Attempt %d: %s %s → %s\n", r.Attempt, r.Action, r.Argument, outcome))
	}
	return b.String()
}

func (h *DecisionHistory) injectStats() (int, int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	successes := 0
	total := 0
	for _, record := range h.records {
		if record.Action != "INJECT" || record.Outcome == "" {
			continue
		}
		total++
		if record.Outcome == "recovered" {
			successes++
		}
	}
	return successes, total
}

// decisionHistoryStore manages per-session decision histories.
// It is safe for concurrent use.
type decisionHistoryStore struct {
	mu        sync.Mutex
	histories map[string]*DecisionHistory
}

func newDecisionHistoryStore() *decisionHistoryStore {
	return &decisionHistoryStore{
		histories: make(map[string]*DecisionHistory),
	}
}

// Get returns (or creates) the history for a session.
func (s *decisionHistoryStore) Get(sessionID string) *DecisionHistory {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.histories[sessionID]
	if !ok {
		h = &DecisionHistory{}
		s.histories[sessionID] = h
	}
	return h
}

// Delete removes the history for a session (e.g. on recovery).
func (s *decisionHistoryStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.histories, sessionID)
}
