package budget

import "sync"

// BudgetState represents the current state of a session's token budget.
type BudgetState string

const (
	BudgetOK       BudgetState = "ok"
	BudgetWarning  BudgetState = "warning"
	BudgetExceeded BudgetState = "exceeded"
)

// SessionQuota defines per-session token and cost limits.
type SessionQuota struct {
	MaxInputTokens   int     // Maximum cumulative input tokens (0 = unlimited)
	MaxOutputTokens  int     // Maximum cumulative output tokens (0 = unlimited)
	MaxTotalTokens   int     // Maximum cumulative total tokens (0 = unlimited)
	MaxCostUSD       float64 // Maximum cumulative cost in USD (0 = unlimited)
	WarningThreshold float64 // Fraction at which to warn (e.g. 0.8 = 80%)
}

// Usage captures cumulative token consumption for a session.
type Usage struct {
	InputTokens     int
	OutputTokens    int
	TotalTokens     int
	EstimatedCostUSD float64
	TurnCount       int
}

// BudgetCheck is the result of evaluating a session's budget state.
type BudgetCheck struct {
	State           BudgetState
	RemainingTokens int     // Remaining total tokens before quota; -1 if unlimited
	UsagePercent    float64 // Highest usage ratio across all tracked dimensions (0-1)
	SuggestedModel  string  // Non-empty when a cheaper model is recommended
}

// ModelTier describes a model and its relative cost for downgrade decisions.
type ModelTier struct {
	Name           string  // Model identifier (e.g. "gpt-4", "deepseek-chat")
	CostPer1KInput float64 // Cost per 1K input tokens in USD
	Priority       int     // Lower value = cheaper model
}

// DefaultModelTiers provides a reference ordering of common models from
// cheapest to most expensive, used for downgrade suggestions.
var DefaultModelTiers = []ModelTier{
	{Name: "deepseek-chat", CostPer1KInput: 0.00014, Priority: 1},
	{Name: "gpt-3.5-turbo", CostPer1KInput: 0.0005, Priority: 2},
	{Name: "claude-3-haiku", CostPer1KInput: 0.00025, Priority: 3},
	{Name: "claude-3-sonnet", CostPer1KInput: 0.003, Priority: 4},
	{Name: "gpt-4", CostPer1KInput: 0.03, Priority: 5},
	{Name: "claude-3-opus", CostPer1KInput: 0.015, Priority: 6},
}

// sessionState holds the mutable usage data for a single session.
type sessionState struct {
	usage     Usage
	lastModel string
}

// Manager tracks per-session token budgets, enforces limits, and suggests
// model downgrades when usage approaches the configured quota.
type Manager struct {
	mu           sync.RWMutex
	sessions     map[string]*sessionState
	defaultQuota SessionQuota
	modelTiers   []ModelTier
}

// NewManager creates a Manager with the given default quota and model tier
// ordering. If tiers is nil, DefaultModelTiers is used.
func NewManager(defaultQuota SessionQuota, tiers []ModelTier) *Manager {
	if tiers == nil {
		tiers = DefaultModelTiers
	}
	return &Manager{
		sessions:     make(map[string]*sessionState),
		defaultQuota: defaultQuota,
		modelTiers:   tiers,
	}
}

// RecordUsage adds token consumption for the given session. It is safe for
// concurrent use.
func (m *Manager) RecordUsage(sessionID string, inputTokens, outputTokens int, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.getOrCreateLocked(sessionID)
	s.usage.InputTokens += inputTokens
	s.usage.OutputTokens += outputTokens
	s.usage.TotalTokens += inputTokens + outputTokens
	s.usage.EstimatedCostUSD += estimateCost(inputTokens, outputTokens, model, m.modelTiers)
	s.usage.TurnCount++
	s.lastModel = model
}

// CheckBudget evaluates the session's current usage against its quota and
// returns the budget state, remaining tokens, usage percentage, and an
// optional model downgrade suggestion.
func (m *Manager) CheckBudget(sessionID string) BudgetCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return BudgetCheck{
			State:           BudgetOK,
			RemainingTokens: m.remainingTotal(nil),
			UsagePercent:    0,
		}
	}

	usagePct := m.highestUsageRatio(&s.usage)
	state := m.stateFromRatio(usagePct)

	check := BudgetCheck{
		State:           state,
		RemainingTokens: m.remainingTotal(&s.usage),
		UsagePercent:    usagePct,
	}

	if state == BudgetWarning || state == BudgetExceeded {
		if suggested, ok := m.suggestDowngradeLocked(s.lastModel); ok {
			check.SuggestedModel = suggested
		}
	}

	return check
}

