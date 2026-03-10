package lark

import (
	"context"
	"errors"
	"testing"

	"alex/internal/shared/notification"
)

// spyNotifier records calls to Send and can return a configured error.
type spyNotifier struct {
	calls []spySend
	err   error
}

type spySend struct {
	target  notification.Target
	content string
}

func (s *spyNotifier) Send(_ context.Context, target notification.Target, content string) error {
	s.calls = append(s.calls, spySend{target: target, content: content})
	return s.err
}

func TestRateLimitedNotifier_NonLarkPassthrough(t *testing.T) {
	inner := &spyNotifier{}
	limiter := NewRateLimiter(DefaultRateLimiterConfig())
	n := NewRateLimitedNotifier(inner, limiter, nil)

	target := notification.Target{Channel: notification.ChannelMoltbook, ChatID: "chat1"}
	if err := n.Send(context.Background(), target, "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 passthrough call, got %d", len(inner.calls))
	}
	if inner.calls[0].content != "hello" {
		t.Errorf("expected content 'hello', got %q", inner.calls[0].content)
	}
}

func TestRateLimitedNotifier_EmptyChatIDPassthrough(t *testing.T) {
	inner := &spyNotifier{}
	limiter := NewRateLimiter(DefaultRateLimiterConfig())
	n := NewRateLimitedNotifier(inner, limiter, nil)

	target := notification.Target{Channel: notification.ChannelLark, ChatID: ""}
	if err := n.Send(context.Background(), target, "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 passthrough call, got %d", len(inner.calls))
	}
}

func TestRateLimitedNotifier_AllowedForwardsAndRecords(t *testing.T) {
	inner := &spyNotifier{}
	limiter := NewRateLimiter(DefaultRateLimiterConfig())
	n := NewRateLimitedNotifier(inner, limiter, nil)

	target := notification.Target{Channel: notification.ChannelLark, ChatID: "chat1"}
	if err := n.Send(context.Background(), target, "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(inner.calls))
	}

	// Verify the limiter recorded the send (chat window should have 1 entry).
	allowed, _ := limiter.Allow("chat1", "chat1", PriorityNormal)
	if !allowed {
		t.Error("expected second send to still be allowed within limits")
	}
}

func TestRateLimitedNotifier_RateLimitedEnqueues(t *testing.T) {
	cfg := RateLimiterConfig{ChatHourlyLimit: 1, UserDailyLimit: 50}
	inner := &spyNotifier{}
	limiter := NewRateLimiter(cfg)
	n := NewRateLimitedNotifier(inner, limiter, nil)

	target := notification.Target{Channel: notification.ChannelLark, ChatID: "chat1"}

	// First send: allowed.
	if err := n.Send(context.Background(), target, "msg1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(inner.calls))
	}

	// Second send: should be rate limited and enqueued.
	if err := n.Send(context.Background(), target, "msg2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("expected still 1 call (second was rate limited), got %d", len(inner.calls))
	}

	// Verify the message was enqueued.
	if limiter.PendingCount("chat1") != 1 {
		t.Errorf("expected 1 pending message, got %d", limiter.PendingCount("chat1"))
	}

	digest := limiter.FlushDigest("chat1")
	if digest == "" {
		t.Error("expected non-empty digest")
	}
}

func TestRateLimitedNotifier_InnerErrorSkipsRecord(t *testing.T) {
	sendErr := errors.New("send failed")
	inner := &spyNotifier{err: sendErr}
	limiter := NewRateLimiter(RateLimiterConfig{ChatHourlyLimit: 2, UserDailyLimit: 50})
	n := NewRateLimitedNotifier(inner, limiter, nil)

	target := notification.Target{Channel: notification.ChannelLark, ChatID: "chat1"}

	// Send fails — limiter should NOT record it.
	err := n.Send(context.Background(), target, "msg1")
	if !errors.Is(err, sendErr) {
		t.Fatalf("expected send error, got %v", err)
	}

	// Since we didn't record, the next send should still be allowed
	// (chat window still empty).
	inner.err = nil
	if err := n.Send(context.Background(), target, "msg2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.calls) != 2 {
		t.Errorf("expected 2 calls (both allowed since first wasn't recorded), got %d", len(inner.calls))
	}
}

func TestRateLimitedNotifier_LimiterAccessor(t *testing.T) {
	limiter := NewRateLimiter(DefaultRateLimiterConfig())
	n := NewRateLimitedNotifier(&spyNotifier{}, limiter, nil)

	if n.Limiter() != limiter {
		t.Error("Limiter() should return the underlying rate limiter")
	}
}
