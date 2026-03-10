package lark

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// AttentionRoute describes how a message should be handled after scoring.
type AttentionRoute string

const (
	// AttentionRouteSuppress drops low-signal messages from immediate routing.
	AttentionRouteSuppress AttentionRoute = "suppress"
	// AttentionRouteSummarize batches medium-signal messages into summaries.
	AttentionRouteSummarize AttentionRoute = "summarize"
	// AttentionRouteQueue defers high-signal but non-urgent messages.
	AttentionRouteQueue AttentionRoute = "queue"
	// AttentionRouteNotifyNow dispatches urgent messages immediately.
	AttentionRouteNotifyNow AttentionRoute = "notify_now"
	// AttentionRouteEscalate dispatches critical messages immediately and flags them for escalation.
	AttentionRouteEscalate AttentionRoute = "escalate"
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
	// Matched case-insensitively against message content and scored at 80+.
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
	// QuietHoursStart is the hour (0-23) when quiet hours begin.
	// During quiet hours only UrgencyHigh messages pass through;
	// all others are queued until quiet hours end.
	// Set QuietHoursStart == QuietHoursEnd to disable quiet hours.
	QuietHoursStart int `yaml:"quiet_hours_start"`
	// QuietHoursEnd is the hour (0-23) when quiet hours end (exclusive).
	// Wraps around midnight: start=22, end=8 means 22:00-07:59.
	QuietHoursEnd int `yaml:"quiet_hours_end"`
	// SummarizeThreshold is the minimum score routed to summarize.
	// Default: 40.
	SummarizeThreshold int `yaml:"summarize_threshold"`
	// QueueThreshold is the minimum score routed to queue.
	// Default: 60.
	QueueThreshold int `yaml:"queue_threshold"`
	// NotifyNowThreshold is the minimum score routed to notify_now.
	// Default: 80.
	NotifyNowThreshold int `yaml:"notify_now_threshold"`
	// EscalateThreshold is the minimum score routed to escalate.
	// Default: 90.
	EscalateThreshold int `yaml:"escalate_threshold"`
}

const defaultAutoAckMessage = "收到，已记录并跟踪中。"

const (
	minAttentionScore = 0
	maxAttentionScore = 100

	baseAttentionScore      = 20
	summarizeAttentionScore = 40
	queueAttentionScore     = 60
	notifyNowAttentionScore = 80
	escalateAttentionScore  = 90

	defaultSummarizeThreshold = 40
	defaultQueueThreshold     = 60
	defaultNotifyNowThreshold = 80
	defaultEscalateThreshold  = 90
)

var summarizeAttentionPatterns = []string{
	"please", "pls", "review", "check", "look at", "take a look",
	"can you", "could you", "help", "请", "帮忙", "看一下", "看看", "麻烦",
}

var queueAttentionPatterns = []string{
	"today", "tonight", "this week", "before ", "follow up", "deadline",
	"eod", "尽快", "今天", "今晚", "本周", "截止", "稍后",
}

var builtinUrgencyPatterns = []string{
	"紧急", "urgent", "asap", "deadline",
	"立刻", "马上", "immediately",
	"出错", "报错", "error", "失败", "failed", "故障",
	"挂了", "崩了", "down", "宕机",
	"blocked", "阻塞",
}

var escalateAttentionPatterns = []string{
	"p0", "sev0", "sev1", "incident", "outage",
	"生产事故", "生产故障", "宕机", "崩了",
}

// FocusTimeChecker determines whether a user is in a focus time window.
// When set on AttentionGate, non-urgent messages are suppressed for users
// currently in focus time.
type FocusTimeChecker interface {
	ShouldSuppress(userID string, now time.Time) bool
}

// QueuedMessage is a non-urgent message held back during quiet hours.
type QueuedMessage struct {
	Content        string
	ChatID         string
	UserID         string
	AttentionScore int
	Route          AttentionRoute
	Urgency        UrgencyLevel
	QueuedAt       time.Time
}

// AttentionAssessment captures the numeric score and threshold-derived route.
type AttentionAssessment struct {
	Score int
	Route AttentionRoute
}

// AttentionGate filters messages based on urgency criteria and enforces
// a per-chat notification budget.
type AttentionGate struct {
	cfg AttentionGateConfig

	// lowerKeywords is the pre-lowered keyword set for fast matching.
	lowerKeywords []string
	thresholds    attentionRoutingThresholds

	// focusTime is an optional checker for focus time suppression.
	// When non-nil, non-urgent messages are suppressed during focus time.
	focusTime FocusTimeChecker

	// nowFn overrides time.Now for testing. Nil means use time.Now.
	nowFn func() time.Time

	mu      sync.Mutex
	budgets map[string]*chatBudget // chatID → budget tracker
	queued  []QueuedMessage        // messages held during quiet hours

	// drainInterval overrides the default 1-minute drain check interval (for tests).
	drainInterval time.Duration

	// drainCancel stops the drain timer goroutine; drainWG tracks its exit.
	drainCancel context.CancelFunc
	drainWG     sync.WaitGroup
}

