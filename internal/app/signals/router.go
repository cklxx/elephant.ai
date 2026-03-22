package signals

import (
	"context"
	"sync"
	"time"
)

// FocusTimeChecker determines whether a user is currently in focus time.
type FocusTimeChecker interface {
	ShouldSuppress(userID string, now time.Time) bool
}

// QueuedSignal is a signal held back during quiet hours.
type QueuedSignal struct {
	Event    SignalEvent
	QueuedAt time.Time
}

// Router applies policy rules and routes scored signals.
type Router struct {
	thresholds routingThresholds
	budgets    map[string]*chatBudget
	budgetMax  int
	window     time.Duration
	quietStart int
	quietEnd   int
	focusCheck FocusTimeChecker
	queue      []QueuedSignal
	nowFn      func() time.Time
	mu         sync.Mutex
}

type routingThresholds struct {
	summarize int
	queue     int
	notifyNow int
	escalate  int
}

type chatBudget struct {
	timestamps []time.Time
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithFocusChecker attaches a FocusTimeChecker.
func WithFocusChecker(fc FocusTimeChecker) RouterOption {
	return func(r *Router) { r.focusCheck = fc }
}

// NewRouter creates a Router from the given Config.
func NewRouter(cfg Config, nowFn func() time.Time, opts ...RouterOption) *Router {
	if nowFn == nil {
		nowFn = time.Now
	}
	r := &Router{
		thresholds: buildThresholds(cfg),
		budgets:    make(map[string]*chatBudget),
		budgetMax:  cfg.BudgetMax,
		window:     cfg.BudgetWindow,
		quietStart: cfg.QuietHoursStart,
		quietEnd:   cfg.QuietHoursEnd,
		nowFn:      nowFn,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Route applies policy and returns the final routing decision.
func (r *Router) Route(_ context.Context, event *SignalEvent) AttentionRoute {
	route := r.routeForScore(event.Score)
	now := r.nowFn()

	// Critical alerts must never be budget-suppressed.
	if route == RouteEscalate {
		return RouteEscalate
	}
	if route == RouteNotifyNow {
		return r.applyBudget(event.ChatID, now, route)
	}
	if r.inQuietHours(now.Hour()) {
		r.enqueue(*event, now)
		return RouteQueue
	}
	if r.shouldSuppressFocus(event.UserID, now) {
		return RouteSuppress
	}
	return r.applyBudget(event.ChatID, now, route)
}

// DrainQueue returns queued signals and clears the queue.
func (r *Router) DrainQueue() []QueuedSignal {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queue) == 0 {
		return nil
	}
	out := r.queue
	r.queue = nil
	return out
}

func (r *Router) routeForScore(score int) AttentionRoute {
	switch {
	case score >= r.thresholds.escalate:
		return RouteEscalate
	case score >= r.thresholds.notifyNow:
		return RouteNotifyNow
	case score >= r.thresholds.queue:
		return RouteQueue
	case score >= r.thresholds.summarize:
		return RouteSummarize
	default:
		return RouteSuppress
	}
}

func (r *Router) inQuietHours(hour int) bool {
	if r.quietStart == r.quietEnd {
		return false
	}
	if r.quietStart < r.quietEnd {
		return hour >= r.quietStart && hour < r.quietEnd
	}
	return hour >= r.quietStart || hour < r.quietEnd
}

func (r *Router) enqueue(event SignalEvent, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue = append(r.queue, QueuedSignal{Event: event, QueuedAt: now})
}

func (r *Router) shouldSuppressFocus(userID string, now time.Time) bool {
	r.mu.Lock()
	fc := r.focusCheck
	r.mu.Unlock()
	return fc != nil && fc.ShouldSuppress(userID, now)
}

func (r *Router) applyBudget(chatID string, now time.Time, route AttentionRoute) AttentionRoute {
	if r.budgetMax <= 0 {
		return route
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	b := r.budgets[chatID]
	if b == nil {
		b = &chatBudget{}
		r.budgets[chatID] = b
	}
	cutoff := now.Add(-r.window)
	trimmed := b.timestamps[:0]
	for _, ts := range b.timestamps {
		if ts.After(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	b.timestamps = trimmed

	if len(b.timestamps) >= r.budgetMax {
		return RouteSuppress
	}
	b.timestamps = append(b.timestamps, now)

	// Periodic eviction: if the budgets map grows beyond 1000 entries,
	// sweep entries that haven't been updated in the last hour.
	if len(r.budgets) > 1000 {
		evictCutoff := now.Add(-time.Hour)
		for id, cb := range r.budgets {
			if len(cb.timestamps) == 0 || cb.timestamps[len(cb.timestamps)-1].Before(evictCutoff) {
				delete(r.budgets, id)
			}
		}
	}

	return route
}

func buildThresholds(cfg Config) routingThresholds {
	return routingThresholds{
		summarize: withDefault(cfg.SummarizeThreshold, 40),
		queue:     withDefault(cfg.QueueThreshold, 60),
		notifyNow: withDefault(cfg.NotifyNowThreshold, 80),
		escalate:  withDefault(cfg.EscalateThreshold, 90),
	}
}

func withDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
