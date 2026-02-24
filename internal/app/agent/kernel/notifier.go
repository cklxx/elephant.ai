package kernel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// CycleNotifier is called after each non-empty kernel cycle.
type CycleNotifier func(ctx context.Context, result *kerneldomain.CycleResult, err error)

func statusLabel(status kerneldomain.CycleResultStatus) string {
	switch status {
	case kerneldomain.CycleSuccess:
		return "success"
	case kerneldomain.CyclePartialSuccess:
		return "partial_success"
	case kerneldomain.CycleFailed:
		return "failed"
	default:
		return string(status)
	}
}

// ---------------------------------------------------------------------------
// CycleAggregator — buffers routine cycles, sends periodic summaries
// ---------------------------------------------------------------------------

// WindowSummary aggregates statistics for one notification window.
type WindowSummary struct {
	WindowStart      time.Time
	WindowEnd        time.Time
	TotalCycles      int
	TotalDispatched  int
	TotalSucceeded   int
	TotalFailed      int
	ImmediateAlerts  int      // alerts already sent during this window
	NoteworthyEvents []string // one-line-per-event (anomalies only)
	UniqueAgents     []string // deduplicated active agents
}

// windowStats tracks running counters during a window.
type windowStats struct {
	cycles          int
	dispatched      int
	succeeded       int
	failed          int
	immediateAlerts int
	noteworthy      []string
	agentSet        map[string]struct{}
}

// CycleAggregator buffers routine-success cycles and periodically flushes
// an aggregated summary. Anomalous cycles are forwarded immediately.
type CycleAggregator struct {
	kernelID string
	window   time.Duration
	sender   func(ctx context.Context, text string)
	now      func() time.Time // injectable clock for testing

	mu          sync.Mutex
	windowStart time.Time
	stats       windowStats
	flushTimer  *time.Timer
	closed      bool
}

// NewCycleAggregator creates a new aggregator.
// sender is the function that actually delivers a text notification.
func NewCycleAggregator(kernelID string, window time.Duration, sender func(ctx context.Context, text string)) *CycleAggregator {
	return &CycleAggregator{
		kernelID: kernelID,
		window:   window,
		sender:   sender,
		now:      time.Now,
	}
}

// HandleCycle is the CycleNotifier-compatible entry point.
// Anomalous cycles (error, failed, partial_success) are sent immediately.
// Routine successes are buffered until the window expires.
func (a *CycleAggregator) HandleCycle(ctx context.Context, result *kerneldomain.CycleResult, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}

	now := a.now()

	// Initialize window on first call.
	if a.windowStart.IsZero() {
		a.windowStart = now
		a.stats = newWindowStats()
		a.scheduleFlush(ctx)
	}

	// Classify: anomalous → immediate; routine → buffer.
	if isAnomalous(result, err) {
		a.stats.immediateAlerts++
		if result != nil {
			noteText := fmt.Sprintf("[%s] %s — %s", result.CycleID, statusLabel(result.Status), anomalyReason(result, err))
			a.stats.noteworthy = append(a.stats.noteworthy, noteText)
		}
		a.accumulateStats(result)
		text := FormatImmediateAlert(a.kernelID, result, err)
		a.sender(ctx, text)
		return
	}

	// Routine success — accumulate.
	a.accumulateStats(result)

	// Check if window has expired (belt-and-suspenders alongside timer).
	if now.Sub(a.windowStart) >= a.window {
		a.flushLocked(ctx, now)
	}
}

// Close stops the timer and flushes any remaining buffered cycles.
func (a *CycleAggregator) Close(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.closed = true
	if a.flushTimer != nil {
		a.flushTimer.Stop()
		a.flushTimer = nil
	}
	if a.stats.cycles > 0 {
		a.flushLocked(ctx, a.now())
	}
}

func (a *CycleAggregator) accumulateStats(result *kerneldomain.CycleResult) {
	if result == nil {
		a.stats.cycles++
		return
	}
	a.stats.cycles++
	a.stats.dispatched += result.Dispatched
	a.stats.succeeded += result.Succeeded
	a.stats.failed += result.Failed
	for _, entry := range result.AgentSummary {
		a.stats.agentSet[entry.AgentID] = struct{}{}
	}
}

func (a *CycleAggregator) scheduleFlush(ctx context.Context) {
	a.flushTimer = time.AfterFunc(a.window, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closed {
			return
		}
		a.flushLocked(ctx, a.now())
	})
}

// flushLocked sends the aggregated summary and resets the window.
// Caller must hold a.mu.
func (a *CycleAggregator) flushLocked(ctx context.Context, now time.Time) {
	summary := a.buildSummary(now)
	// Reset state for next window.
	a.windowStart = now
	a.stats = newWindowStats()

	if a.flushTimer != nil {
		a.flushTimer.Stop()
	}
	if !a.closed {
		a.scheduleFlush(ctx)
	}

	if summary.TotalCycles == 0 {
		return
	}

	text := FormatAggregatedSummary(a.kernelID, summary)
	a.sender(ctx, text)
}

