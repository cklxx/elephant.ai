package context

import (
	"strings"
	"testing"
)

func TestBuildToolRoutingSectionIncludesDeterministicAndMemoryBoundaries(t *testing.T) {
	t.Parallel()

	section := buildToolRoutingSection()
	// Check for compressed meta-rules key phrases
	for _, snippet := range []string{
		"Exploration first",
		"Memory hierarchy",
		"Tool selection patterns",
		"Autonomous loops",
		"Safety",
		"user delegation",
		"read_file for workspace files",
		"execute_code",
		"bash as fallback",
		"inject runtime facts",
		"Never expose secrets",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected tool routing section to contain %q", snippet)
		}
	}
}
