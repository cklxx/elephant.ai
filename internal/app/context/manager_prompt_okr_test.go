package context

import "testing"

func TestBuildOKRSection_Empty(t *testing.T) {
	for _, input := range []string{"", "  ", "\n\t"} {
		if got := buildOKRSection(input); got != "" {
			t.Errorf("buildOKRSection(%q) = %q; want empty", input, got)
		}
	}
}

func TestBuildOKRSection_NonEmpty(t *testing.T) {
	input := "**Active OKR Goals:**\n\n### Q1-2026\n- KR1: 50/100"
	got := buildOKRSection(input)
	if got == "" {
		t.Fatal("expected non-empty OKR section")
	}
	if got[:12] != "# OKR Goals\n" {
		t.Errorf("expected section to start with '# OKR Goals\\n', got %q", got[:20])
	}
	if len(got) <= len("# OKR Goals\n") {
		t.Error("expected content after heading")
	}
}

func TestComposeSystemPrompt_IncludesOKRSection(t *testing.T) {
	input := systemPromptInput{
		OKRContext: "**Active OKR Goals:**\n\n### Q1\n- KR1",
	}
	prompt := composeSystemPrompt(input)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsSubstring(prompt, "# OKR Goals") {
		t.Error("expected system prompt to contain OKR Goals section")
	}
	if !containsSubstring(prompt, "### Q1") {
		t.Error("expected system prompt to contain goal content")
	}
}

func TestComposeSystemPrompt_NoOKRSection_WhenEmpty(t *testing.T) {
	input := systemPromptInput{
		OKRContext: "",
	}
	prompt := composeSystemPrompt(input)
	if containsSubstring(prompt, "# OKR Goals") {
		t.Error("did not expect OKR Goals section in prompt when context is empty")
	}
}

func TestComposeSystemPrompt_IncludesHabitStewardshipSection(t *testing.T) {
	prompt := composeSystemPrompt(systemPromptInput{})
	if !containsSubstring(prompt, "# Habit Stewardship") {
		t.Fatalf("expected system prompt to include habit stewardship section, got %q", prompt)
	}
	if !containsSubstring(prompt, "stable user habits") {
		t.Fatalf("expected system prompt to include habit capture guidance, got %q", prompt)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
