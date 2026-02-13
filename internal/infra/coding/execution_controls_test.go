package coding

import "testing"

func TestApplyExecutionControls_CodexPlanForcesReadOnly(t *testing.T) {
	cfg := applyExecutionControls("codex", "plan", "full", map[string]string{})
	if cfg["execution_mode"] != "plan" {
		t.Fatalf("expected execution_mode=plan, got %q", cfg["execution_mode"])
	}
	if cfg["sandbox"] != "read-only" {
		t.Fatalf("expected sandbox=read-only, got %q", cfg["sandbox"])
	}
	if cfg["approval_policy"] != "never" {
		t.Fatalf("expected approval_policy=never, got %q", cfg["approval_policy"])
	}
}

func TestApplyExecutionControls_ClaudeFullDefaultsAutonomous(t *testing.T) {
	cfg := applyExecutionControls("claude_code", "execute", "full", map[string]string{})
	if cfg["mode"] != "autonomous" {
		t.Fatalf("expected mode=autonomous, got %q", cfg["mode"])
	}
	if cfg["allowed_tools"] != "*" {
		t.Fatalf("expected allowed_tools=*, got %q", cfg["allowed_tools"])
	}
}

func TestBuildPlanOnlyPromptAppendsInstruction(t *testing.T) {
	out := buildPlanOnlyPrompt("Implement auth changes")
	if out == "" {
		t.Fatal("expected non-empty prompt")
	}
	if out == "Implement auth changes" {
		t.Fatalf("expected prompt enrichment, got %q", out)
	}
}
