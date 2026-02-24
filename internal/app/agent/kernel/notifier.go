package kernel

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// CycleNotifier is called after each non-empty kernel cycle.
type CycleNotifier func(ctx context.Context, result *kerneldomain.CycleResult, err error)

// FormatCycleNotification formats a human-readable notification for a kernel cycle.
func FormatCycleNotification(kernelID string, result *kerneldomain.CycleResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Kernel[%s] 周期异常\n- 错误: %v", kernelID, err)
	}
	if result == nil {
		return fmt.Sprintf("Kernel[%s] 周期完成总结\n- 状态: unknown\n- 任务总数: 0\n- 已完成: 0\n- 失败: 0\n- 完成率: 0.0%%\n- 失败任务: (none)\n- 执行总结: (none)\n- 耗时: 0.0s", kernelID)
	}

	dur := fmt.Sprintf("%.1fs", result.Duration.Seconds())
	failedAgents := "(none)"
	if len(result.FailedAgents) > 0 {
		failedAgents = strings.Join(result.FailedAgents, ",")
	}
	rate := 0.0
	if result.Dispatched > 0 {
		rate = 100 * float64(result.Succeeded) / float64(result.Dispatched)
		rate = math.Round(rate*10) / 10
	}

	lines := []string{
		fmt.Sprintf("Kernel[%s] 周期完成总结", kernelID),
		fmt.Sprintf("- cycle_id: %s", result.CycleID),
		fmt.Sprintf("- 状态: %s", statusLabel(result.Status)),
		fmt.Sprintf("- 任务总数: %d", result.Dispatched),
		fmt.Sprintf("- 已完成: %d", result.Succeeded),
		fmt.Sprintf("- 失败: %d", result.Failed),
		fmt.Sprintf("- 完成率: %.1f%%", rate),
		fmt.Sprintf("- 失败任务: %s", failedAgents),
	}
	autonomy := summarizeAutonomySignals(result)
	lines = append(lines, fmt.Sprintf("- 主动性: actionable=%d/%d, auto_recovered=%d, blocked_awaiting_input=%d, blocked_no_action=%d",
		autonomy.Actionable, autonomy.Total, autonomy.AutoRecovered, autonomy.BlockedAwaitingInput, autonomy.BlockedNoAction))
	if len(result.AgentSummary) == 0 {
		lines = append(lines, "- 执行总结: (none)")
	} else {
		lines = append(lines, "- 执行总结:")
		for _, entry := range result.AgentSummary {
			lines = append(lines, fmt.Sprintf("  - %s", formatAgentSummaryLine(entry)))
		}
	}
	lines = append(lines, fmt.Sprintf("- 耗时: %s", dur))
	return strings.Join(lines, "\n")
}

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

func formatAgentSummaryLine(entry kerneldomain.AgentCycleSummary) string {
	status := string(entry.Status)
	if status == "" {
		status = string(kerneldomain.DispatchDone)
	}
	if status == string(kerneldomain.DispatchFailed) {
		errMsg := compactSummary(entry.Error, 220)
		if errMsg == "" {
			errMsg = "(unknown error)"
		}
		return fmt.Sprintf("[%s|%s] %s", entry.AgentID, status, errMsg)
	}
	summary := compactSummary(entry.Summary, 220)
	if summary == "" {
		summary = "(empty summary)"
	}
	return fmt.Sprintf("[%s|%s] %s", entry.AgentID, status, summary)
}

type autonomySignalSummary struct {
	Total                int
	Actionable           int
	AutoRecovered        int
	BlockedAwaitingInput int
	BlockedNoAction      int
}

func summarizeAutonomySignals(result *kerneldomain.CycleResult) autonomySignalSummary {
	signals := autonomySignalSummary{}
	if result == nil {
		return signals
	}
	signals.Total = len(result.AgentSummary)
	for _, entry := range result.AgentSummary {
		switch entry.Status {
		case kerneldomain.DispatchDone:
			signals.Actionable++
			if extractAttempts(entry.Summary) > defaultKernelAttemptCount {
				signals.AutoRecovered++
			}
		case kerneldomain.DispatchFailed:
			lowerErr := strings.ToLower(strings.TrimSpace(entry.Error))
			if strings.Contains(lowerErr, strings.ToLower(errKernelAwaitingUserConfirmation.Error())) {
				signals.BlockedAwaitingInput++
			}
			if strings.Contains(lowerErr, strings.ToLower(errKernelNoRealToolAction.Error())) {
				signals.BlockedNoAction++
			}
		}
	}
	return signals
}

func extractAttempts(summary string) int {
	lower := strings.ToLower(summary)
	idx := strings.Index(lower, "attempts=")
	if idx < 0 {
		return defaultKernelAttemptCount
	}
	start := idx + len("attempts=")
	end := start
	for end < len(lower) && lower[end] >= '0' && lower[end] <= '9' {
		end++
	}
	if end <= start {
		return defaultKernelAttemptCount
	}
	value, err := strconv.Atoi(lower[start:end])
	if err != nil || value <= 0 {
		return defaultKernelAttemptCount
	}
	return value
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

// bufferedCycle stores the minimum info needed from a routine success cycle.
type bufferedCycle struct {
	dispatched int
	succeeded  int
	failed     int
	agents     []string // agent IDs from AgentSummary
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
		return fmt.Sprintf("Kernel[%s] ⚠ 异常\n- 错误: %v", kernelID, err)
	}
	if result == nil {
		return fmt.Sprintf("Kernel[%s] ⚠ 异常\n- 状态: unknown", kernelID)
	}
	lines := []string{
		fmt.Sprintf("Kernel[%s] ⚠ 异常", kernelID),
		fmt.Sprintf("- cycle: %s", result.CycleID),
		fmt.Sprintf("- 状态: %s", statusLabel(result.Status)),
	}
	for _, entry := range result.AgentSummary {
		if entry.Status == kerneldomain.DispatchFailed {
			errMsg := compactSummary(entry.Error, 150)
			if errMsg == "" {
				errMsg = "(unknown error)"
			}
			lines = append(lines, fmt.Sprintf("- 失败: %s — %s", entry.AgentID, errMsg))
		}
	}
	lines = append(lines, fmt.Sprintf("- 耗时: %.1fs", result.Duration.Seconds()))
	return strings.Join(lines, "\n")
}

// FormatAggregatedSummary formats a compact window summary notification.
func FormatAggregatedSummary(kernelID string, s WindowSummary) string {
	startFmt := s.WindowStart.Format("15:04")
	endFmt := s.WindowEnd.Format("15:04")

	statusLine := "全部成功"
	if s.TotalFailed > 0 {
		statusLine = fmt.Sprintf("%d 成功, %d 失败", s.TotalSucceeded, s.TotalFailed)
	}

	lines := []string{
		fmt.Sprintf("Kernel[%s] 阶段总结 (%s–%s)", kernelID, startFmt, endFmt),
		fmt.Sprintf("- 周期: %d, %s", s.TotalCycles, statusLine),
		fmt.Sprintf("- 任务: %d dispatched, %d succeeded (%.0f%%)", s.TotalDispatched, s.TotalSucceeded, windowRate(s)),
	}
	if len(s.UniqueAgents) > 0 {
		lines = append(lines, fmt.Sprintf("- 代理: %s", strings.Join(s.UniqueAgents, ", ")))
	}
	if s.ImmediateAlerts > 0 {
		lines = append(lines, fmt.Sprintf("- 即时告警: %d", s.ImmediateAlerts))
	}
	if len(s.NoteworthyEvents) > 0 {
		lines = append(lines, "- 异常事件:")
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
