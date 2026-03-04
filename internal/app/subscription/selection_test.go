package subscription

import (
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestResolveSelectionForCodexCLI(t *testing.T) {
	resolver := NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.CLICredentials{
			Codex: runtimeconfig.CLICredential{
				Provider:  "codex",
				APIKey:    "tok-abc",
				AccountID: "acct-1",
				BaseURL:   "https://chatgpt.com/backend-api/codex",
				Source:    runtimeconfig.SourceCodexCLI,
			},
		}
	})

	selection := Selection{Mode: "cli", Provider: "codex", Model: "gpt-5.2-codex", Source: "codex_cli"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatalf("expected selection to resolve")
	}
	if resolved.Provider != "codex" || resolved.Model != "gpt-5.2-codex" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	if resolved.APIKey != "tok-abc" || resolved.BaseURL == "" {
		t.Fatalf("expected api key + base url")
	}
	if resolved.Headers["ChatGPT-Account-Id"] != "acct-1" {
		t.Fatalf("expected account header")
	}
}

func TestResolveSelectionIgnoresYamlMode(t *testing.T) {
	resolver := NewSelectionResolver(func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} })
	if _, ok := resolver.Resolve(Selection{Mode: "yaml"}); ok {
		t.Fatalf("expected yaml selection to be ignored")
	}
}

func TestResolveSelectionCodexEmptyCreds(t *testing.T) {
	// When CLI credentials are empty (e.g. expired token), Resolve should
	// still return a partial selection with Pinned=true and fallback BaseURL
	// so that the downstream credential refresher can fill in the API key.
	resolver := NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.CLICredentials{} // all empty
	})

	selection := Selection{Mode: "cli", Provider: "codex", Model: "gpt-5.2-codex"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatalf("expected selection to resolve even with empty creds")
	}
	if resolved.Provider != "codex" || resolved.Model != "gpt-5.2-codex" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	if resolved.APIKey != "" {
		t.Fatalf("expected empty api key, got %q", resolved.APIKey)
	}
	if resolved.BaseURL != "https://chatgpt.com/backend-api/codex" {
		t.Fatalf("expected fallback base url, got %q", resolved.BaseURL)
	}
	if !resolved.Pinned {
		t.Fatal("expected Pinned=true")
	}
}

func TestResolveSelectionClaudeEmptyCreds(t *testing.T) {
	resolver := NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.CLICredentials{}
	})

	selection := Selection{Mode: "cli", Provider: "anthropic", Model: "claude-sonnet-4-20250514"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatalf("expected selection to resolve even with empty creds")
	}
	if resolved.Provider != "anthropic" || resolved.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	if resolved.APIKey != "" {
		t.Fatalf("expected empty api key, got %q", resolved.APIKey)
	}
	if resolved.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("expected fallback base url, got %q", resolved.BaseURL)
	}
	if !resolved.Pinned {
		t.Fatal("expected Pinned=true")
	}
}

func TestResolveSelectionForLlamaServer(t *testing.T) {
	t.Setenv("LLAMA_SERVER_BASE_URL", "http://127.0.0.1:8082/v1")
	resolver := NewSelectionResolver(func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} })

	selection := Selection{Mode: "cli", Provider: "llama_server", Model: "local-llama", Source: "llama_server"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatalf("expected selection to resolve")
	}
	if resolved.Provider != "llama.cpp" || resolved.Model != "local-llama" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	if resolved.BaseURL != "http://127.0.0.1:8082/v1" {
		t.Fatalf("expected base url from env, got %q", resolved.BaseURL)
	}
	if resolved.Source != "llama_server" {
		t.Fatalf("expected llama_server source, got %q", resolved.Source)
	}
}

func TestResolveSelectionForKimiViaEnv(t *testing.T) {
	t.Parallel()
	mockEnv := map[string]string{"KIMI_API_KEY": "sk-kimi-test"}
	resolver := NewSelectionResolver(
		func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} },
		WithEnvLookup(func(key string) (string, bool) {
			v, ok := mockEnv[key]
			return v, ok
		}),
	)

	selection := Selection{Mode: "cli", Provider: "kimi", Model: "kimi-for-coding"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatal("expected kimi selection to resolve")
	}
	if resolved.Provider != "kimi" || resolved.Model != "kimi-for-coding" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	if resolved.APIKey != "sk-kimi-test" {
		t.Fatalf("expected api key sk-kimi-test, got %q", resolved.APIKey)
	}
	if resolved.BaseURL != "https://api.kimi.com/coding/v1" {
		t.Fatalf("expected kimi base url, got %q", resolved.BaseURL)
	}
	if resolved.Source != "KIMI_API_KEY" {
		t.Fatalf("expected KIMI_API_KEY source, got %q", resolved.Source)
	}
	if !resolved.Pinned {
		t.Fatal("expected Pinned=true")
	}
}

func TestResolveSelectionForOpenAIViaEnv(t *testing.T) {
	t.Parallel()
	mockEnv := map[string]string{"OPENAI_API_KEY": "sk-oai-test"}
	resolver := NewSelectionResolver(
		func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} },
		WithEnvLookup(func(key string) (string, bool) {
			v, ok := mockEnv[key]
			return v, ok
		}),
	)

	selection := Selection{Mode: "cli", Provider: "openai", Model: "gpt-5-mini"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatal("expected openai selection to resolve")
	}
	if resolved.Provider != "openai" || resolved.Model != "gpt-5-mini" {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	if resolved.APIKey != "sk-oai-test" {
		t.Fatalf("expected api key, got %q", resolved.APIKey)
	}
	if resolved.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected openai base url, got %q", resolved.BaseURL)
	}
}

func TestResolveSelectionGenericPresetNoEnv(t *testing.T) {
	t.Parallel()
	// When no env vars are set, should still return partial resolution with
	// preset default base URL for known providers.
	resolver := NewSelectionResolver(
		func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} },
		WithEnvLookup(func(string) (string, bool) { return "", false }),
	)

	selection := Selection{Mode: "cli", Provider: "kimi", Model: "kimi-for-coding"}
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		t.Fatal("expected partial resolution for known preset")
	}
	if resolved.APIKey != "" {
		t.Fatalf("expected empty api key, got %q", resolved.APIKey)
	}
	if resolved.BaseURL != "https://api.kimi.com/coding/v1" {
		t.Fatalf("expected default base url, got %q", resolved.BaseURL)
	}
	if !resolved.Pinned {
		t.Fatal("expected Pinned=true")
	}
}

func TestResolveSelectionUnknownProviderNoEnv(t *testing.T) {
	t.Parallel()
	resolver := NewSelectionResolver(
		func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} },
		WithEnvLookup(func(string) (string, bool) { return "", false }),
	)

	selection := Selection{Mode: "cli", Provider: "totally_unknown", Model: "some-model"}
	_, ok := resolver.Resolve(selection)
	if ok {
		t.Fatal("expected unknown provider with no env to fail resolution")
	}
}
