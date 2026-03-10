package lark

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// UrgencyLevel classifies how urgently a message needs human attention.
type UrgencyLevel int

const (
	// UrgencyLow indicates a routine message that can be auto-acknowledged.
	UrgencyLow UrgencyLevel = iota
	// UrgencyNormal indicates a standard message processed normally.
	UrgencyNormal
	// UrgencyHigh indicates an urgent message that bypasses batching.
	UrgencyHigh
)

// AttentionGateConfig controls the attention gate behavior.
type AttentionGateConfig struct {
	// Enabled activates the attention gate. When false, all messages pass through.
	Enabled bool `yaml:"enabled"`
	// UrgentKeywords are strings that elevate a message to UrgencyHigh.
	// Matched case-insensitively against message content.
	UrgentKeywords []string `yaml:"urgent_keywords"`
	// AutoAckMessage is the reply sent for non-urgent messages.
	// Default: "收到，已记录并跟踪中。"
	AutoAckMessage string `yaml:"auto_ack_message"`
	// BudgetWindow is the sliding window for the message budget.
	// Default: 10 minutes.
	BudgetWindow time.Duration `yaml:"budget_window"`
	// BudgetMax is the maximum outgoing messages per budget window per chat.
	// 0 disables budget limiting.
	BudgetMax int `yaml:"budget_max"`
}

const defaultAutoAckMessage = "收到，已记录并跟踪中。"

// FocusTimeChecker determines whether a user is in a focus time window.
// When set on AttentionGate, non-urgent messages are suppressed for users
// currently in focus time.
type FocusTimeChecker interface {
	ShouldSuppress(userID string, now time.Time) bool
}

// AttentionGate filters messages based on urgency criteria and enforces
// a per-chat notification budget.
type AttentionGate struct {
	cfg AttentionGateConfig

	// lowerKeywords is the pre-lowered keyword set for fast matching.
	lowerKeywords []string

	// focusTime is an optional checker for focus time suppression.
	// When non-nil, non-urgent messages are suppressed during focus time.
	focusTime FocusTimeChecker

	mu      sync.Mutex
	budgets map[string]*chatBudget // chatID → budget tracker
}

type chatBudget struct {
	timestamps []time.Time
}

// NewAttentionGate creates an AttentionGate with the given config.
func NewAttentionGate(cfg AttentionGateConfig) *AttentionGate {
	lower := make([]string, 0, len(cfg.UrgentKeywords))
	for _, kw := range cfg.UrgentKeywords {
		trimmed := strings.TrimSpace(kw)
		if trimmed == "" {
			continue
		}
		lower = append(lower, strings.ToLower(trimmed))
	}
	if cfg.AutoAckMessage == "" {
		cfg.AutoAckMessage = defaultAutoAckMessage
	}
	if cfg.BudgetWindow <= 0 {
		cfg.BudgetWindow = 10 * time.Minute
	}
	return &AttentionGate{
		cfg:           cfg,
		lowerKeywords: lower,
		budgets:       make(map[string]*chatBudget),
	}
}

// ClassifyUrgency determines the urgency level of a message.
// A message is UrgencyHigh if it contains any configured urgent keyword
// or matches urgency patterns (deadline expressions, @mentions of specific
// patterns). Returns UrgencyLow for routine messages when the gate is enabled,
// or UrgencyNormal when disabled.
func (g *AttentionGate) ClassifyUrgency(content string) UrgencyLevel {
	if !g.cfg.Enabled {
		return UrgencyNormal
	}
	if content == "" {
		return UrgencyLow
	}

	lower := strings.ToLower(content)

	// Check configured urgent keywords.
	for _, kw := range g.lowerKeywords {
		if strings.Contains(lower, kw) {
			return UrgencyHigh
		}
	}

	// Built-in urgency patterns.
	if matchesBuiltinUrgencyPatterns(lower) {
		return UrgencyHigh
	}

	return UrgencyLow
}

