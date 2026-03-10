package lark

import (
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	if cfg.ChatHourlyLimit != 10 {
		t.Errorf("ChatHourlyLimit = %d, want 10", cfg.ChatHourlyLimit)
	}
	if cfg.UserDailyLimit != 50 {
		t.Errorf("UserDailyLimit = %d, want 50", cfg.UserDailyLimit)
	}
}

func TestRateLimiter_Allow_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 5,
		UserDailyLimit:  20,
	})

	allowed, reason := rl.Allow("chat-1", "user-1", PriorityNormal)
	if !allowed {
		t.Errorf("should allow under limit, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("reason should be empty, got %q", reason)
	}

	// Record a few and check still under limit.
	for i := 0; i < 4; i++ {
		rl.Record("chat-1", "user-1")
	}
	allowed, _ = rl.Allow("chat-1", "user-1", PriorityNormal)
	if !allowed {
		t.Error("should still allow at 4/5 chat limit")
	}
}

func TestRateLimiter_Allow_ChatHourlyExceeded(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 3,
		UserDailyLimit:  50,
	})

	for i := 0; i < 3; i++ {
		rl.Record("chat-1", "user-1")
	}

	allowed, reason := rl.Allow("chat-1", "user-1", PriorityNormal)
	if allowed {
		t.Error("should block after 3 chat messages in the hour")
	}
	if reason == "" {
		t.Error("reason should explain the limit")
	}

	// A different chat should still be allowed.
	allowed, _ = rl.Allow("chat-2", "user-1", PriorityNormal)
	if !allowed {
		t.Error("different chat should still be allowed")
	}
}

func TestRateLimiter_Allow_UserDailyExceeded(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 100, // high so we hit user limit first
		UserDailyLimit:  5,
	})

	for i := 0; i < 5; i++ {
		chatID := "chat-" + string(rune('a'+i))
		rl.Record(chatID, "user-1")
	}

	allowed, reason := rl.Allow("chat-x", "user-1", PriorityNormal)
	if allowed {
		t.Error("should block after 5 user messages in the day")
	}
	if reason == "" {
		t.Error("reason should explain the limit")
	}

	// A different user should still be allowed.
	allowed, _ = rl.Allow("chat-x", "user-2", PriorityNormal)
	if !allowed {
		t.Error("different user should still be allowed")
	}
}

func TestRateLimiter_Allow_CriticalBypassesChatLimit(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 2,
		UserDailyLimit:  50,
	})

	// Fill up the chat limit.
	rl.Record("chat-1", "user-1")
	rl.Record("chat-1", "user-1")

	// Normal should be blocked.
	allowed, _ := rl.Allow("chat-1", "user-1", PriorityNormal)
	if allowed {
		t.Error("normal priority should be blocked when chat limit exceeded")
	}

	// Critical should bypass chat limit.
	allowed, reason := rl.Allow("chat-1", "user-1", PriorityCritical)
	if !allowed {
		t.Errorf("critical should bypass chat limit, reason: %s", reason)
	}
}

func TestRateLimiter_Allow_CriticalRespectsUserDaily(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 2,
		UserDailyLimit:  3,
	})

	// Fill up the user daily limit.
	rl.Record("chat-1", "user-1")
	rl.Record("chat-2", "user-1")
	rl.Record("chat-3", "user-1")

	// Critical should still be blocked by user daily limit.
	allowed, reason := rl.Allow("chat-4", "user-1", PriorityCritical)
	if allowed {
		t.Error("critical should still respect user daily limit")
	}
	if reason == "" {
		t.Error("reason should explain daily limit")
	}
}

func TestRateLimiter_SlidingWindow_ExpiresOldEntries(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 2,
		UserDailyLimit:  50,
	})
	rl.nowFunc = func() time.Time { return now }

	// Fill the chat hourly limit.
	rl.Record("chat-1", "user-1")
	rl.Record("chat-1", "user-1")

	// Should be blocked now.
	allowed, _ := rl.Allow("chat-1", "user-1", PriorityNormal)
	if allowed {
		t.Error("should be blocked at limit")
	}

	// Advance time past the 1-hour window.
	now = now.Add(61 * time.Minute)

	// Old entries should have expired; should be allowed again.
	allowed, reason := rl.Allow("chat-1", "user-1", PriorityNormal)
	if !allowed {
		t.Errorf("should allow after window expires, reason: %s", reason)
	}
}

func TestRateLimiter_Enqueue_And_FlushDigest(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	rl.Enqueue("chat-1", "alert: CPU high")
	rl.Enqueue("chat-1", "alert: disk full")
	rl.Enqueue("chat-1", "alert: memory low")

	digest := rl.FlushDigest("chat-1")
	expected := "3 notifications batched:\n- alert: CPU high\n- alert: disk full\n- alert: memory low\n"
	if digest != expected {
		t.Errorf("digest =\n%q\nwant:\n%q", digest, expected)
	}

	// After flush, should be empty.
	if rl.FlushDigest("chat-1") != "" {
		t.Error("second flush should return empty string")
	}
}

func TestRateLimiter_FlushDigest_Empty(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	if got := rl.FlushDigest("nonexistent"); got != "" {
		t.Errorf("flush of empty chat should return empty string, got %q", got)
	}
}

func TestRateLimiter_PendingCount(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	if rl.PendingCount("chat-1") != 0 {
		t.Error("initial pending count should be 0")
	}

	rl.Enqueue("chat-1", "msg1")
	rl.Enqueue("chat-1", "msg2")
	rl.Enqueue("chat-2", "msg3")

	if got := rl.PendingCount("chat-1"); got != 2 {
		t.Errorf("PendingCount(chat-1) = %d, want 2", got)
	}
	if got := rl.PendingCount("chat-2"); got != 1 {
		t.Errorf("PendingCount(chat-2) = %d, want 1", got)
	}

	rl.FlushDigest("chat-1")
	if got := rl.PendingCount("chat-1"); got != 0 {
		t.Errorf("PendingCount after flush = %d, want 0", got)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		ChatHourlyLimit: 100,
		UserDailyLimit:  1000,
	})

	const goroutines = 20
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			chatID := "chat-concurrent"
			userID := "user-concurrent"

			for i := 0; i < opsPerGoroutine; i++ {
				rl.Allow(chatID, userID, PriorityNormal)
				rl.Record(chatID, userID)
				rl.Enqueue(chatID, "msg")
				rl.PendingCount(chatID)
			}
		}(g)
	}

	wg.Wait()

	// Verify no panic occurred and state is consistent.
	count := rl.PendingCount("chat-concurrent")
	expected := goroutines * opsPerGoroutine
	if count != expected {
		t.Errorf("PendingCount = %d, want %d", count, expected)
	}

	digest := rl.FlushDigest("chat-concurrent")
	if digest == "" {
		t.Error("digest should not be empty after concurrent enqueues")
	}
}
