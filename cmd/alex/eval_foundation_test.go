package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFoundationEvaluationFromEvalCommand(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	casePath := filepath.Join(tmp, "cases.yaml")
	caseYAML := `
version: "1"
name: "mini"
scenarios:
  - id: "case-1"
    category: "planning"
    intent: "Break this task into milestones."
    expected_tools: ["plan"]
`
	if err := os.WriteFile(casePath, []byte(caseYAML), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	outputDir := filepath.Join(tmp, "out")
	var c CLI
	err := c.handleEval([]string{
		"foundation",
		"--cases", casePath,
		"--output", outputDir,
		"--mode", "web",
		"--preset", "full",
		"--format", "json",
	})
	if err != nil {
		t.Fatalf("handleEval foundation error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(outputDir, "foundation_result_*.json"))
	if err != nil {
		t.Fatalf("glob output: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected foundation json report in %s", outputDir)
	}
}

func TestRunFoundationEvaluationRejectsInvalidMode(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	casePath := filepath.Join(tmp, "cases.yaml")
	caseYAML := `
version: "1"
name: "mini"
scenarios:
  - id: "case-1"
    category: "planning"
    intent: "Break this task into milestones."
    expected_tools: ["plan"]
`
	if err := os.WriteFile(casePath, []byte(caseYAML), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	var c CLI
	err := c.runFoundationEvaluation([]string{
		"--cases", casePath,
		"--output", filepath.Join(tmp, "out"),
		"--mode", "invalid-mode",
	})
	if err == nil {
		t.Fatalf("expected invalid mode error")
	}
	if !strings.Contains(err.Error(), "unsupported mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}
