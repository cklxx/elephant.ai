package agent_eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFoundationSuiteSetValid(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	casesA := filepath.Join(tmp, "cases-a.yaml")
	casesB := filepath.Join(tmp, "cases-b.yaml")
	writeCaseSet(t, casesA, "case-a", "Plan with milestones", "plan")
	writeCaseSet(t, casesB, "case-b", "Inspect workspace file content", "read_file")

	suitePath := filepath.Join(tmp, "suite.yaml")
	suiteYAML := `
version: "1"
name: "foundation-suite"
collections:
  - id: "tool-coverage"
    name: "Tool Coverage"
    dimension: "tool_coverage"
    cases_path: "` + filepath.ToSlash(casesA) + `"
    mode: "web"
    preset: "full"
    toolset: "default"
    top_k: 3
  - id: "prompt-effectiveness"
    name: "Prompt Effectiveness"
    dimension: "prompt_effectiveness"
    cases_path: "` + filepath.ToSlash(casesB) + `"
`
	if err := os.WriteFile(suitePath, []byte(suiteYAML), 0644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	set, err := LoadFoundationSuiteSet(suitePath)
	if err != nil {
		t.Fatalf("LoadFoundationSuiteSet error: %v", err)
	}
	if set.Name != "foundation-suite" {
		t.Fatalf("unexpected suite name: %s", set.Name)
	}
	if len(set.Collections) != 2 {
		t.Fatalf("unexpected collections count: %d", len(set.Collections))
	}
	if set.Collections[0].ID != "tool-coverage" {
		t.Fatalf("unexpected first collection id: %s", set.Collections[0].ID)
	}
}

func TestLoadFoundationSuiteSetRejectsDuplicateCollectionID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cases := filepath.Join(tmp, "cases.yaml")
	writeCaseSet(t, cases, "case-a", "Plan with milestones", "plan")

	suitePath := filepath.Join(tmp, "suite.yaml")
	suiteYAML := `
version: "1"
name: "dup-suite"
collections:
  - id: "dup"
    name: "A"
    cases_path: "` + filepath.ToSlash(cases) + `"
  - id: "dup"
    name: "B"
    cases_path: "` + filepath.ToSlash(cases) + `"
`
	if err := os.WriteFile(suitePath, []byte(suiteYAML), 0644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	if _, err := LoadFoundationSuiteSet(suitePath); err == nil {
		t.Fatalf("expected duplicate id error")
	}
}

func TestRunFoundationEvaluationSuiteEndToEnd(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	casesA := filepath.Join(tmp, "tool_cases.yaml")
	casesB := filepath.Join(tmp, "proactive_cases.yaml")
	writeCaseSet(t, casesA, "case-plan", "Break migration into phased milestones.", "plan")
	writeCaseSet(t, casesB, "case-request-user", "The flow is blocked and needs user login action.", "request_user")

	suitePath := filepath.Join(tmp, "suite.yaml")
	suiteYAML := `
version: "1"
name: "mini-suite"
collections:
  - id: "tool-coverage"
    name: "Tool Coverage"
    dimension: "tool_coverage"
    mode: "web"
    preset: "full"
    toolset: "default"
    top_k: 3
    cases_path: "` + filepath.ToSlash(casesA) + `"
  - id: "proactivity"
    name: "Proactivity"
    dimension: "proactivity"
    mode: "web"
    preset: "full"
    toolset: "default"
    top_k: 3
    cases_path: "` + filepath.ToSlash(casesB) + `"
`
	if err := os.WriteFile(suitePath, []byte(suiteYAML), 0644); err != nil {
		t.Fatalf("write suite yaml: %v", err)
	}

	outputDir := filepath.Join(tmp, "out")
	result, err := RunFoundationEvaluationSuite(context.Background(), &FoundationSuiteOptions{
		OutputDir:    outputDir,
		SuitePath:    suitePath,
		ReportFormat: "json",
	})
	if err != nil {
		t.Fatalf("RunFoundationEvaluationSuite error: %v", err)
	}
	if result.TotalCollections != 2 {
		t.Fatalf("expected 2 collections, got %d", result.TotalCollections)
	}
	if result.TotalCases != 2 {
		t.Fatalf("expected total cases 2, got %d", result.TotalCases)
	}
	if result.CollectionPassRatio != "2/2" {
		t.Fatalf("expected collection pass ratio 2/2, got %s", result.CollectionPassRatio)
	}
	if result.CasePassRatio != "2/2" {
		t.Fatalf("expected case pass ratio 2/2, got %s", result.CasePassRatio)
	}
	if len(result.CollectionResults) != 2 {
		t.Fatalf("expected 2 collection results, got %d", len(result.CollectionResults))
	}
	for _, row := range result.CollectionResults {
		if row.CasePassRatio == "" {
			t.Fatalf("expected non-empty case ratio for collection %s", row.ID)
		}
	}
	if len(result.ReportArtifacts) == 0 {
		t.Fatalf("expected suite artifacts")
	}
	if result.CollectionResults[0].Summary == nil {
		t.Fatalf("expected nested summary for collection result")
	}

	var sawJSON bool
	for _, artifact := range result.ReportArtifacts {
		if strings.HasSuffix(artifact.Name, ".json") {
			sawJSON = true
		}
	}
	if !sawJSON {
		t.Fatalf("expected suite json artifact, got %+v", result.ReportArtifacts)
	}
}

func TestBuildFoundationSuiteMarkdownReportIncludesPassRatios(t *testing.T) {
	t.Parallel()

	report := buildFoundationSuiteMarkdownReport(&FoundationSuiteResult{
		RunID:                "suite-1",
		SuiteName:            "suite",
		SuitePath:            "evaluation/agent_eval/datasets/foundation_eval_suite.yaml",
		TotalCollections:     4,
		PassedCollections:    3,
		CollectionPassRatio:  "3/4",
		TotalCases:           10,
		PassedCases:          8,
		CasePassRatio:        "8/10",
		DeliverableCaseRatio: "4/10",
		DeliverableGoodRatio: "3/4",
		DeliverableBadRatio:  "1/4",
		CollectionResults: []FoundationSuiteCollectionResult{
			{
				ID:                   "tool-coverage",
				Name:                 "Tool Coverage",
				TopK:                 3,
				CasePassRatio:        "5/6",
				DeliverableCaseRatio: "2/6",
				DeliverableGoodRatio: "1/2",
				DeliverableBadRatio:  "1/2",
				Mode:                 "web",
				Preset:               "full",
				Toolset:              "default",
				TotalCases:           6,
				Summary: &FoundationEvaluationResult{
					Implicit: FoundationImplicitSummary{
						CaseResults: []FoundationCaseResult{
							{
								ID:            "good-case",
								ExpectedTools: []string{"artifacts_write"},
								TopMatches:    []FoundationToolMatch{{Name: "artifacts_write", Score: 8.8}},
								DeliverableCheck: &FoundationDeliverableCheck{
									Applicable:       true,
									SignalCount:      2,
									MatchedSignals:   2,
									ContractCoverage: 1,
									Status:           "good",
									Reason:           "covered",
								},
							},
							{
								ID:            "bad-case",
								ExpectedTools: []string{"write_attachment"},
								TopMatches:    []FoundationToolMatch{{Name: "read_file", Score: 4.1}},
								DeliverableCheck: &FoundationDeliverableCheck{
									Applicable:       true,
									SignalCount:      2,
									MatchedSignals:   0,
									ContractCoverage: 0,
									Status:           "bad",
									Reason:           "missing attachment signal",
								},
							},
						},
					},
				},
			},
		},
	})

	if !strings.Contains(report, "3/4") {
		t.Fatalf("expected collection x/x ratio in report, got: %s", report)
	}
	if !strings.Contains(report, "8/10") {
		t.Fatalf("expected case x/x ratio in report, got: %s", report)
	}
	if !strings.Contains(report, "5/6") {
		t.Fatalf("expected per-collection x/x ratio in report, got: %s", report)
	}
	if !strings.Contains(report, "Deliverable Sampling Check") {
		t.Fatalf("expected deliverable sampling section in report, got: %s", report)
	}
	if !strings.Contains(report, "good-case") || !strings.Contains(report, "bad-case") {
		t.Fatalf("expected both good and bad samples in report, got: %s", report)
	}
	if !strings.Contains(report, "pass@1") || !strings.Contains(report, "pass@5") {
		t.Fatalf("expected pass@ metrics in suite report, got: %s", report)
	}
}

func writeCaseSet(t *testing.T, path, caseID, intent, expectedTool string) {
	t.Helper()
	content := `
version: "1"
name: "mini"
scenarios:
  - id: "` + caseID + `"
    category: "test"
    intent: "` + intent + `"
    expected_tools: ["` + expectedTool + `"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write case set %s: %v", path, err)
	}
}
