package preparation

import (
	"strings"
	"testing"
)

func TestDefaultSystemPromptIncludesRoutingBoundaries(t *testing.T) {
	t.Parallel()

	for _, snippet := range []string{
		"do not use clarify for explicit operational asks",
		"exhaust safe deterministic attempts before asking questions",
		"inspect injected memory snapshot and thread context first",
		"ask one minimal blocking question only then",
		"search/install suitable skills or tools from trusted sources",
		"explicit approval/consent/manual gates",
		"low-risk read-only inspection asks",
		"do not ask for reconfirmation",
		"Treat explicit user delegation signals (\"you decide\", \"anything works\", \"use your judgment\") as authorization for low-risk reversible actions",
		"For Lark/Feishu operations, run local skill CLIs via shell_exec",
		"/tmp as the default location for temporary/generated files",
		"artifacts_list for inventory and artifacts_write for creating/updating durable outputs",
	} {
		if !strings.Contains(DefaultSystemPrompt, snippet) {
			t.Fatalf("expected DefaultSystemPrompt to contain %q", snippet)
		}
	}
}
