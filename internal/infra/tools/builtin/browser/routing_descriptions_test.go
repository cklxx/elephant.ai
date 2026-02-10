package browser

import (
	"strings"
	"testing"
)

func TestBrowserDescriptionsExpressComputationBoundary(t *testing.T) {
	t.Parallel()

	actionDesc := NewBrowserAction(nil).Definition().Description
	if !strings.Contains(actionDesc, "deterministic computation/recalculation") {
		t.Fatalf("expected browser_action description to discourage compute routing, got %q", actionDesc)
	}
}
