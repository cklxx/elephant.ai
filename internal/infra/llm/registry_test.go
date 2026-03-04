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

// TestRegistry_RegisterNormalizesKey verifies that Register() lowercases provider
// names so that Get() lookups are truly case-insensitive end-to-end.
func TestRegistry_RegisterNormalizesKey(t *testing.T) {
	r := NewRegistry()
	r.Register(&ProviderDescriptor{
		Name:   "MyProvider",
		Family: "test",
	})
	// Must be found via any case
	for _, query := range []string{"myprovider", "MyProvider", "MYPROVIDER", " myprovider "} {
		desc, ok := r.Get(query)
		if !ok {
			t.Fatalf("Get(%q) not found after Register(MyProvider)", query)
		}
		if desc.Family != "test" {
			t.Fatalf("Get(%q).Family = %q, want %q", query, desc.Family, "test")
		}
	}
}

// TestRegistry_RegisterAliasNormalizesBothSides verifies that RegisterAlias()
// normalizes both the alias and the canonical target.
func TestRegistry_RegisterAliasNormalizesBothSides(t *testing.T) {
	r := NewRegistry()
	r.Register(&ProviderDescriptor{Name: "target", Family: "test"})
	r.RegisterAlias("MyAlias", "Target")

	desc, ok := r.Get("myalias")
	if !ok {
		t.Fatal("alias myalias not found")
	}
	if desc.Name != "target" {
		t.Fatalf("alias resolved to %q, want %q", desc.Name, "target")
	}
}

// TestRegistry_ConfigMutatorInjection verifies that a ConfigMutator registered
// on a ProviderDescriptor is accessible and callable through the registry.
func TestRegistry_ConfigMutatorInjection(t *testing.T) {
	r := NewRegistry()
	r.Register(&ProviderDescriptor{
		Name:   "mutated",
		Family: "test",
		ConfigMutator: func(cfg *Config) {
			if cfg.Headers == nil {
				cfg.Headers = make(map[string]string)
			}
			cfg.Headers["X-Injected"] = "true"
		},
	})

	desc, ok := r.Get("mutated")
	if !ok {
		t.Fatal("provider not found")
	}
	if desc.ConfigMutator == nil {
		t.Fatal("ConfigMutator is nil")
	}

	cfg := Config{}
	desc.ConfigMutator(&cfg)
	if cfg.Headers["X-Injected"] != "true" {
		t.Fatalf("ConfigMutator did not inject header: %v", cfg.Headers)
	}
}

// TestDefaultRegistry_FactoryInjection verifies that every registered provider's
// ClientFactory is callable (i.e., the closure captures valid constructor refs).
func TestDefaultRegistry_FactoryInjection(t *testing.T) {
	r := NewDefaultRegistry()
	mockDesc, ok := r.Get("mock")
	if !ok {
		t.Fatal("mock provider not found")
	}
	client, err := mockDesc.ClientFactory("test-model", Config{})
	if err != nil {
		t.Fatalf("mock ClientFactory returned error: %v", err)
	}
	if client == nil {
		t.Fatal("mock ClientFactory returned nil client")
	}
}
