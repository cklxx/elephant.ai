package agent_eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveSuitesOnlyReferenceActiveTools(t *testing.T) {
	t.Parallel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if strings.HasSuffix(filepath.ToSlash(cwd), "/evaluation/agent_eval") {
		if err := os.Chdir(filepath.Clean(filepath.Join(cwd, "..", ".."))); err != nil {
			t.Fatalf("chdir repo root: %v", err)
		}
		defer func() { _ = os.Chdir(cwd) }()
	}

	activeTools := map[string]struct{}{
		"browser_action": {},
		"channel":        {},
		"clarify":        {},
		"execute_code":   {},
		"memory_get":     {},
		"memory_search":  {},
		"plan":           {},
		"read_file":      {},
		"replace_in_file": {},
		"request_user":   {},
		"shell_exec":     {},
		"skills":         {},
		"web_search":     {},
		"write_file":     {},
	}

	suitePaths := []string{
		"evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml",
		"evaluation/agent_eval/datasets/foundation_eval_suite_active_tools_systematic_hard.yaml",
	}

	for _, suitePath := range suitePaths {
		suite, err := LoadFoundationSuiteSet(suitePath)
		if err != nil {
			t.Fatalf("load suite %s: %v", suitePath, err)
		}
		for _, collection := range suite.Collections {
			caseSet, err := LoadFoundationCaseSet(collection.CasesPath)
			if err != nil {
				trimmed := strings.TrimPrefix(collection.CasesPath, "evaluation/agent_eval/")
				caseSet, err = LoadFoundationCaseSet(trimmed)
			}
			if err != nil {
				t.Fatalf("load cases %s: %v", collection.CasesPath, err)
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
