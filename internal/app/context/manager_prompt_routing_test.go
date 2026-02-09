package context

import (
	"strings"
	"testing"
)

func TestBuildToolRoutingSectionIncludesDeterministicAndMemoryBoundaries(t *testing.T) {
	t.Parallel()

	section := buildToolRoutingSection()
	for _, snippet := range []string{
		"Use read_file for repository/workspace files and proof/context windows; use memory_search/memory_get only for persistent memory notes.",
		"Use execute_code for deterministic computation/recalculation/metric checks, not browser_action or lark_calendar_query.",
		"Use scheduler_list_jobs for recurring scheduler inventory and scheduler_delete_job only for retiring scheduler jobs.",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected tool routing section to contain %q", snippet)
		}
	}
}

