package lark

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Priority classifies notification priority for rate limiting decisions.
type Priority int

const (
	// PriorityNormal is a standard notification subject to all rate limits.
	PriorityNormal Priority = iota
	// PriorityCritical bypasses per-chat hourly limits but still respects
	// the per-user daily limit.
	PriorityCritical
)

// RateLimiterConfig controls per-chat and per-user notification limits.
type RateLimiterConfig struct {
	// ChatHourlyLimit is the maximum notifications per hour per chat ID.
	// Default: 10.
	ChatHourlyLimit int `yaml:"chat_hourly_limit"`
	// UserDailyLimit is the maximum notifications per user per rolling 24h.
	// Default: 50.
	UserDailyLimit int `yaml:"user_daily_limit"`
}

// DefaultRateLimiterConfig returns the default rate limiter configuration.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		ChatHourlyLimit: 10,
		UserDailyLimit:  50,
	}
}

// slidingWindow tracks timestamps within a rolling time window.
type slidingWindow struct {
	timestamps []time.Time
	limit      int
	window     time.Duration
}

// count trims expired entries and returns the current count.
func (w *slidingWindow) count(now time.Time) int {
	w.trim(now)
	return len(w.timestamps)
}

// allowed returns true if the window has capacity.
func (w *slidingWindow) allowed(now time.Time) bool {
	return w.count(now) < w.limit
}

// record adds a timestamp to the window.
func (w *slidingWindow) record(now time.Time) {
	w.timestamps = append(w.timestamps, now)
}

// trim removes timestamps outside the window.
func (w *slidingWindow) trim(now time.Time) {
	cutoff := now.Add(-w.window)
	trimmed := w.timestamps[:0]
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	w.timestamps = trimmed
}

// RateLimiter enforces per-chat hourly and per-user daily notification limits
// with digest batching for rate-limited messages.
type RateLimiter struct {
	mu          sync.Mutex
	config      RateLimiterConfig
	chatWindows map[string]*slidingWindow // chatID -> hourly window
	userWindows map[string]*slidingWindow // userID -> daily window
	pending     map[string][]string       // chatID -> batched messages

	// nowFunc allows injecting a custom time source for testing.
	nowFunc func() time.Time
}

// NewRateLimiter creates a RateLimiter with the given config.
// Zero-value fields are replaced with defaults.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	defaults := DefaultRateLimiterConfig()
	if cfg.ChatHourlyLimit <= 0 {
		cfg.ChatHourlyLimit = defaults.ChatHourlyLimit
	}
	if cfg.UserDailyLimit <= 0 {
		cfg.UserDailyLimit = defaults.UserDailyLimit
	}
	return &RateLimiter{
		config:      cfg,
		chatWindows: make(map[string]*slidingWindow),
		userWindows: make(map[string]*slidingWindow),
		pending:     make(map[string][]string),
		nowFunc:     time.Now,
	}
}

// now returns the current time, using the injected nowFunc if set.
func (r *RateLimiter) now() time.Time {
	return r.nowFunc()
}

// Allow checks if a notification is allowed for the given chat+user+priority.
// Returns (allowed, reason). Critical priority bypasses the per-chat hourly
// limit but still respects the per-user daily limit.
func (r *RateLimiter) Allow(chatID, userID string, priority Priority) (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()

	// Always check user daily limit first — applies to all priorities.
	uw := r.userWindow(userID)
	if !uw.allowed(now) {
		return false, fmt.Sprintf("user %s exceeded daily limit of %d", userID, r.config.UserDailyLimit)
	}

	// Critical notifications bypass chat hourly limit.
	if priority == PriorityCritical {
		return true, ""
	}

	// Check per-chat hourly limit.
	cw := r.chatWindow(chatID)
	if !cw.allowed(now) {
		return false, fmt.Sprintf("chat %s exceeded hourly limit of %d", chatID, r.config.ChatHourlyLimit)
	}

	return true, ""
}

// Record records a sent notification against both the chat and user windows.
// Call this after a successful send.
func (r *RateLimiter) Record(chatID, userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.chatWindow(chatID).record(now)
	r.userWindow(userID).record(now)
}

// Enqueue adds a message to the pending digest for a rate-limited chat.
func (r *RateLimiter) Enqueue(chatID, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pending[chatID] = append(r.pending[chatID], message)
}

// FlushDigest returns and clears all pending messages for a chat as a
// formatted digest. Returns empty string if no pending messages.
func (r *RateLimiter) FlushDigest(chatID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	msgs := r.pending[chatID]
	if len(msgs) == 0 {
		return ""
	}
	delete(r.pending, chatID)

	var b strings.Builder
	fmt.Fprintf(&b, "%d notifications batched:\n", len(msgs))
	for _, m := range msgs {
		fmt.Fprintf(&b, "- %s\n", m)
	}
	return b.String()
}

// PendingCount returns the number of batched messages for a chat.
func (r *RateLimiter) PendingCount(chatID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.pending[chatID])
}

// chatWindow returns or creates the sliding window for a chat ID.
// Caller must hold r.mu.
func (r *RateLimiter) chatWindow(chatID string) *slidingWindow {
	w := r.chatWindows[chatID]
	if w == nil {
		w = &slidingWindow{
			limit:  r.config.ChatHourlyLimit,
			window: time.Hour,
		}
		r.chatWindows[chatID] = w
	}
	return w
}

// userWindow returns or creates the sliding window for a user ID.
// Caller must hold r.mu.
func (r *RateLimiter) userWindow(userID string) *slidingWindow {
	w := r.userWindows[userID]
	if w == nil {
		w = &slidingWindow{
			limit:  r.config.UserDailyLimit,
			window: 24 * time.Hour,
		}
		r.userWindows[userID] = w
	}
	return w
}
