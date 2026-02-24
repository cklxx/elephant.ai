package lifecycle

import (
	"context"
	"fmt"
	"time"
)

// Drainable represents a subsystem that can be gracefully drained.
type Drainable interface {
	// Drain gracefully stops the subsystem.
	// The context carries a deadline; implementations should respect it.
	Drain(ctx context.Context) error
	// Name returns the subsystem name for logging.
	Name() string
}

// DrainAll drains multiple subsystems in order with a per-subsystem timeout.
func DrainAll(ctx context.Context, timeout time.Duration, subsystems ...Drainable) []error {
	var errs []error
	for _, s := range subsystems {
		subCtx, cancel := context.WithTimeout(ctx, timeout)
		if err := s.Drain(subCtx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
		}
		cancel()
	}
	return errs
}
