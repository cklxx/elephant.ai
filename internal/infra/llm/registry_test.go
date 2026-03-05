package llm

import (
	"testing"
)

func TestDefaultRegistry_AllCanonicalProviders(t *testing.T) {
	r := NewDefaultRegistry()
	for _, name := range []string{
		"openai", "openrouter", "deepseek", "kimi", "glm", "minimax",
		"openai-responses", "codex",
		"anthropic",
		"llama.cpp",
		"mock",
	} {
		desc, ok := r.Get(name)
		if !ok {
			t.Fatalf("provider %q not found in default registry", name)
		}
		if desc.Name != name {
			t.Fatalf("expected Name=%q, got %q", name, desc.Name)
		}
		if desc.ClientFactory == nil {
			t.Fatalf("provider %q has nil ClientFactory", name)
		}
	}
}

func TestDefaultRegistry_Aliases(t *testing.T) {
	r := NewDefaultRegistry()
	tests := []struct {
		alias     string
		canonical string
	}{
		{"responses", "openai-responses"},
		{"claude", "anthropic"},
		{"llama-cpp", "llama.cpp"},
		{"llamacpp", "llama.cpp"},
	}
	for _, tc := range tests {
		desc, ok := r.Get(tc.alias)
		if !ok {
			t.Fatalf("alias %q not found", tc.alias)
		}
		if desc.Name != tc.canonical {
			t.Fatalf("alias %q resolved to %q, want %q", tc.alias, desc.Name, tc.canonical)
		}
	}
}

func TestDefaultRegistry_CaseInsensitive(t *testing.T) {
	r := NewDefaultRegistry()
	for _, name := range []string{"OpenAI", "ANTHROPIC", "Claude", " kimi "} {
		if _, ok := r.Get(name); !ok {
			t.Fatalf("case-insensitive lookup failed for %q", name)
		}
	}
}

func TestDefaultRegistry_UnknownProvider(t *testing.T) {
	r := NewDefaultRegistry()
	if _, ok := r.Get("nonexistent"); ok {
		t.Fatal("expected unknown provider to return false")
	}
}

func TestDefaultRegistry_List(t *testing.T) {
	r := NewDefaultRegistry()
	list := r.List()
	if len(list) != 11 {
		t.Fatalf("expected 11 providers, got %d", len(list))
	}
	// Verify sorted
	for i := 1; i < len(list); i++ {
		if list[i].Name < list[i-1].Name {
			t.Fatalf("list not sorted: %q before %q", list[i-1].Name, list[i].Name)
		}
	}
}

func TestDefaultRegistry_Families(t *testing.T) {
	r := NewDefaultRegistry()
	families := map[string]string{
		"openai":           "openai-compat",
		"kimi":             "openai-compat",
		"deepseek":         "openai-compat",
		"openai-responses": "codex-compat",
		"codex":            "codex-compat",
		"anthropic":        "anthropic",
		"llama.cpp":        "llamacpp",
		"mock":             "mock",
	}
	for provider, wantFamily := range families {
		desc, ok := r.Get(provider)
		if !ok {
			t.Fatalf("provider %q not found", provider)
		}
		if desc.Family != wantFamily {
			t.Fatalf("provider %q: family=%q, want %q", provider, desc.Family, wantFamily)
		}
	}
}
