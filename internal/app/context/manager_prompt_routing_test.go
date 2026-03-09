package context

import (
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestBuildToolRoutingSectionIncludesDeterministicAndMemoryBoundaries(t *testing.T) {
	t.Parallel()

	section := buildToolRoutingSection()
	// Check for decision tree + ALWAYS/NEVER key phrases
	for _, snippet := range []string{
		"task_has_explicit_operation",
		"read_only_inspection",
		"user_delegates",
		"needs_human_gate",
		"ALWAYS exhaust deterministic tools",
		"ALWAYS use read_file for workspace",
		"ALWAYS inject runtime facts",
		"ALWAYS probe capabilities",
		"NEVER expose secrets",
		"NEVER use ask_user for explicit",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected tool routing section to contain %q", snippet)
		}
	}
}

func TestBuildSafetySectionIncludesNeverRules(t *testing.T) {
	t.Parallel()
	section := buildSafetySection()
	for _, snippet := range []string{
		"NEVER bypass approval gates",
		"NEVER fabricate tool outputs",
		"NEVER execute irreversible actions",
		"NEVER include secrets",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected safety section to contain %q", snippet)
		}
	}
}

func TestBuildReasoningSectionIncludesNeverRules(t *testing.T) {
	t.Parallel()
	section := buildReasoningSection()
	for _, snippet := range []string{
		"NEVER switch reasoning verbosity",
		"NEVER emit internal chain-of-thought",
		"NEVER suppress reasoning traces",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected reasoning section to contain %q", snippet)
		}
	}
}

func TestBuildToolingSectionIncludesAlwaysNeverRules(t *testing.T) {
	t.Parallel()
	section := buildToolingSection(nil)
	for _, snippet := range []string{
		"ALWAYS inspect tool definitions",
		"NEVER assume a tool exists",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected tooling section to contain %q", snippet)
		}
	}
}

func TestBuildHabitStewardshipSectionIncludesNeverRules(t *testing.T) {
	t.Parallel()
	section := buildHabitStewardshipSection()
	for _, snippet := range []string{
		"NEVER invent or extrapolate habits",
		"NEVER record habits that conflict",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected habit stewardship section to contain %q", snippet)
		}
	}
}

func TestBuildWorkspaceSectionIncludesNeverRule(t *testing.T) {
	t.Parallel()
	section := buildWorkspaceSection()
	if !strings.Contains(section, "NEVER write generated files into the repository") {
		t.Fatal("expected workspace section to contain NEVER rule about generated files")
	}
}

func TestBuildSelfUpdateSectionIncludesNeverRule(t *testing.T) {
	t.Parallel()
	section := buildSelfUpdateSection()
	if !strings.Contains(section, "NEVER run update.run without explicit user request") {
		t.Fatal("expected self-update section to contain NEVER rule")
	}
}

func TestBuildChannelFormattingSectionWithHintPassesThrough(t *testing.T) {
	t.Parallel()
	hint := "# Reply Formatting (Lark Channel)\nCurrent reply channel is Lark; Lark text messages do not render Markdown.\nFor long-running or parallel execution, proactively send intermediate checkpoints via shell_exec + skills/feishu-cli/run.py so users can see progress.\nDo not use Markdown syntax"
	section := buildChannelFormattingSection(hint)
	for _, snippet := range []string{
		"Current reply channel is Lark",
		"send intermediate checkpoints via shell_exec + skills/feishu-cli/run.py",
		"Do not use Markdown syntax",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected channel formatting section to contain %q, got %q", snippet, section)
		}
	}
}

func TestBuildChannelFormattingSectionEmptyHintEmpty(t *testing.T) {
	t.Parallel()
	if section := buildChannelFormattingSection(""); section != "" {
		t.Fatalf("expected empty hint to produce empty section, got %q", section)
	}
	if section := buildChannelFormattingSection("   "); section != "" {
		t.Fatalf("expected whitespace-only hint to produce empty section, got %q", section)
	}
}

func TestBuildPoliciesSectionRendersCommunicationStyle(t *testing.T) {
	t.Parallel()
	policies := []agent.PolicyRule{{
		ID:              "Communication Style",
		HardConstraints: []string{"Brevity is law.", "NEVER sacrifice clarity for cleverness."},
		SoftPreferences: []string{"Plain language only.", "A dash of wit."},
	}}
	section := buildPoliciesSection(policies)
	for _, snippet := range []string{
		"# Guardrails & Policies",
		"Communication Style:",
		"Brevity is law.",
		"NEVER sacrifice clarity for cleverness.",
		"Plain language only.",
		"A dash of wit.",
	} {
		if !strings.Contains(section, snippet) {
			t.Fatalf("expected policies section to contain %q", snippet)
		}
	}
}
