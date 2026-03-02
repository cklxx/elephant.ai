package kernel

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	kerneldomain "alex/internal/domain/kernel"
)

// ---------------------------------------------------------------------------
// ValidateSchedule
// ---------------------------------------------------------------------------

func TestValidateSchedule_ValidExpressions(t *testing.T) {
	valid := []string{
		"0 * * * *",   // every hour
		"0 9 * * 1-5", // 9am weekdays
		"*/5 * * * *", // every 5 minutes
		"0 0 1 * *",   // first of each month
		"30 6 * * 0",  // 6:30am Sundays
	}
	for _, expr := range valid {
		t.Run(expr, func(t *testing.T) {
			if err := ValidateSchedule(expr); err != nil {
				t.Errorf("ValidateSchedule(%q) = %v, want nil", expr, err)
			}
		})
	}
}

func TestValidateSchedule_InvalidExpressions(t *testing.T) {
	invalid := []string{
		"",
		"not-a-cron",
		"99 * * * *",
		"* * * * * * extra",
	}
	for _, expr := range invalid {
		t.Run(fmt.Sprintf("expr=%q", expr), func(t *testing.T) {
			if err := ValidateSchedule(expr); err == nil {
				t.Errorf("ValidateSchedule(%q) = nil, want error", expr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsSandboxPathRestriction
// ---------------------------------------------------------------------------

func TestIsSandboxPathRestriction_NilError(t *testing.T) {
	if IsSandboxPathRestriction(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsSandboxPathRestriction_PermissionError(t *testing.T) {
	err := fs.ErrPermission
	if !IsSandboxPathRestriction(err) {
		t.Error("expected true for fs.ErrPermission")
	}
}

func TestIsSandboxPathRestriction_StringMatches(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"permission denied by OS", true},
		{"operation not permitted", true},
		{"read-only file system", true},
		{"path must stay within the working directory", true},
		{"sandbox: restrict access", true},
		{"some other error", false},
		{"timeout occurred", false},
	}
	for _, tc := range cases {
		err := errors.New(tc.msg)
		got := IsSandboxPathRestriction(err)
		if got != tc.want {
			t.Errorf("IsSandboxPathRestriction(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

func TestIsSandboxPathRestriction_WrappedPermissionError(t *testing.T) {
	wrapped := fmt.Errorf("disk op failed: %w", fs.ErrPermission)
	if !IsSandboxPathRestriction(wrapped) {
		t.Error("expected true for wrapped fs.ErrPermission")
	}
}

// ---------------------------------------------------------------------------
// FormatCycleMetrics
// ---------------------------------------------------------------------------

func TestFormatCycleMetrics_Nil(t *testing.T) {
	got := FormatCycleMetrics(nil)
	if got != "" {
		t.Errorf("FormatCycleMetrics(nil) = %q, want empty", got)
	}
}

func TestFormatCycleMetrics_ZeroDispatched(t *testing.T) {
	result := &kerneldomain.CycleResult{
		CycleID:    "c1",
		Dispatched: 0,
		Succeeded:  0,
		Failed:     0,
		Duration:   2 * time.Second,
	}
	got := FormatCycleMetrics(result)
	if !strings.Contains(got, "成功率 0.0%") {
		t.Errorf("expected 成功率 0.0%%, got %q", got)
	}
	if !strings.Contains(got, "0 个任务") {
		t.Errorf("expected 0 个任务, got %q", got)
	}
}

func TestFormatCycleMetrics_FullSuccess(t *testing.T) {
	result := &kerneldomain.CycleResult{
		Dispatched: 4,
		Succeeded:  4,
		Failed:     0,
		Duration:   10 * time.Second,
	}
	got := FormatCycleMetrics(result)
	if !strings.Contains(got, "成功率 100.0%") {
		t.Errorf("expected 成功率 100.0%%, got %q", got)
	}
	if !strings.Contains(got, "10.0 秒") {
		t.Errorf("expected 10.0 秒, got %q", got)
	}
}

func TestFormatCycleMetrics_PartialSuccess(t *testing.T) {
	result := &kerneldomain.CycleResult{
		Dispatched: 4,
		Succeeded:  3,
		Failed:     1,
		Duration:   5500 * time.Millisecond,
	}
	got := FormatCycleMetrics(result)
	if !strings.Contains(got, "75.0%") {
		t.Errorf("expected 75.0%% in %q", got)
	}
	if !strings.Contains(got, "5.5 秒") {
		t.Errorf("expected 5.5 秒 in %q", got)
	}
}

// ---------------------------------------------------------------------------
// NarrateCycleFallback
// ---------------------------------------------------------------------------

func TestNarrateCycleFallback_Error(t *testing.T) {
	err := errors.New("planning failed")
	got := NarrateCycleFallback(nil, err)
	if !strings.Contains(got, "planning failed") {
		t.Errorf("expected error message in output, got %q", got)
	}
}

func TestNarrateCycleFallback_Nil(t *testing.T) {
	got := NarrateCycleFallback(nil, nil)
	if got == "" {
		t.Error("expected non-empty output for nil result")
	}
}

func TestNarrateCycleFallback_Success(t *testing.T) {
	result := &kerneldomain.CycleResult{
		Status:     kerneldomain.CycleSuccess,
		Dispatched: 3,
		Succeeded:  3,
		Duration:   4 * time.Second,
	}
	got := NarrateCycleFallback(result, nil)
	if !strings.Contains(got, "3") {
		t.Errorf("expected task count in output, got %q", got)
	}
	if !strings.Contains(got, "4.0 秒") {
		t.Errorf("expected duration in output, got %q", got)
	}
}

func TestNarrateCycleFallback_PartialSuccess(t *testing.T) {
	result := &kerneldomain.CycleResult{
		Status:     kerneldomain.CyclePartialSuccess,
		Dispatched: 5,
		Failed:     2,
		Duration:   3 * time.Second,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-1", Status: kerneldomain.DispatchDone, Summary: "done well"},
			{AgentID: "agent-2", Status: kerneldomain.DispatchFailed, Error: "timeout"},
		},
	}
	got := NarrateCycleFallback(result, nil)
	if !strings.Contains(got, "2") {
		t.Errorf("expected failed count in output, got %q", got)
	}
	if !strings.Contains(got, "agent-1") {
		t.Errorf("expected agent-1 summary, got %q", got)
	}
	if !strings.Contains(got, "timeout") {
		t.Errorf("expected agent-2 error, got %q", got)
	}
}

func TestNarrateCycleFallback_Failed(t *testing.T) {
	result := &kerneldomain.CycleResult{
		Status:     kerneldomain.CycleFailed,
		Dispatched: 2,
		Failed:     2,
		Duration:   1 * time.Second,
	}
	got := NarrateCycleFallback(result, nil)
	if !strings.Contains(got, "全部失败") {
		t.Errorf("expected 全部失败 in %q", got)
	}
}

func TestNarrateCycleFallback_UnknownStatus(t *testing.T) {
	result := &kerneldomain.CycleResult{
		Status:     kerneldomain.CycleResultStatus("unknown_status"),
		Dispatched: 1,
		Duration:   500 * time.Millisecond,
	}
	got := NarrateCycleFallback(result, nil)
	if got == "" {
		t.Error("expected non-empty output for unknown status")
	}
}