type chatBudget struct {
	timestamps []time.Time
}

type attentionRoutingThresholds struct {
	summarize int
	queue     int
	notifyNow int
	escalate  int
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
		thresholds:    normalizeAttentionRoutingThresholds(cfg),
		budgets:       make(map[string]*chatBudget),
	}
}

// Assess returns the numeric attention score and route for a message.
func (g *AttentionGate) Assess(content string) AttentionAssessment {
	score := g.AttentionScore(content)
	return AttentionAssessment{
		Score: score,
		Route: g.RouteForScore(score),
	}
}

// AttentionScore determines the 0-100 attention score for a message.
// The score is compatibility-oriented: messages that were previously
// "urgent" always score at or above the notify_now threshold.
func (g *AttentionGate) AttentionScore(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return minAttentionScore
	}

	lower := strings.ToLower(trimmed)
	score := baseAttentionScore

	if containsAny(lower, summarizeAttentionPatterns) {
		score = maxInt(score, summarizeAttentionScore)
	}
	if containsAny(lower, queueAttentionPatterns) {
		score = maxInt(score, queueAttentionScore)
	}

	urgentSignals := countMatches(lower, g.lowerKeywords)
	if urgentSignals > 0 {
		score = maxInt(score, notifyNowAttentionScore)
	}

	builtinSignals := countMatches(lower, builtinUrgencyPatterns)
	if builtinSignals > 0 {
		score = maxInt(score, notifyNowAttentionScore)
		urgentSignals += builtinSignals
	}

	if hasExclamationBurst(lower) {
		score = maxInt(score, notifyNowAttentionScore)
		urgentSignals++
	}

	escalateSignals := countMatches(lower, escalateAttentionPatterns)
	if escalateSignals > 0 || urgentSignals >= 2 {
		score = maxInt(score, escalateAttentionScore)
	}
	if escalateSignals+urgentSignals >= 3 {
		score = maxAttentionScore
	}

	return clampAttentionScore(score)
}

// RouteForScore converts a numeric score into a routing outcome.
func (g *AttentionGate) RouteForScore(score int) AttentionRoute {
	score = clampAttentionScore(score)
	switch {
	case score >= g.thresholds.escalate:
		return AttentionRouteEscalate
	case score >= g.thresholds.notifyNow:
		return AttentionRouteNotifyNow
	case score >= g.thresholds.queue:
		return AttentionRouteQueue
	case score >= g.thresholds.summarize:
		return AttentionRouteSummarize
	default:
		return AttentionRouteSuppress
	}
}

// ClassifyUrgency maps numeric attention scoring back to the legacy urgency
// API for callers that still depend on UrgencyLevel.
func (g *AttentionGate) ClassifyUrgency(content string) UrgencyLevel {
	if !g.cfg.Enabled {
		return UrgencyNormal
	}

	return g.legacyUrgencyForScore(g.AttentionScore(content))
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

// inQuietHours returns true if the given hour falls within the configured
// quiet hours window. Returns false when quiet hours are disabled
// (start == end).
func (g *AttentionGate) inQuietHours(hour int) bool {
	start := g.cfg.QuietHoursStart
	end := g.cfg.QuietHoursEnd
	if start == end {
		return false // disabled
	}
	if start < end {
		return hour >= start && hour < end
	}
	// Wraps midnight: e.g. 22-8 means hours 22,23,0,1,...,7.
	return hour >= start || hour < end
}

// ShouldDispatch decides whether a message should be dispatched to a user.
// It combines urgency classification, quiet hours enforcement, focus time
// suppression, and budget enforcement. Critical/P0 (UrgencyHigh) messages
// always pass through, even during quiet hours.
// Returns the urgency level and whether the message should be sent.
func (g *AttentionGate) ShouldDispatch(content, chatID, userID string, now time.Time) (UrgencyLevel, bool) {
	if !g.cfg.Enabled {
		return UrgencyNormal, true
	}

	assessment := g.Assess(content)
	urgency := g.legacyUrgencyForScore(assessment.Score)

	// Critical messages always pass through.
	if urgency == UrgencyHigh {
		return urgency, true
	}

	// Quiet hours enforcement: queue non-urgent messages.
	if g.inQuietHours(now.Hour()) {
		g.mu.Lock()
		g.queued = append(g.queued, QueuedMessage{
			Content:        content,
			ChatID:         chatID,
			UserID:         userID,
			AttentionScore: assessment.Score,
			Route:          assessment.Route,
			Urgency:        urgency,
			QueuedAt:       now,
		})
		g.mu.Unlock()
		return urgency, false
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

// DrainQueue returns all messages queued during quiet hours and clears
// the queue. Callers should invoke this when quiet hours end (e.g. at
// the first tick after QuietHoursEnd) to dispatch the held messages.
// Returns nil if the queue is empty.
func (g *AttentionGate) DrainQueue() []QueuedMessage {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.queued) == 0 {
		return nil
	}
	out := g.queued
	g.queued = nil
	return out
}

// QueueLen returns the number of messages currently held in the quiet
// hours queue.
func (g *AttentionGate) QueueLen() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.queued)
}

