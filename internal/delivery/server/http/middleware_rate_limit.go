package http

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
	EntryTTL          time.Duration
	CleanupInterval   time.Duration
}

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiter struct {
	mu              sync.Mutex
	limit           rate.Limit
	burst           int
	entries         map[string]*rateLimitEntry
	entryTTL        time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

func newRateLimiter(cfg RateLimitConfig) *rateLimiter {
	ttl := cfg.EntryTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	cleanup := cfg.CleanupInterval
	if cleanup <= 0 {
		cleanup = 5 * time.Minute
	}
	return &rateLimiter{
		limit:           rate.Every(time.Minute / time.Duration(cfg.RequestsPerMinute)),
		burst:           cfg.Burst,
		entries:         make(map[string]*rateLimitEntry),
		entryTTL:        ttl,
		cleanupInterval: cleanup,
		lastCleanup:     time.Now(),
	}
}

func (r *rateLimiter) allow(key string) bool {
	if r == nil || key == "" {
		return true
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cleanupInterval > 0 && now.Sub(r.lastCleanup) >= r.cleanupInterval {
		for k, entry := range r.entries {
			if entry == nil || now.Sub(entry.lastSeen) > r.entryTTL {
				delete(r.entries, k)
			}
		}
		r.lastCleanup = now
	}

	entry, ok := r.entries[key]
	if !ok {
		entry = &rateLimitEntry{
			limiter:  rate.NewLimiter(r.limit, r.burst),
			lastSeen: now,
		}
		r.entries[key] = entry
	} else {
		entry.lastSeen = now
	}

	return entry.limiter.Allow()
}

func RateLimitMiddleware(cfg RateLimitConfig) func(http.Handler) http.Handler {
	if cfg.RequestsPerMinute <= 0 || cfg.Burst <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	limiter := newRateLimiter(cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)
			if !limiter.allow(key) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if user, ok := CurrentUser(r.Context()); ok {
		if id := strings.TrimSpace(user.ID); id != "" {
			return "user:" + id
		}
	}
	if ip := clientIP(r); ip != "" {
		return "ip:" + ip
	}
	return "anonymous"
}
