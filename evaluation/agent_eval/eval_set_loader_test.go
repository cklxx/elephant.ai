package agent_eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildEvalSetFromDefinitionFiltersGeneralAgentTasks(t *testing.T) {
	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "tasks.json")
	defPath := filepath.Join(dir, "baseline.yaml")

	dataset := `[
  {
    "id": "t1",
    "title": "Medium planning task",
    "goal": "Plan a release",
    "context": "Release context",
    "input": "Input",
    "constraints": ["Include owners", "List risks"],
    "expected_output": "A markdown table with owners and risks.",
    "skills": ["planning"],
    "difficulty": "medium",
    "domain": "program_management",
    "surface": "web"
  },
  {
    "id": "t2",
    "title": "Hard security task",
    "goal": "Threat model",
    "context": "Security context",
    "input": "Input",
    "constraints": ["Rank top threats"],
    "expected_output": "A ranked list of threats.",
    "skills": ["security"],
    "difficulty": "hard",
    "domain": "security",
    "surface": "web"
  }
]`

	if err := os.WriteFile(datasetPath, []byte(dataset), 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	definition := `version: "1.0.0"
name: "baseline"
description: "Baseline eval set"
dataset:
  type: general_agent
  path: "` + datasetPath + `"
filters:
  difficulty: ["medium"]
composition_rules:
  - difficulty: "medium"
    min_count: 1
`

	if err := os.WriteFile(defPath, []byte(definition), 0o644); err != nil {
		t.Fatalf("write definition: %v", err)
	}

	def, err := LoadEvalSetDefinition(defPath)
	if err != nil {
		t.Fatalf("LoadEvalSetDefinition: %v", err)
	}

	set, instances, err := BuildEvalSetFromDefinition(def)
	if err != nil {
		t.Fatalf("BuildEvalSetFromDefinition: %v", err)
	}

	if set.Config.Name != "baseline" {
		t.Fatalf("expected name baseline, got %s", set.Config.Name)
	}
	if len(set.Tasks) != 1 {
		t.Fatalf("expected 1 task after filtering, got %d", len(set.Tasks))
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance after filtering, got %d", len(instances))
	}
	if set.Tasks[0].ID != "t1" {
		t.Fatalf("expected task t1, got %s", set.Tasks[0].ID)
	}
}

func TestBuildEvalSetFromDefinitionCompositionViolation(t *testing.T) {
	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "tasks.json")
	defPath := filepath.Join(dir, "baseline.yaml")

	dataset := `[
  {
    "id": "t1",
    "title": "Medium planning task",
    "goal": "Plan a release",
    "context": "Release context",
    "input": "Input",
    "constraints": ["Include owners"],
    "expected_output": "A markdown table with owners.",
    "skills": ["planning"],
    "difficulty": "medium",
    "domain": "program_management",
    "surface": "web"
  }
]`

	if err := os.WriteFile(datasetPath, []byte(dataset), 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	definition := `version: "1.0.0"
name: "baseline"
dataset:
  type: general_agent
  path: "` + datasetPath + `"
composition_rules:
  - difficulty: "hard"
    min_count: 1
`
	if err := os.WriteFile(defPath, []byte(definition), 0o644); err != nil {
		t.Fatalf("write definition: %v", err)
	}

	def, err := LoadEvalSetDefinition(defPath)
	if err != nil {
		t.Fatalf("LoadEvalSetDefinition: %v", err)
	}

	_, _, err = BuildEvalSetFromDefinition(def)
	if err == nil {
		t.Fatalf("expected composition violation error")
	}
}
