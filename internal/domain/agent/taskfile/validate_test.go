package taskfile

import (
	"strings"
	"testing"
)

func TestValidate_HappyPath(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "plan-ok",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
		},
	}
	if err := validate(tf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_NilTaskFile(t *testing.T) {
	err := validate(nil)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	tf := &TaskFile{Tasks: []TaskSpec{{ID: "a", Prompt: "p"}}}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected version error, got: %v", err)
	}
}

func TestValidate_NoTasks(t *testing.T) {
	tf := &TaskFile{Version: "1"}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "at least one task") {
		t.Fatalf("expected no tasks error, got: %v", err)
	}
}

func TestValidate_MissingID(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks:   []TaskSpec{{Prompt: "p"}},
	}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("expected id error, got: %v", err)
	}
}

func TestValidate_MissingPrompt(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks:   []TaskSpec{{ID: "a"}},
	}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected prompt error, got: %v", err)
	}
}

func TestValidate_DuplicateID(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "p"},
			{ID: "a", Prompt: "q"},
		},
	}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}
}

func TestValidate_SelfDependency(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks:   []TaskSpec{{ID: "a", Prompt: "p", DependsOn: []string{"a"}}},
	}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "depends on itself") {
		t.Fatalf("expected self-dependency error, got: %v", err)
	}
}

func TestValidate_UnknownDependency(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks:   []TaskSpec{{ID: "a", Prompt: "p", DependsOn: []string{"nonexistent"}}},
	}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "unknown task") {
		t.Fatalf("expected unknown dep error, got: %v", err)
	}
}

func TestValidate_CycleDetection(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "p", DependsOn: []string{"b"}},
			{ID: "b", Prompt: "q", DependsOn: []string{"a"}},
		},
	}
	err := validate(tf)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got: %v", err)
	}
}
