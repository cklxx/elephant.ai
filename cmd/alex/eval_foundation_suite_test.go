package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunFoundationSuiteEvaluationFromEvalCommand(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	casesPath := filepath.Join(tmp, "cases.yaml")
	casesYAML := `
version: "1"
name: "mini"
scenarios:
  - id: "case-1"
    category: "planning"
    intent: "Break this task into milestones."
    expected_tools: ["plan"]
`
	if err := os.WriteFile(casesPath, []byte(casesYAML), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

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
    cases_path: "` + filepath.ToSlash(casesPath) + `"
`
	if err := os.WriteFile(suitePath, []byte(suiteYAML), 0644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	outputDir := filepath.Join(tmp, "out")
	var c CLI
	err := c.handleEval([]string{
		"foundation-suite",
		"--suite", suitePath,
		"--output", outputDir,
		"--format", "json",
	})
	if err != nil {
		t.Fatalf("handleEval foundation-suite error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(outputDir, "foundation_suite_result_*.json"))
	if err != nil {
		t.Fatalf("glob output: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected foundation suite json report in %s", outputDir)
	}
}
