package agent_eval

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	ports "alex/internal/domain/agent/ports"
)

func TestLoadFoundationCaseSetValid(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cases.yaml")
	content := `
version: "1"
name: "foundation"
scenarios:
  - id: "case-a"
    category: "browser"
    intent: "Find a selector and submit the form."
    expected_tools: ["browser_dom"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	set, err := LoadFoundationCaseSet(path)
	if err != nil {
		t.Fatalf("LoadFoundationCaseSet error: %v", err)
	}
	if set.Name != "foundation" {
		t.Fatalf("unexpected set name: %s", set.Name)
	}
	if len(set.Scenarios) != 1 {
		t.Fatalf("unexpected scenario count: %d", len(set.Scenarios))
	}
}

func TestLoadFoundationCaseSetRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cases.yaml")
	content := `
version: "1"
name: "dup"
scenarios:
  - id: "same"
    category: "a"
    intent: "x"
    expected_tools: ["plan"]
  - id: "same"
    category: "b"
    intent: "y"
    expected_tools: ["clarify"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	if _, err := LoadFoundationCaseSet(path); err == nil {
		t.Fatalf("expected duplicate id error")
	}
}

func TestScoreToolProfileDetectsThinDefinition(t *testing.T) {
	t.Parallel()
	score := scoreToolProfile(foundationToolProfile{
		Definition: ports.ToolDefinition{
			Name:        "foo",
			Description: "run",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"input": {Type: "", Description: "x"},
				},
				Required: []string{"input", "missing"},
			},
		},
		Metadata: ports.ToolMetadata{
			Name:        "foo",
			Category:    "",
			Tags:        nil,
			Dangerous:   false,
			SafetyLevel: 0,
		},
		TokenWeights: map[string]float64{"foo": 1},
	})

	if score.UsabilityScore >= 80 {
		t.Fatalf("expected low usability score, got %.1f", score.UsabilityScore)
	}
	if !slices.Contains(score.Issues, "required_property_missing") {
		t.Fatalf("expected required_property_missing issue, got %v", score.Issues)
	}
	if !slices.Contains(score.Issues, "property_type_missing") {
		t.Fatalf("expected property_type_missing issue, got %v", score.Issues)
	}
}

func TestEvaluateImplicitCasesComputesTopK(t *testing.T) {
	t.Parallel()
	scenarios := []FoundationScenario{
		{
			ID:            "pass-case",
			Category:      "web",
			Intent:        "Download and inspect full page content from a URL",
			ExpectedTools: []string{"web_fetch"},
		},
		{
			ID:            "fail-case",
			Category:      "visual",
			Intent:        "Render architecture as a picture",
			ExpectedTools: []string{"diagram_render"},
		},
	}
	profiles := []foundationToolProfile{
		{
			Definition: ports.ToolDefinition{Name: "web_fetch"},
			TokenWeights: map[string]float64{
				"download": 3, "content": 3, "web": 2, "url": 2, "fetch": 4,
			},
		},
		{
			Definition: ports.ToolDefinition{Name: "web_search"},
			TokenWeights: map[string]float64{
				"search": 4, "web": 2, "query": 3,
			},
		},
	}

	summary := evaluateImplicitCases(scenarios, profiles, 2)
	if summary.TotalCases != 2 {
		t.Fatalf("unexpected total cases: %d", summary.TotalCases)
	}
	if summary.PassedCases != 1 {
		t.Fatalf("expected 1 passed case, got %d", summary.PassedCases)
	}
	if summary.FailedCases != 1 {
		t.Fatalf("expected 1 failed case, got %d", summary.FailedCases)
	}
}

func TestRunFoundationEvaluationEndToEnd(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	casePath := filepath.Join(tmp, "cases.yaml")
	caseYAML := `
version: "1"
name: "mini"
scenarios:
  - id: "one"
    category: "planning"
    intent: "Break this task into milestones and explicit checkpoints."
    expected_tools: ["plan"]
  - id: "two"
    category: "browser"
    intent: "Find selectors on a webpage and submit a form."
    expected_tools: ["browser_dom"]
`
	if err := os.WriteFile(casePath, []byte(caseYAML), 0644); err != nil {
		t.Fatalf("write case yaml: %v", err)
	}

	result, err := RunFoundationEvaluation(context.Background(), &FoundationEvaluationOptions{
		OutputDir:    filepath.Join(tmp, "out"),
		Mode:         "web",
		Preset:       "full",
		Toolset:      "default",
		CasesPath:    casePath,
		TopK:         3,
		ReportFormat: "json",
	})
	if err != nil {
		t.Fatalf("RunFoundationEvaluation error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result, got nil")
	}
	if result.Prompt.TotalPrompts == 0 {
		t.Fatalf("expected prompt scores")
	}
	if result.Tools.TotalTools == 0 {
		t.Fatalf("expected tool scores")
	}
	if result.Implicit.TotalCases != 2 {
		t.Fatalf("expected 2 cases, got %d", result.Implicit.TotalCases)
	}
	if len(result.ReportArtifacts) == 0 {
		t.Fatalf("expected report artifacts")
	}
	if _, err := os.Stat(result.ReportArtifacts[0].Path); err != nil {
		t.Fatalf("artifact not written: %v", err)
	}
}
