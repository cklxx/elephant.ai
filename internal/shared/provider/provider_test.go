package provider

import "testing"

func TestFamily(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"anthropic", FamilyAnthropic},
		{"claude", FamilyAnthropic},
		{"codex", FamilyCodex},
		{"openai-responses", FamilyCodex},
		{"responses", FamilyCodex},
		{"openai", FamilyOpenAI},
		{"kimi", FamilyOpenAI},
		{"deepseek", FamilyOpenAI},
		{"glm", FamilyOpenAI},
		{"minimax", FamilyOpenAI},
		{"openrouter", FamilyOpenAI},
		{"llama.cpp", FamilyLlamaCpp},
		{"llama-cpp", FamilyLlamaCpp},
		{"llamacpp", FamilyLlamaCpp},
		{"mock", FamilyMock},
		{"unknown", "unknown"},
		{"  OpenAI  ", FamilyOpenAI},
	}
	for _, tc := range tests {
		if got := Family(tc.input); got != tc.want {
			t.Errorf("Family(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestUsesCLIAuth(t *testing.T) {
	trueProviders := []string{"anthropic", "claude", "codex", "openai-responses", "responses", "auto", "cli"}
	for _, p := range trueProviders {
		if !UsesCLIAuth(p) {
			t.Errorf("UsesCLIAuth(%q) = false, want true", p)
		}
	}
	falseProviders := []string{"openai", "kimi", "deepseek", "glm", "minimax", "mock", "llama.cpp"}
	for _, p := range falseProviders {
		if UsesCLIAuth(p) {
			t.Errorf("UsesCLIAuth(%q) = true, want false", p)
		}
	}
}

func TestRequiresAPIKey(t *testing.T) {
	falseProviders := []string{"", "mock", "llama.cpp", "llamacpp", "llama-cpp", "ollama"}
	for _, p := range falseProviders {
		if RequiresAPIKey(p) {
			t.Errorf("RequiresAPIKey(%q) = true, want false", p)
		}
	}
	trueProviders := []string{"openai", "anthropic", "kimi", "codex"}
	for _, p := range trueProviders {
		if !RequiresAPIKey(p) {
			t.Errorf("RequiresAPIKey(%q) = false, want true", p)
		}
	}
}
