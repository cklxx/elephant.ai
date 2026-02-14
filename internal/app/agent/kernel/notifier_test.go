package kernel

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

func TestFormatCycleNotification_Success(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "cycle-1",
		KernelID:   "default",
		Status:     kerneldomain.CycleSuccess,
		Dispatched: 2,
		Succeeded:  2,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchDone, Summary: "已完成 A"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchDone, Summary: "已完成 B"},
		},
		Duration: 3200 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成总结\n- cycle_id: cycle-1\n- 状态: success\n- 任务总数: 2\n- 已完成: 2\n- 失败: 0\n- 完成率: 100.0%\n- 失败任务: (none)\n- 主动性: actionable=2/2, auto_recovered=0, blocked_awaiting_input=0, blocked_no_action=0\n- 执行总结:\n  - [agent-a|done] 已完成 A\n  - [agent-b|done] 已完成 B\n- 耗时: 3.2s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_PartialFailure(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:      "cycle-2",
		KernelID:     "default",
		Status:       kerneldomain.CyclePartialSuccess,
		Dispatched:   3,
		Succeeded:    2,
		Failed:       1,
		FailedAgents: []string{"agent-b"},
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchDone, Summary: "修复配置"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "rate limit"},
		},
		Duration: 5100 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成总结\n- cycle_id: cycle-2\n- 状态: partial_success\n- 任务总数: 3\n- 已完成: 2\n- 失败: 1\n- 完成率: 66.7%\n- 失败任务: agent-b\n- 主动性: actionable=1/2, auto_recovered=0, blocked_awaiting_input=0, blocked_no_action=0\n- 执行总结:\n  - [agent-a|done] 修复配置\n  - [agent-b|failed] rate limit\n- 耗时: 5.1s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_AllFailed(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:      "cycle-3",
		KernelID:     "default",
		Status:       kerneldomain.CycleFailed,
		Dispatched:   2,
		Failed:       2,
		FailedAgents: []string{"agent-a", "agent-b"},
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchFailed, Error: "a failed"},
			{AgentID: "agent-b", Status: kerneldomain.DispatchFailed, Error: "b failed"},
		},
		Duration: 1500 * time.Millisecond,
	}
	got := FormatCycleNotification("default", result, nil)
	want := "Kernel[default] 周期完成总结\n- cycle_id: cycle-3\n- 状态: failed\n- 任务总数: 2\n- 已完成: 0\n- 失败: 2\n- 完成率: 0.0%\n- 失败任务: agent-a,agent-b\n- 主动性: actionable=0/2, auto_recovered=0, blocked_awaiting_input=0, blocked_no_action=0\n- 执行总结:\n  - [agent-a|failed] a failed\n  - [agent-b|failed] b failed\n- 耗时: 1.5s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_CycleError(t *testing.T) {
	got := FormatCycleNotification("default", nil, fmt.Errorf("read state: file not found"))
	want := "Kernel[default] 周期异常\n- 错误: read state: file not found"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCycleNotification_AutonomySignals(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "cycle-4",
		KernelID:   "default",
		Status:     kerneldomain.CyclePartialSuccess,
		Dispatched: 3,
		Succeeded:  1,
		Failed:     2,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{
				AgentID: "agent-a",
				Status:  kerneldomain.DispatchDone,
				Summary: "[autonomy=actionable, attempts=2, recovered_from=awaiting_input] 已执行",
			},
			{
				AgentID: "agent-b",
				Status:  kerneldomain.DispatchFailed,
				Error:   errKernelAwaitingUserConfirmation.Error(),
			},
			{
				AgentID: "agent-c",
				Status:  kerneldomain.DispatchFailed,
				Error:   errKernelNoRealToolAction.Error(),
			},
		},
		Duration: 2500 * time.Millisecond,
	}

	got := FormatCycleNotification("default", result, nil)
	wantLine := "- 主动性: actionable=1/3, auto_recovered=1, blocked_awaiting_input=1, blocked_no_action=1"
	if !strings.Contains(got, wantLine) {
		t.Fatalf("expected autonomy signals line %q in notification:\n%s", wantLine, got)
	}
}