func (a *CycleAggregator) buildSummary(now time.Time) WindowSummary {
	agents := make([]string, 0, len(a.stats.agentSet))
	for id := range a.stats.agentSet {
		agents = append(agents, id)
	}
	sort.Strings(agents)
	return WindowSummary{
		WindowStart:      a.windowStart,
		WindowEnd:        now,
		TotalCycles:      a.stats.cycles,
		TotalDispatched:  a.stats.dispatched,
		TotalSucceeded:   a.stats.succeeded,
		TotalFailed:      a.stats.failed,
		ImmediateAlerts:  a.stats.immediateAlerts,
		NoteworthyEvents: a.stats.noteworthy,
		UniqueAgents:     agents,
	}
}

func newWindowStats() windowStats {
	return windowStats{agentSet: make(map[string]struct{})}
}

func isAnomalous(result *kerneldomain.CycleResult, err error) bool {
	if err != nil {
		return true
	}
	if result == nil {
		return false
	}
	return result.Status == kerneldomain.CycleFailed || result.Status == kerneldomain.CyclePartialSuccess
}

func anomalyReason(result *kerneldomain.CycleResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result == nil {
		return "unknown"
	}
	if len(result.FailedAgents) > 0 {
		parts := make([]string, 0, len(result.AgentSummary))
		for _, entry := range result.AgentSummary {
			if entry.Status == kerneldomain.DispatchFailed {
				errMsg := entry.Error
				if errMsg == "" {
					errMsg = "(unknown error)"
				}
				parts = append(parts, fmt.Sprintf("%s — %s", entry.AgentID, compactSummary(errMsg, 100)))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return statusLabel(result.Status)
}

// FormatImmediateAlert formats a short alert for anomalous cycles.
func FormatImmediateAlert(kernelID string, result *kerneldomain.CycleResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Kernel[%s] ⚠ Alert\n- error: %v", kernelID, err)
	}
	if result == nil {
		return fmt.Sprintf("Kernel[%s] ⚠ Alert\n- status: unknown", kernelID)
	}
	lines := []string{
		fmt.Sprintf("Kernel[%s] ⚠ Alert", kernelID),
		fmt.Sprintf("- cycle: %s", result.CycleID),
		fmt.Sprintf("- status: %s", statusLabel(result.Status)),
	}
	for _, entry := range result.AgentSummary {
		if entry.Status == kerneldomain.DispatchFailed {
			errMsg := compactSummary(entry.Error, 150)
			if errMsg == "" {
				errMsg = "(unknown error)"
			}
			lines = append(lines, fmt.Sprintf("- FAIL: %s — %s", entry.AgentID, errMsg))
		}
	}
	lines = append(lines, fmt.Sprintf("- elapsed: %.1fs", result.Duration.Seconds()))
	return strings.Join(lines, "\n")
}

// FormatAggregatedSummary formats a compact window summary notification.
func FormatAggregatedSummary(kernelID string, s WindowSummary) string {
	startFmt := s.WindowStart.Format("15:04")
	endFmt := s.WindowEnd.Format("15:04")

	statusLine := "all ok"
	if s.TotalFailed > 0 {
		statusLine = fmt.Sprintf("%d ok, %d failed", s.TotalSucceeded, s.TotalFailed)
	}

	lines := []string{
		fmt.Sprintf("Kernel[%s] Summary (%s–%s)", kernelID, startFmt, endFmt),
		fmt.Sprintf("- cycles: %d, %s", s.TotalCycles, statusLine),
		fmt.Sprintf("- tasks: %d dispatched, %d succeeded (%.0f%%)", s.TotalDispatched, s.TotalSucceeded, windowRate(s)),
	}
	if len(s.UniqueAgents) > 0 {
		lines = append(lines, fmt.Sprintf("- agents: %s", strings.Join(s.UniqueAgents, ", ")))
	}
	if s.ImmediateAlerts > 0 {
		lines = append(lines, fmt.Sprintf("- alerts sent: %d", s.ImmediateAlerts))
	}
	if len(s.NoteworthyEvents) > 0 {
		lines = append(lines, "- events:")
		for _, event := range s.NoteworthyEvents {
			lines = append(lines, fmt.Sprintf("  - %s", event))
		}
	}
	return strings.Join(lines, "\n")
}

func windowRate(s WindowSummary) float64 {
	if s.TotalDispatched == 0 {
		return 0
	}
	return 100 * float64(s.TotalSucceeded) / float64(s.TotalDispatched)
}
