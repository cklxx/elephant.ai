package sandbox

import (
	"strings"
	"testing"
)

func TestSandboxBrowserDescriptionsExpressReadVsActionBoundary(t *testing.T) {
	t.Parallel()

	cfg := SandboxConfig{}
	infoDesc := NewSandboxBrowserInfo(cfg).Definition().Description
	if !strings.Contains(infoDesc, "read-only") {
		t.Fatalf("expected browser_info description to mention read-only usage, got %q", infoDesc)
	}

	actionDesc := NewSandboxBrowser(cfg).Definition().Description
	if !strings.Contains(actionDesc, "only to inspect URL/title/session metadata") {
		t.Fatalf("expected browser_action description to discourage metadata-only use, got %q", actionDesc)
	}
}

func TestSandboxFileDescriptionsExpressReplaceVsDiscoveryBoundary(t *testing.T) {
	t.Parallel()

	cfg := SandboxConfig{}
	replaceDesc := NewSandboxFileReplace(cfg).Definition().Description
	if !strings.Contains(replaceDesc, "in-place edits") || !strings.Contains(replaceDesc, "listing/inventory") {
		t.Fatalf("expected replace_in_file description to mention in-place-only routing, got %q", replaceDesc)
	}

	searchDesc := NewSandboxFileSearch(cfg).Definition().Description
	if !strings.Contains(searchDesc, "path/name discovery") {
		t.Fatalf("expected search_file description to mention discovery boundary, got %q", searchDesc)
	}
}

