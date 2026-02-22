package context

import (
	"strings"
	"testing"
)

func TestBuildToolRoutingSectionIncludesDeterministicAndMemoryBoundaries(t *testing.T) {
	t.Parallel()

	section := buildToolRoutingSection()
	// Check for decision tree + ALWAYS/NEVER key phrases
	for _, snippet := range []string{
		"task_has_explicit_operation",
		"read_only_inspection",
		"memory_search/memory_get",
		"user_delegates",
		"needs_human_gate",
		"ALWAYS exhaust deterministic tools",
		"ALWAYS use read_file for workspace",
		"execute_code",
		"ALWAYS inject runtime facts",
		"ALWAYS probe capabilities",
		"NEVER expose secrets",
		"NEVER use clarify for explicit",
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
