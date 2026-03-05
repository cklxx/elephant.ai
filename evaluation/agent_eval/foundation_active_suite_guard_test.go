package agent_eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveSuitesOnlyReferenceActiveTools(t *testing.T) {
	t.Parallel()
	repoRoot := mustResolveRepoRoot(t)

	activeTools := map[string]struct{}{
		"ask_user":        {},
		"channel":         {},
		"plan":            {},
		"read_file":       {},
		"replace_in_file": {},
		"shell_exec":      {},
		"skills":          {},
		"web_search":      {},
		"write_file":      {},
	}

	suitePaths := []string{
		filepath.Join(repoRoot, "evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml"),
		filepath.Join(repoRoot, "evaluation/agent_eval/datasets/foundation_eval_suite_motivation_aware.yaml"),
	}

	for _, suitePath := range suitePaths {
		suite, err := LoadFoundationSuiteSet(suitePath)
		if err != nil {
			t.Fatalf("load suite %s: %v", suitePath, err)
		}
		for _, collection := range suite.Collections {
			casePath := collection.CasesPath
			if !filepath.IsAbs(casePath) {
				casePath = filepath.Join(repoRoot, casePath)
			}
			caseSet, err := LoadFoundationCaseSet(casePath)
			if err != nil {
				t.Fatalf("load cases %s: %v", casePath, err)
			}
			for _, scenario := range caseSet.Scenarios {
				for _, tool := range scenario.ExpectedTools {
					if _, ok := activeTools[tool]; !ok {
						t.Fatalf("suite %s collection %s scenario %s references unavailable tool %s", suitePath, collection.ID, scenario.ID, tool)
					}
				}
			}
		}
	}
}

func mustResolveRepoRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	clean := filepath.Clean(cwd)
	evalSuffix := filepath.Clean(filepath.FromSlash("evaluation/agent_eval"))
	if strings.HasSuffix(clean, evalSuffix) {
		return filepath.Clean(filepath.Join(clean, "..", ".."))
	}
	return clean
}
