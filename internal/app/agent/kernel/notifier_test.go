package kernel

import (
	"fmt"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

func TestFormatCycleNotification_Success(t *testing.T) {
	result := &kerneldomain.CycleResult{
		KernelID:   "default",
		Status:     kerneldomain.CycleSuccess,
		Dispatched: 2,
		Succeeded:  2,
		Duration:   3200 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成 (分发=2 成功=2 耗时=3.2s)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_PartialFailure(t *testing.T) {
	result := &kerneldomain.CycleResult{
		KernelID:     "default",
		Status:       kerneldomain.CyclePartialSuccess,
		Dispatched:   3,
		Succeeded:    2,
		Failed:       1,
		FailedAgents: []string{"agent-b"},
		Duration:     5100 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 部分失败 (分发=3 成功=2 失败=1 [agent-b] 耗时=5.1s)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_AllFailed(t *testing.T) {
	result := &kerneldomain.CycleResult{
		KernelID:     "default",
		Status:       kerneldomain.CycleFailed,
		Dispatched:   2,
		Failed:       2,
		FailedAgents: []string{"agent-a", "agent-b"},
		Duration:     1500 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 全部失败 (分发=2 失败=2 [agent-a,agent-b] 耗时=1.5s)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_CycleError(t *testing.T) {
	got := FormatCycleNotification("default", nil, fmt.Errorf("read state: file not found"))
	want := "Kernel[default] 周期异常: read state: file not found"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
