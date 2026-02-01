package larktesting

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TestReport is the JSON-serializable output of a full scenario run.
type TestReport struct {
	Timestamp  time.Time        `json:"timestamp"`
	Duration   time.Duration    `json:"duration_ms"`
	Summary    ReportSummary    `json:"summary"`
	Scenarios  []ScenarioReport `json:"scenarios"`
}

// ReportSummary aggregates pass/fail/skip counts.
type ReportSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
}

// ScenarioReport captures the outcome of a single scenario.
type ScenarioReport struct {
	Name     string       `json:"name"`
	Status   string       `json:"status"` // "pass", "fail"
	Duration string       `json:"duration"`
	Turns    []TurnReport `json:"turns"`
	Errors   []string     `json:"errors,omitempty"`
}

// TurnReport captures a single turn's outcome for the report.
type TurnReport struct {
	Index    int      `json:"index"`
	Duration string   `json:"duration"`
	Calls    int      `json:"calls"`
	Errors   []string `json:"errors,omitempty"`
}

// FailureDiagnosis classifies a scenario failure for the self-iteration loop.
type FailureDiagnosis struct {
	ScenarioName string `json:"scenario_name"`
	RootCause    string `json:"root_cause"`   // test_drift, prompt_issue, tool_bug, gateway_logic, context_issue, llm_quality, architecture
	FixTier      int    `json:"fix_tier"`      // 1-4
	Description  string `json:"description"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
	FilesInvolved []string `json:"files_involved,omitempty"`
}

// BuildReport constructs a TestReport from a slice of ScenarioResults.
func BuildReport(results []*ScenarioResult) *TestReport {
	report := &TestReport{
		Timestamp: time.Now(),
	}

	var totalDuration time.Duration
	for _, r := range results {
		totalDuration += r.Duration

		sr := ScenarioReport{
			Name:     r.Name,
			Duration: r.Duration.Round(time.Millisecond).String(),
		}

		if r.Passed {
			sr.Status = "pass"
			report.Summary.Passed++
		} else {
			sr.Status = "fail"
			report.Summary.Failed++
		}
		report.Summary.Total++

		for _, tr := range r.Turns {
			turnReport := TurnReport{
				Index:    tr.TurnIndex,
				Duration: tr.Duration.Round(time.Millisecond).String(),
				Calls:    len(tr.Calls),
				Errors:   tr.Errors,
			}
			sr.Turns = append(sr.Turns, turnReport)
			sr.Errors = append(sr.Errors, tr.Errors...)
		}

		report.Scenarios = append(report.Scenarios, sr)
	}

	report.Duration = totalDuration
	return report
}

// ToJSON serializes the report as indented JSON.
func (r *TestReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToMarkdown renders the report as a human-readable markdown string.
func (r *TestReport) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 测试报告 %s\n\n", r.Timestamp.Format("2006-01-02 15:04")))

	sb.WriteString("### 概要\n")
	sb.WriteString(fmt.Sprintf("- 通过: %d / 失败: %d / 跳过: %d\n",
		r.Summary.Passed, r.Summary.Failed, r.Summary.Skipped))
	sb.WriteString(fmt.Sprintf("- 耗时: %s\n\n", r.Duration.Round(time.Millisecond)))

	if r.Summary.Failed > 0 {
		sb.WriteString("### 失败场景\n\n")
		sb.WriteString("| 场景 | 耗时 | 错误 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, s := range r.Scenarios {
			if s.Status != "fail" {
				continue
			}
			errSummary := "(none)"
			if len(s.Errors) > 0 {
				first := s.Errors[0]
				if len(first) > 60 {
					first = first[:60] + "..."
				}
				errSummary = first
				if len(s.Errors) > 1 {
					errSummary += fmt.Sprintf(" (+%d more)", len(s.Errors)-1)
				}
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", s.Name, s.Duration, errSummary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### 全部场景\n\n")
	sb.WriteString("| 场景 | 状态 | 耗时 |\n")
	sb.WriteString("|------|------|------|\n")
	for _, s := range r.Scenarios {
		status := "PASS"
		if s.Status == "fail" {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", s.Name, status, s.Duration))
	}

	return sb.String()
}

// ClassifyFixTier determines the fix tier for a failure based on the root cause.
func ClassifyFixTier(rootCause string) int {
	switch rootCause {
	case "test_drift":
		return 1
	case "prompt_issue":
		return 2
	case "tool_bug", "gateway_logic", "context_issue":
		return 3
	case "llm_quality", "architecture":
		return 4
	default:
		return 4
	}
}

// FilesForTier returns the file patterns that each tier is allowed to modify.
func FilesForTier(tier int) []string {
	switch tier {
	case 1:
		return []string{
			"tests/scenarios/*.yaml",
			"internal/channels/lark/testing/*",
		}
	case 2:
		return []string{
			"skills/*.md",
			"skills/*/SKILL.md",
		}
	case 3:
		return []string{
			"internal/channels/lark/*",
			"internal/tools/builtin/*",
			"internal/context/*",
		}
	default:
		return nil // tier 4: report only
	}
}
