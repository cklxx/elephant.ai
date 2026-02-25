package larktesting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	larkgw "alex/internal/delivery/channels/lark"
)

func TestBuildReport(t *testing.T) {
	results := []*ScenarioResult{
		{
			Name:     "basic_p2p",
			Passed:   true,
			Duration: 100 * time.Millisecond,
			Turns: []TurnResult{
				{TurnIndex: 0, Duration: 50 * time.Millisecond, Calls: []larkgw.MessengerCall{{Method: "ReplyMessage"}}},
			},
		},
		{
			Name:     "dedup",
			Passed:   false,
			Duration: 200 * time.Millisecond,
			Turns: []TurnResult{
				{TurnIndex: 0, Duration: 100 * time.Millisecond},
				{TurnIndex: 1, Duration: 100 * time.Millisecond, Errors: []string{"expected no ReplyMessage calls"}},
			},
		},
	}

	report := BuildReport(results)

	if report.Summary.Total != 2 {
		t.Fatalf("expected 2 total, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 1 {
		t.Fatalf("expected 1 passed, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", report.Summary.Failed)
	}
	if len(report.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(report.Scenarios))
	}
	if report.Scenarios[0].Status != "pass" {
		t.Fatalf("expected first scenario to pass, got %q", report.Scenarios[0].Status)
	}
	if report.Scenarios[1].Status != "fail" {
		t.Fatalf("expected second scenario to fail, got %q", report.Scenarios[1].Status)
	}
}

func TestReportToJSON(t *testing.T) {
	results := []*ScenarioResult{
		{Name: "test_1", Passed: true, Duration: 50 * time.Millisecond},
	}
	report := BuildReport(results)

	data, err := report.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var parsed TestReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if parsed.Summary.Total != 1 {
		t.Fatalf("expected 1 total in parsed JSON, got %d", parsed.Summary.Total)
	}
}

func TestReportToMarkdown(t *testing.T) {
	results := []*ScenarioResult{
		{Name: "pass_scenario", Passed: true, Duration: 50 * time.Millisecond},
		{
			Name:     "fail_scenario",
			Passed:   false,
			Duration: 100 * time.Millisecond,
			Turns: []TurnResult{
				{TurnIndex: 0, Duration: 100 * time.Millisecond, Errors: []string{"assertion failed"}},
			},
		},
	}
	report := BuildReport(results)

	md := report.ToMarkdown()
	if !strings.Contains(md, "测试报告") {
		t.Fatalf("expected report header, got %q", md)
	}
	if !strings.Contains(md, "通过: 1") {
		t.Fatalf("expected 1 passed, got %q", md)
	}
	if !strings.Contains(md, "失败: 1") {
		t.Fatalf("expected 1 failed, got %q", md)
	}
	if !strings.Contains(md, "fail_scenario") {
		t.Fatalf("expected fail_scenario in report, got %q", md)
	}
	if !strings.Contains(md, "失败场景") {
		t.Fatalf("expected failure section in report, got %q", md)
	}
}

func TestClassifyFixTier(t *testing.T) {
	tests := []struct {
		cause string
		tier  int
	}{
		{"test_drift", 1},
		{"prompt_issue", 2},
		{"tool_bug", 3},
		{"gateway_logic", 3},
		{"context_issue", 3},
		{"llm_quality", 4},
		{"architecture", 4},
		{"unknown", 4},
	}
	for _, tt := range tests {
		t.Run(tt.cause, func(t *testing.T) {
			got := ClassifyFixTier(tt.cause)
			if got != tt.tier {
				t.Fatalf("ClassifyFixTier(%q) = %d, want %d", tt.cause, got, tt.tier)
			}
		})
	}
}

func TestFilesForTier(t *testing.T) {
	if files := FilesForTier(1); len(files) == 0 {
		t.Fatal("expected tier 1 to have allowed files")
	}
	if files := FilesForTier(2); len(files) == 0 {
		t.Fatal("expected tier 2 to have allowed files")
	}
	if files := FilesForTier(3); len(files) == 0 {
		t.Fatal("expected tier 3 to have allowed files")
	}
	if files := FilesForTier(4); files != nil {
		t.Fatalf("expected tier 4 to have no allowed files, got %v", files)
	}
}
