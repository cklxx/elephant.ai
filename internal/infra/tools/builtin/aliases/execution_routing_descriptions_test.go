package aliases

import (
	"strings"
	"testing"

	"alex/internal/infra/tools/builtin/shared"
)

func TestExecuteCodeDescriptionExpressesDeterministicBoundary(t *testing.T) {
	t.Parallel()

	desc := NewExecuteCode(shared.ShellToolConfig{}).Definition().Description
	if !strings.Contains(desc, "deterministic computation/recalculation/invariant checks") || !strings.Contains(desc, "Do not use for browser interaction or calendar querying") {
		t.Fatalf("expected execute_code description to express deterministic boundary, got %q", desc)
	}
}