// matchesBuiltinUrgencyPatterns checks for common urgency signals:
// deadline words, exclamation-heavy messages, and error/failure keywords.
func matchesBuiltinUrgencyPatterns(lower string) bool {
	urgentPatterns := []string{
		"紧急", "urgent", "asap", "deadline",
		"立刻", "马上", "immediately",
		"出错", "报错", "error", "失败", "failed", "故障",
		"挂了", "崩了", "down", "宕机",
		"blocked", "阻塞",
	}
	for _, p := range urgentPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// Messages that are mostly exclamation marks signal urgency.
	exclamations := strings.Count(lower, "!") + strings.Count(lower, "！")
	if exclamations >= 3 && utf8.RuneCountInString(lower) < 50 {
		return true
	}
	return false
}

// RecordDispatch records an outgoing message for budget tracking.
// Returns true if the message is within budget, false if over budget.
func (g *AttentionGate) RecordDispatch(chatID string, now time.Time) bool {
	if g.cfg.BudgetMax <= 0 {
		return true
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	b := g.budgets[chatID]
	if b == nil {
		b = &chatBudget{}
		g.budgets[chatID] = b
	}

	// Trim expired entries for this chat.
	cutoff := now.Add(-g.cfg.BudgetWindow)
	trimmed := b.timestamps[:0]
	for _, ts := range b.timestamps {
		if ts.After(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	b.timestamps = trimmed

	// Periodically GC stale budget entries for other chats to prevent
	// unbounded map growth. Run when the map exceeds a reasonable size.
	if len(g.budgets) > budgetGCThreshold {
		g.gcStaleBudgets(cutoff, chatID)
	}

	if len(b.timestamps) >= g.cfg.BudgetMax {
		return false
	}
	b.timestamps = append(b.timestamps, now)
	return true
}

// budgetGCThreshold is the number of chatID entries before GC runs.
const budgetGCThreshold = 50

// gcStaleBudgets removes budget entries whose most recent timestamp is
// older than cutoff. skipID is excluded from eviction (the caller's
// current chat, whose new timestamp hasn't been appended yet).
// Caller must hold g.mu.
func (g *AttentionGate) gcStaleBudgets(cutoff time.Time, skipID string) {
	for id, b := range g.budgets {
		if id == skipID {
			continue
		}
		if len(b.timestamps) == 0 {
			delete(g.budgets, id)
			continue
		}
		// timestamps are appended in order; last entry is the most recent.
		if !b.timestamps[len(b.timestamps)-1].After(cutoff) {
			delete(g.budgets, id)
		}
	}
}

// IsEnabled returns whether the attention gate is active.
func (g *AttentionGate) IsEnabled() bool {
	return g.cfg.Enabled
}

// AutoAckMessage returns the configured auto-acknowledgement message.
func (g *AttentionGate) AutoAckMessage() string {
	return g.cfg.AutoAckMessage
}

// SetFocusTimeChecker attaches a FocusTimeChecker to the gate.
// When set, ShouldDispatch will suppress non-urgent messages for users
// currently in focus time.
func (g *AttentionGate) SetFocusTimeChecker(ftc FocusTimeChecker) {
	g.focusTime = ftc
}

// ShouldDispatch decides whether a message should be dispatched to a user.
// It combines urgency classification, focus time suppression, and budget
// enforcement. Critical/P0 (UrgencyHigh) messages always pass through.
// Returns the urgency level and whether the message should be sent.
func (g *AttentionGate) ShouldDispatch(content, chatID, userID string, now time.Time) (UrgencyLevel, bool) {
	urgency := g.ClassifyUrgency(content)

	// Critical messages always pass through.
	if urgency == UrgencyHigh {
		return urgency, true
	}

	// Gate disabled → pass through.
	if !g.cfg.Enabled {
		return urgency, true
	}

	// Check focus time suppression for non-urgent messages.
	if g.focusTime != nil && g.focusTime.ShouldSuppress(userID, now) {
		return urgency, false
	}

	// Check budget.
	if !g.RecordDispatch(chatID, now) {
		return urgency, false
	}

	return urgency, true
}