func normalizeAttentionRoutingThresholds(cfg AttentionGateConfig) attentionRoutingThresholds {
	thresholds := attentionRoutingThresholds{
		summarize: cfg.SummarizeThreshold,
		queue:     cfg.QueueThreshold,
		notifyNow: cfg.NotifyNowThreshold,
		escalate:  cfg.EscalateThreshold,
	}
	if thresholds.summarize == 0 {
		thresholds.summarize = defaultSummarizeThreshold
	}
	if thresholds.queue == 0 {
		thresholds.queue = defaultQueueThreshold
	}
	if thresholds.notifyNow == 0 {
		thresholds.notifyNow = defaultNotifyNowThreshold
	}
	if thresholds.escalate == 0 {
		thresholds.escalate = defaultEscalateThreshold
	}
	return thresholds
}

func (g *AttentionGate) legacyUrgencyForScore(score int) UrgencyLevel {
	if score >= notifyNowAttentionScore {
		return UrgencyHigh
	}
	return UrgencyLow
}

func containsAny(lower string, patterns []string) bool {
	return countMatches(lower, patterns) > 0
}

func countMatches(lower string, patterns []string) int {
	count := 0
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			count++
		}
	}
	return count
}

func hasExclamationBurst(lower string) bool {
	exclamations := strings.Count(lower, "!") + strings.Count(lower, "！")
	return exclamations >= 3 && utf8.RuneCountInString(lower) < 50
}

func clampAttentionScore(score int) int {
	if score < minAttentionScore {
		return minAttentionScore
	}
	if score > maxAttentionScore {
		return maxAttentionScore
	}
	return score
}

func maxInt(left, right int) int {
	if right > left {
		return right
	}
	return left
}

// defaultDrainInterval is how often the drain timer checks whether quiet
// hours have ended.
const defaultDrainInterval = 1 * time.Minute

// DrainCallback is called by the drain timer when quiet hours end and
// there are queued messages to deliver.
type DrainCallback func(msgs []QueuedMessage)

// StartDrainTimer launches a background goroutine that checks every
// tick whether quiet hours have ended. When the transition from quiet
// to non-quiet is detected and the queue is non-empty, it drains the
// queue and invokes cb with the held messages.
// It is safe to call multiple times; subsequent calls are no-ops until
// StopDrainTimer is called.
func (g *AttentionGate) StartDrainTimer(ctx context.Context, cb DrainCallback) {
	if g.drainCancel != nil {
		return // already running
	}
	if g.cfg.QuietHoursStart == g.cfg.QuietHoursEnd {
		return // quiet hours disabled, nothing to drain
	}

	drainCtx, cancel := context.WithCancel(ctx)
	g.drainCancel = cancel

	g.drainWG.Add(1)
	go g.drainLoop(drainCtx, cb)
}

// StopDrainTimer stops the drain timer goroutine and blocks until it exits.
func (g *AttentionGate) StopDrainTimer() {
	if g.drainCancel != nil {
		g.drainCancel()
	}
	g.drainWG.Wait()
	g.drainCancel = nil
}

// drainLoop is the background ticker that watches for quiet-hours transitions.
func (g *AttentionGate) drainLoop(ctx context.Context, cb DrainCallback) {
	defer g.drainWG.Done()

	interval := g.drainInterval
	if interval <= 0 {
		interval = defaultDrainInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	wasQuiet := g.inQuietHours(g.now().Hour())

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isQuiet := g.inQuietHours(g.now().Hour())
			// Detect transition: was quiet → no longer quiet.
			if wasQuiet && !isQuiet {
				if msgs := g.DrainQueue(); len(msgs) > 0 {
					cb(msgs)
				}
			}
			wasQuiet = isQuiet
		}
	}
}

// now returns the current time, using nowFn if set (for testing).
func (g *AttentionGate) now() time.Time {
	if g.nowFn != nil {
		return g.nowFn()
	}
	return time.Now()
}
