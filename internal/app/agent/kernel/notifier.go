package kernel

import (
	"context"
	"fmt"
	"math"
	"strings"

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
