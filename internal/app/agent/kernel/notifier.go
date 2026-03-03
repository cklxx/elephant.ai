package kernel

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	kerneldomain "alex/internal/domain/kernel"
)

// CycleNotifier is called after each non-empty kernel cycle.
type CycleNotifier func(ctx context.Context, result *kerneldomain.CycleResult, err error)

// FormatCycleNotification formats a human-readable notification for a kernel cycle.
func FormatCycleNotification(kernelID string, result *kerneldomain.CycleResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Kernel[%s] Cycle Error\n- Error: %v", kernelID, err)
	}
	if result == nil {
		return fmt.Sprintf("Kernel[%s] Cycle Summary\n- Status: unknown\n- Total tasks: 0\n- Succeeded: 0\n- Failed: 0\n- Success rate: 0.0%%\n- Failed tasks: (none)\n- Execution summary: (none)\n- Duration: 0.0s", kernelID)
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
		fmt.Sprintf("Kernel[%s] Cycle Summary", kernelID),
		fmt.Sprintf("- cycle_id: %s", result.CycleID),
		fmt.Sprintf("- Status: %s", statusLabel(result.Status)),
		fmt.Sprintf("- Total tasks: %d", result.Dispatched),
		fmt.Sprintf("- Succeeded: %d", result.Succeeded),
		fmt.Sprintf("- Failed: %d", result.Failed),
		fmt.Sprintf("- Success rate: %.1f%%", rate),
		fmt.Sprintf("- Failed tasks: %s", failedAgents),
	}
	autonomy := summarizeAutonomySignals(result)
	lines = append(lines, fmt.Sprintf("- Autonomy: actionable=%d/%d, auto_recovered=%d, blocked_awaiting_input=%d, blocked_no_action=%d",
		autonomy.Actionable, autonomy.Total, autonomy.AutoRecovered, autonomy.BlockedAwaitingInput, autonomy.BlockedNoAction))
	if len(result.AgentSummary) == 0 {
		lines = append(lines, "- Execution summary: (none)")
	} else {
		lines = append(lines, "- Execution summary:")
		for _, entry := range result.AgentSummary {
			lines = append(lines, fmt.Sprintf("  - %s", formatAgentSummaryLine(entry)))
		}
	}
	lines = append(lines, fmt.Sprintf("- Duration: %s", dur))
	return strings.Join(lines, "\n")
}

// FormatCycleMetrics returns a one-line Chinese metrics summary suitable for
// appending after a narrated cycle notification.
func FormatCycleMetrics(result *kerneldomain.CycleResult) string {
	if result == nil {
		return ""
	}
	rate := 0.0
	if result.Dispatched > 0 {
		rate = 100 * float64(result.Succeeded) / float64(result.Dispatched)
		rate = math.Round(rate*10) / 10
	}
	dur := fmt.Sprintf("%.1f", result.Duration.Seconds())
	return fmt.Sprintf("成功率 %.1f%% | %d 个任务（%d 成功 %d 失败）| 耗时 %s 秒",
		rate, result.Dispatched, result.Succeeded, result.Failed, dur)
}

// NarrateCycleFallback produces a Chinese template-based cycle summary when
// LLM narration is unavailable.
func NarrateCycleFallback(result *kerneldomain.CycleResult, err error) string {
	if err != nil {
		return fmt.Sprintf("调度周期执行出错：%v", err)
	}
	if result == nil {
		return "调度周期已完成，无任务执行。"
	}
	var b strings.Builder
	switch result.Status {
	case kerneldomain.CycleSuccess:
		b.WriteString(fmt.Sprintf("本轮调度全部 %d 个任务成功完成", result.Dispatched))
	case kerneldomain.CyclePartialSuccess:
		b.WriteString(fmt.Sprintf("本轮调度 %d 个任务中有 %d 个失败", result.Dispatched, result.Failed))
	case kerneldomain.CycleFailed:
		b.WriteString(fmt.Sprintf("本轮调度 %d 个任务全部失败", result.Dispatched))
	default:
		b.WriteString(fmt.Sprintf("本轮调度执行了 %d 个任务", result.Dispatched))
	}
	b.WriteString(fmt.Sprintf("，耗时 %.1f 秒。", result.Duration.Seconds()))
	for _, entry := range result.AgentSummary {
		summary := compactSummary(entry.Summary, 120)
		if entry.Status == kerneldomain.DispatchFailed {
			summary = compactSummary(entry.Error, 120)
		}
		if summary != "" {
			b.WriteString(fmt.Sprintf("\n%s：%s", entry.AgentID, summary))
		}
	}
	return b.String()
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
			failureClass := strings.TrimSpace(entry.FailureClass)
			if failureClass == "" {
				failureClass = inferFailureClassFromError(entry.Error)
			}
			switch failureClass {
			case kernelAutonomyAwaiting:
				signals.BlockedAwaitingInput++
			case kernelAutonomyNoTool:
				signals.BlockedNoAction++
			}
		}
	}
	return signals
}

func inferFailureClassFromError(errMsg string) string {
	trimmed := strings.TrimSpace(errMsg)
	if strings.HasPrefix(trimmed, "[") {
		if end := strings.Index(trimmed, "]"); end > 1 {
			return strings.TrimSpace(trimmed[1:end])
		}
	}
	switch trimmed {
	case errKernelAwaitingUserConfirmation.Error():
		return kernelAutonomyAwaiting
	case errKernelNoRealToolAction.Error():
		return kernelAutonomyNoTool
	default:
		return ""
	}
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
