package preparation

import (
	"strings"
	"testing"
)

func TestDefaultSystemPromptIncludesRoutingBoundaries(t *testing.T) {
	t.Parallel()

	for _, snippet := range []string{
		"do not use clarify for explicit operational asks",
		"explicit approval/consent/manual gates",
		"browser_info for read-only browser state",
		"lark_chat_history for prior thread context",
		"artifacts_list for inventory and artifacts_write for creating/updating durable outputs",
	} {
		if !strings.Contains(DefaultSystemPrompt, snippet) {
			t.Fatalf("expected DefaultSystemPrompt to contain %q", snippet)
		}
	}
}

