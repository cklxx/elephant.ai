package calendar

import (
	"context"
	"time"
)

// CalendarPort provides access to calendar events for proactive scheduling.
type CalendarPort interface {
	// ListUpcoming1on1s returns 1:1 meetings starting within the given window
	// for the specified member. Implementations should filter to meetings where
	// the member is a participant and Is1on1 is true.
	ListUpcoming1on1s(ctx context.Context, memberID string, window time.Duration) ([]Meeting, error)
}
