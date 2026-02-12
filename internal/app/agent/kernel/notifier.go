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
		return fmt.Sprintf("Kernel[%s] 周期完成总结\n- 状态: unknown\n- 任务总数: 0\n- 已完成: 0\n- 失败: 0\n- 完成率: 0.0%%\n- 失败任务: (none)\n- 耗时: 0.0s", kernelID)
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

	return fmt.Sprintf(
		"Kernel[%s] 周期完成总结\n- cycle_id: %s\n- 状态: %s\n- 任务总数: %d\n- 已完成: %d\n- 失败: %d\n- 完成率: %.1f%%\n- 失败任务: %s\n- 耗时: %s",
		kernelID,
		result.CycleID,
		statusLabel(result.Status),
		result.Dispatched,
		result.Succeeded,
		result.Failed,
		rate,
		failedAgents,
		dur,
	)
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
