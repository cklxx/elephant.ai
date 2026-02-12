package kernel

import (
	"context"
	"fmt"
	"strings"

	kerneldomain "alex/internal/domain/kernel"
)

// CycleNotifier is called after each non-empty kernel cycle.
type CycleNotifier func(ctx context.Context, result *kerneldomain.CycleResult, err error)

// FormatCycleNotification formats a human-readable notification for a kernel cycle.
func FormatCycleNotification(kernelID string, result *kerneldomain.CycleResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Kernel[%s] 周期异常: %v", kernelID, err)
	}

	dur := fmt.Sprintf("%.1fs", result.Duration.Seconds())

	switch result.Status {
	case kerneldomain.CycleSuccess:
		return fmt.Sprintf("Kernel[%s] 周期完成 (分发=%d 成功=%d 耗时=%s)",
			kernelID, result.Dispatched, result.Succeeded, dur)

	case kerneldomain.CyclePartialSuccess:
		return fmt.Sprintf("Kernel[%s] 部分失败 (分发=%d 成功=%d 失败=%d [%s] 耗时=%s)",
			kernelID, result.Dispatched, result.Succeeded, result.Failed,
			strings.Join(result.FailedAgents, ","), dur)

	case kerneldomain.CycleFailed:
		return fmt.Sprintf("Kernel[%s] 全部失败 (分发=%d 失败=%d [%s] 耗时=%s)",
			kernelID, result.Dispatched, result.Failed,
			strings.Join(result.FailedAgents, ","), dur)

	default:
		return fmt.Sprintf("Kernel[%s] 周期完成 (状态=%s)", kernelID, result.Status)
	}
}
