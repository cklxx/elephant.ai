package bootstrap

import (
	"fmt"
	"testing"

	"alex/internal/shared/logging"
)

func TestRunStagesFailsOnRequired(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	degraded := NewDegradedComponents()

	stages := []BootstrapStage{
		{Name: "ok", Required: true, Init: func() error { return nil }},
		{Name: "fail", Required: true, Init: func() error { return fmt.Errorf("boom") }},
		{Name: "unreached", Required: true, Init: func() error {
			t.Fatal("should not be reached")
			return nil
		}},
	}

	err := RunStages(stages, degraded, logger)
	if err == nil {
		t.Fatal("expected error from required stage")
	}
	if degraded.IsEmpty() == false {
		t.Fatal("no optional stages should have been recorded")
	}
}

func TestRunStagesRecordsDegradedForOptional(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	degraded := NewDegradedComponents()

	stages := []BootstrapStage{
		{Name: "ok", Required: true, Init: func() error { return nil }},
		{Name: "optional-fail", Required: false, Init: func() error { return fmt.Errorf("oops") }},
		{Name: "still-runs", Required: true, Init: func() error { return nil }},
	}

	err := RunStages(stages, degraded, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := degraded.Map()
	if len(m) != 1 {
		t.Fatalf("expected 1 degraded component, got %d", len(m))
	}
	if _, ok := m["optional-fail"]; !ok {
		t.Fatal("expected 'optional-fail' in degraded map")
	}
}

func TestRunStagesContinuesAfterOptionalFailure(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	degraded := NewDegradedComponents()
	var reached bool

	stages := []BootstrapStage{
		{Name: "opt-a", Required: false, Init: func() error { return fmt.Errorf("fail-a") }},
		{Name: "opt-b", Required: false, Init: func() error { return fmt.Errorf("fail-b") }},
		{Name: "required", Required: true, Init: func() error { reached = true; return nil }},
	}

	err := RunStages(stages, degraded, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reached {
		t.Fatal("required stage was not reached")
	}
	m := degraded.Map()
	if len(m) != 2 {
		t.Fatalf("expected 2 degraded, got %d", len(m))
	}
}
