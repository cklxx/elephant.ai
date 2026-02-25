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
