package browser

import (
	"strings"
	"testing"
)

func TestBrowserDescriptionsExpressComputationAndVisualBoundaries(t *testing.T) {
	t.Parallel()

	actionDesc := NewBrowserAction(nil).Definition().Description
	if !strings.Contains(actionDesc, "deterministic computation/recalculation") {
		t.Fatalf("expected browser_action description to discourage compute routing, got %q", actionDesc)
	}

	screenshotDesc := NewBrowserScreenshot(nil).Definition().Description
	if !strings.Contains(screenshotDesc, "visual browser page image") || !strings.Contains(screenshotDesc, "semantic text retrieval") {
		t.Fatalf("expected browser_screenshot description to express visual-only routing, got %q", screenshotDesc)
	}
}