// SuggestDowngrade returns a cheaper model name and true if the session is
// above the warning threshold. If the current model is already the cheapest
// (or unknown), it returns ("", false).
func (m *Manager) SuggestDowngrade(sessionID string, currentModel string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return "", false
	}

	usagePct := m.highestUsageRatio(&s.usage)
	if usagePct < m.warningThreshold() {
		return "", false
	}

	return m.suggestDowngradeLocked(currentModel)
}

// GetUsage returns a snapshot of the session's current usage. If the session
// does not exist, a zero-value Usage is returned.
func (m *Manager) GetUsage(sessionID string) Usage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return Usage{}
	}
	return s.usage
}

// ResetSession clears all tracked usage for the given session.
func (m *Manager) ResetSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (m *Manager) getOrCreateLocked(sessionID string) *sessionState {
	s, ok := m.sessions[sessionID]
	if !ok {
		s = &sessionState{}
		m.sessions[sessionID] = s
	}
	return s
}

func (m *Manager) warningThreshold() float64 {
	if m.defaultQuota.WarningThreshold <= 0 || m.defaultQuota.WarningThreshold >= 1 {
		return 0.8
	}
	return m.defaultQuota.WarningThreshold
}

// highestUsageRatio computes the maximum usage fraction across all quota
// dimensions that are configured (non-zero).
func (m *Manager) highestUsageRatio(u *Usage) float64 {
	q := m.defaultQuota
	max := 0.0

	if q.MaxInputTokens > 0 {
		r := float64(u.InputTokens) / float64(q.MaxInputTokens)
		if r > max {
			max = r
		}
	}
	if q.MaxOutputTokens > 0 {
		r := float64(u.OutputTokens) / float64(q.MaxOutputTokens)
		if r > max {
			max = r
		}
	}
	if q.MaxTotalTokens > 0 {
		r := float64(u.TotalTokens) / float64(q.MaxTotalTokens)
		if r > max {
			max = r
		}
	}
	if q.MaxCostUSD > 0 {
		r := u.EstimatedCostUSD / q.MaxCostUSD
		if r > max {
			max = r
		}
	}

	return max
}

func (m *Manager) stateFromRatio(ratio float64) BudgetState {
	if ratio >= 1.0 {
		return BudgetExceeded
	}
	if ratio >= m.warningThreshold() {
		return BudgetWarning
	}
	return BudgetOK
}

func (m *Manager) remainingTotal(u *Usage) int {
	q := m.defaultQuota
	if q.MaxTotalTokens <= 0 {
		return -1
	}
	if u == nil {
		return q.MaxTotalTokens
	}
	remaining := q.MaxTotalTokens - u.TotalTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// suggestDowngradeLocked returns the next cheaper model tier relative to
// currentModel. Must be called with at least m.mu.RLock held.
func (m *Manager) suggestDowngradeLocked(currentModel string) (string, bool) {
	currentPriority := -1
	for _, t := range m.modelTiers {
		if t.Name == currentModel {
			currentPriority = t.Priority
			break
		}
	}

	if currentPriority < 0 {
		// Unknown model — cannot suggest a downgrade.
		return "", false
	}

	// Find the highest-priority (largest Priority value < currentPriority)
	// tier that is cheaper than the current one.
	var best *ModelTier
	for i := range m.modelTiers {
		t := &m.modelTiers[i]
		if t.Priority < currentPriority {
			if best == nil || t.Priority > best.Priority {
				best = t
			}
		}
	}

	if best == nil {
		// Already on the cheapest tier.
		return "", false
	}

	return best.Name, true
}

// estimateCost computes an approximate cost for a single recording based on
// the model tiers table. Falls back to a conservative default when the model
// is not in the tier list.
func estimateCost(inputTokens, outputTokens int, model string, tiers []ModelTier) float64 {
	for _, t := range tiers {
		if t.Name == model {
			// Use input cost as a proxy; output is typically 2x input.
			return (float64(inputTokens) * t.CostPer1KInput / 1000.0) +
				(float64(outputTokens) * t.CostPer1KInput * 2.0 / 1000.0)
		}
	}
	// Unknown model — use a conservative default ($0.001 / 1K tokens).
	return float64(inputTokens+outputTokens) * 0.001 / 1000.0
}
