package larktesting

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// scenariosDir resolves the path to tests/scenarios/lark/ relative to the repo root.
func scenariosDir() string {
	// Walk up from this test file to the repo root.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	dir := filepath.Join(repoRoot, "tests", "scenarios", "lark")
	return dir
}

func TestScenarios(t *testing.T) {
	dir := scenariosDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("scenario directory %s does not exist", dir)
	}

	scenarios, err := LoadScenariosFromDir(dir)
	if err != nil {
		t.Fatalf("failed to load scenarios: %v", err)
	}
	if len(scenarios) == 0 {
		t.Skipf("no scenarios found in %s", dir)
	}

	runner := NewRunner(nil)
	for _, s := range scenarios {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			result := runner.Run(context.Background(), s)
			for _, tr := range result.Turns {
				for _, e := range tr.Errors {
					t.Errorf("turn %d: %s", tr.TurnIndex, e)
				}
			}
			if !result.Passed {
				t.Fatalf("scenario %q failed", s.Name)
			}
		})
	}
}
