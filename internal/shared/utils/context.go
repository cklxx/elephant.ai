package utils

import (
	"context"
	"time"
)

// WithFreshDeadline returns a context with a new deadline independent of the
// parent's deadline (so an exhausted parent budget doesn't kill the child),
// while still propagating explicit cancellation (e.g. user ctrl+C).
//
// Specifically:
//   - If the parent is cancelled with context.Canceled, the child is cancelled too.
//   - If the parent's deadline expires (context.DeadlineExceeded), the child
//     continues running under its own timeout.
func WithFreshDeadline(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	stop := context.AfterFunc(parent, func() {
		if parent.Err() == context.Canceled {
			cancel() // propagate explicit cancellation only
		}
		// ignore DeadlineExceeded — the child has its own timeout
	})
	combined := func() {
		stop()
		cancel()
	}
	return ctx, combined
}
