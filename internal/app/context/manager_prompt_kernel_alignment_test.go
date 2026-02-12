package context

import (
	"strings"
	"testing"
)

func TestComposeSystemPrompt_IncludesKernelAlignmentSection(t *testing.T) {
	prompt := composeSystemPrompt(systemPromptInput{
		KernelAlignmentContext: "Service user: cklxx\n## Kernel Objective\n- keep loop healthy",
	})
	if !strings.Contains(prompt, "# Kernel Alignment") {
		t.Fatalf("expected kernel alignment section, got: %q", prompt)
	}
	if !strings.Contains(prompt, "Service user: cklxx") {
		t.Fatalf("expected kernel alignment content, got: %q", prompt)
	}
}
