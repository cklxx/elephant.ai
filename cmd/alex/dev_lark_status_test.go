package main

import (
	"strings"
	"testing"

	"alex/internal/devops/supervisor"
)

func TestFormatLarkComponentStatusIncludesRunsWhenPresent(t *testing.T) {
	comp := supervisor.ComponentStatus{
		PID:         73499,
		Health:      "healthy",
		DeployedSHA: "0113334faaaaaaaa",
		RunsWindow:  3,
	}

	got := formatLarkComponentStatus("main", comp, "f08a3be4")

	if !strings.Contains(got, "healthy  pid=73499") {
		t.Fatalf("expected health/pid segment in %q", got)
	}
	if !strings.Contains(got, "sha=0113334f (HEAD: f08a3be4)") {
		t.Fatalf("expected sha/head segment in %q", got)
	}
	if !strings.Contains(got, "runs=3") {
		t.Fatalf("expected runs in %q", got)
	}
}

func TestFormatLarkComponentStatusOmitsRunsWhenZero(t *testing.T) {
	comp := supervisor.ComponentStatus{
		PID:    69314,
		Health: "healthy",
	}

	got := formatLarkComponentStatus("main", comp, "f08a3be4")

	if strings.Contains(got, "runs=") {
		t.Fatalf("expected zero-run status to omit runs, got %q", got)
	}
}
