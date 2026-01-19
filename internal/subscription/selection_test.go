package subscription

import (
	"testing"

	runtimeconfig "alex/internal/config"
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
