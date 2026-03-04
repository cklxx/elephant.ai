package taskfile

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTaskFileYAMLRoundTrip(t *testing.T) {
	original := &TaskFile{
		Version: "1",
		PlanID:  "plan-001",
		Defaults: TaskDefaults{
			AgentType:     "codex",
			ExecutionMode: "execute",
			Config:        map[string]string{"task_kind": "coding"},
		},
		Tasks: []TaskSpec{
			{
				ID:          "task-1",
				Description: "implement feature X",
				Prompt:      "Write code for feature X",
				DependsOn:   nil,
			},
			{
				ID:          "task-2",
				Description: "test feature X",
				Prompt:      "Write tests for feature X",
				DependsOn:   []string{"task-1"},
				Config: map[string]string{
					"verify":             "true",
					"retry_max_attempts": "3",
				},
				InheritContext: true,
			},
		},
		Metadata: map[string]string{"author": "test"},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTripped TaskFile
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if roundTripped.Version != original.Version {
		t.Errorf("version: got %q, want %q", roundTripped.Version, original.Version)
	}
	if roundTripped.PlanID != original.PlanID {
		t.Errorf("plan_id: got %q, want %q", roundTripped.PlanID, original.PlanID)
	}
	if len(roundTripped.Tasks) != len(original.Tasks) {
		t.Fatalf("tasks count: got %d, want %d", len(roundTripped.Tasks), len(original.Tasks))
	}
	if roundTripped.Tasks[1].DependsOn[0] != "task-1" {
		t.Errorf("depends_on: got %q, want %q", roundTripped.Tasks[1].DependsOn[0], "task-1")
	}
	if roundTripped.Tasks[1].Config["verify"] != "true" {
		t.Error("verify should be true after round-trip")
	}
	if roundTripped.Tasks[1].Config["retry_max_attempts"] != "3" {
		t.Error("retry_max_attempts should be 3 after round-trip")
	}
}
