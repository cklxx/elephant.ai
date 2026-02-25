package kernel

import (
	"strings"
	"testing"
	"time"
)

func TestRenderInitMarkdown_IncludesRuntimeAndAgents(t *testing.T) {
	snapshot := InitDocSnapshot{
		GeneratedAt:      time.Date(2026, 2, 12, 13, 30, 0, 0, time.UTC),
		KernelID:         "default",
		Schedule:         "*/10 * * * *",
		StateDir:         "/tmp/kernel",
		StatePath:        "/tmp/kernel/default/STATE.md",
		InitPath:         "/tmp/kernel/default/INIT.md",
		SystemPromptPath: "/tmp/kernel/default/SYSTEM_PROMPT.md",
		TimeoutSeconds:   900,
		LeaseSeconds:     1800,
		MaxConcurrent:    1,
		Channel:          "lark",
		UserID:           "ou_123",
		ChatID:           "oc_456",
		SeedState:        "# Kernel State\n## recent_actions\n(none yet)\n",
		Agents: []AgentConfig{
			{
				AgentID:  "autonomous-state-loop",
				Prompt:   "Current state:\n{STATE}\nDo one real action.",
				Priority: 5,
				Enabled:  true,
				Metadata: map[string]string{"purpose": "proactive-loop", "owner": "cklxx"},
			},
		},
	}

	rendered := RenderInitMarkdown(snapshot)
	expectedSnippets := []string{
		"# Kernel Initialization",
		"- kernel_id: default",
		"- state_path: /tmp/kernel/default/STATE.md",
		"## Seed State",
		"## Agents",
		"### 1. autonomous-state-loop",
		"- metadata:",
		"- purpose: proactive-loop",
		"- owner: cklxx",
		"Current state:",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected init markdown to include %q, got:\n%s", snippet, rendered)
		}
	}
}

func TestRenderSystemPromptMarkdown_ContainsPrompt(t *testing.T) {
	prompt := "You are elephant.ai kernel coordinator.\nFollow safety rules."
	rendered := RenderSystemPromptMarkdown(prompt, time.Date(2026, 2, 12, 13, 35, 0, 0, time.UTC))

	if !strings.Contains(rendered, "# Kernel System Prompt") {
		t.Fatalf("missing header: %s", rendered)
	}
	if !strings.Contains(rendered, "- source: AgentCoordinator.GetSystemPrompt()") {
		t.Fatalf("missing source line: %s", rendered)
	}
	if !strings.Contains(rendered, prompt) {
		t.Fatalf("missing prompt content: %s", rendered)
	}
}
