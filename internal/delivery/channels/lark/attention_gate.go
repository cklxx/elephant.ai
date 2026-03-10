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

// AttentionGate filters messages based on urgency criteria and enforces
// a per-chat notification budget.
type AttentionGate struct {
	cfg AttentionGateConfig

	// lowerKeywords is the pre-lowered keyword set for fast matching.
	lowerKeywords []string

	mu      sync.Mutex
	budgets map[string]*chatBudget // chatID → budget tracker
}

type chatBudget struct {
	timestamps []time.Time
}

// NewAttentionGate creates an AttentionGate with the given config.
func NewAttentionGate(cfg AttentionGateConfig) *AttentionGate {
	lower := make([]string, len(cfg.UrgentKeywords))
	for i, kw := range cfg.UrgentKeywords {
		lower[i] = strings.ToLower(kw)
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

	// Trim expired entries.
	cutoff := now.Add(-g.cfg.BudgetWindow)
	trimmed := b.timestamps[:0]
	for _, ts := range b.timestamps {
		if ts.After(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	b.timestamps = trimmed

	if len(b.timestamps) >= g.cfg.BudgetMax {
		return false
	}
	b.timestamps = append(b.timestamps, now)
	return true
}

// IsEnabled returns whether the attention gate is active.
func (g *AttentionGate) IsEnabled() bool {
	return g.cfg.Enabled
}

// AutoAckMessage returns the configured auto-acknowledgement message.
func (g *AttentionGate) AutoAckMessage() string {
	return g.cfg.AutoAckMessage
}
