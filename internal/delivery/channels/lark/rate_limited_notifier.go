package lark

import (
	"context"

	"alex/internal/shared/logging"
	"alex/internal/shared/notification"
)

// RateLimitedNotifier wraps a notification.Notifier with the Lark rate limiter.
// Non-Lark messages pass through unmodified. Rate-limited Lark messages are
// enqueued for later digest delivery.
type RateLimitedNotifier struct {
	inner   notification.Notifier
	limiter *RateLimiter
	logger  logging.Logger
}

// NewRateLimitedNotifier creates a notifier decorator that enforces rate limits
// on Lark notifications. Non-Lark channels are forwarded directly.
func NewRateLimitedNotifier(inner notification.Notifier, limiter *RateLimiter, logger logging.Logger) *RateLimitedNotifier {
	return &RateLimitedNotifier{
		inner:   inner,
		limiter: limiter,
		logger:  logging.OrNop(logger),
	}
}

// Send checks the rate limiter before forwarding to the inner notifier.
// Rate-limited messages are enqueued for digest batching rather than dropped.
func (n *RateLimitedNotifier) Send(ctx context.Context, target notification.Target, content string) error {
	// Non-Lark messages bypass rate limiting entirely.
	if target.Channel != notification.ChannelLark {
		return n.inner.Send(ctx, target, content)
	}

	chatID := target.ChatID
	if chatID == "" {
		return n.inner.Send(ctx, target, content)
	}

	// Use chatID as userID fallback — leader notifications are per-chat, not
	// per-user, so chatID gives a reasonable throttle boundary.
	userID := chatID

	allowed, reason := n.limiter.Allow(chatID, userID, PriorityNormal)
	if !allowed {
		n.logger.Info("Rate limited Lark notification (chat=%s): %s", chatID, reason)
		n.limiter.Enqueue(chatID, content)
		return nil
	}

	err := n.inner.Send(ctx, target, content)
	if err == nil {
		n.limiter.Record(chatID, userID)
	}
	return err
}

// Limiter returns the underlying rate limiter for digest flushing or inspection.
func (n *RateLimitedNotifier) Limiter() *RateLimiter {
	return n.limiter
}
