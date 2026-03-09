package hooks

import (
	"context"
	"time"
)

// StallScanner is the minimal interface the StallDetector needs from Runtime.
type StallScanner interface {
	ScanStalled(threshold time.Duration) []string
}

// StallDetector periodically scans for sessions that have had no heartbeat
// and publishes EventStalled for each one.
type StallDetector struct {
	rt        StallScanner
	bus       Bus
	threshold time.Duration
	interval  time.Duration
}

// NewStallDetector creates a StallDetector.
// threshold is the inactivity duration before a session is declared stalled.
// interval is how often the detector runs; defaults to threshold if ≤ 0.
func NewStallDetector(rt StallScanner, bus Bus, threshold, interval time.Duration) *StallDetector {
	if interval <= 0 {
		interval = threshold
	}
	return &StallDetector{rt: rt, bus: bus, threshold: threshold, interval: interval}
}

// Run starts the detection loop, blocking until ctx is cancelled.
func (d *StallDetector) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, id := range d.rt.ScanStalled(d.threshold) {
				d.bus.Publish(id, Event{
					Type:      EventStalled,
					SessionID: id,
					At:        time.Now(),
				})
			}
		}
	}
}
