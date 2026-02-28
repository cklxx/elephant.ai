package config

import (
	"os"
	"testing"
)

func TestAnthropicProviderResolvesClaudeCLIOverGenericKey(t *testing.T) {
	// Simulates the runtime switch scenario: config.yaml has api_key from
	// LLM_API_KEY (for ARK/OpenAI), but provider is switched to "anthropic".
	// The resolver should pick up the Claude CLI token instead of keeping
	// the generic key.
	fileData := []byte(`
runtime:
  llm_provider: "anthropic"
  llm_model: "claude-sonnet-4-6"
  api_key: "${LLM_API_KEY}"
`)

	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/tmp/config.yaml":
			return fileData, nil
		case "/home/test/.claude/.credentials.json":
			return []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-test-token"}}`), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	cfg, meta, err := Load(
		WithConfigPath("/tmp/config.yaml"),
		WithFileReader(readFile),
		WithHomeDir(func() (string, error) { return "/home/test", nil }),
		WithEnv(envMap{
			"LLM_API_KEY": "ark-generic-key",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", cfg.LLMProvider)
	}
	if cfg.APIKey != "sk-ant-oat01-test-token" {
		t.Fatalf("expected Claude CLI token to override generic key, got %q", cfg.APIKey)
	}
	if meta.Source("api_key") != SourceClaudeCLI {
		t.Fatalf("expected claude_cli source, got %s", meta.Source("api_key"))
	}
}

func TestAnthropicProviderResolvesEnvKeyOverGenericKey(t *testing.T) {
	// When ANTHROPIC_API_KEY env is set, it should win over a generic LLM_API_KEY
	// even when the config file already set api_key.
	fileData := []byte(`
runtime:
  llm_provider: "anthropic"
  llm_model: "claude-sonnet-4-6"
  api_key: "${LLM_API_KEY}"
`)

	cfg, _, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"LLM_API_KEY":       "ark-generic-key",
			"ANTHROPIC_API_KEY": "sk-ant-api-key",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKey != "sk-ant-api-key" {
		t.Fatalf("expected ANTHROPIC_API_KEY to win over LLM_API_KEY, got %q", cfg.APIKey)
	}
}

func TestAnthropicOverrideKeyIsPreserved(t *testing.T) {
	// When the user explicitly overrides api_key, it must be kept even if
	// Claude CLI credentials are available.
	fileData := []byte(`
runtime:
  llm_provider: "anthropic"
  llm_model: "claude-sonnet-4-6"
`)

	overrideKey := "sk-ant-user-provided-key"
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/tmp/config.yaml":
			return fileData, nil
		case "/home/test/.claude/.credentials.json":
			return []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-cli-token"}}`), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	cfg, meta, err := Load(
		WithConfigPath("/tmp/config.yaml"),
		WithFileReader(readFile),
		WithHomeDir(func() (string, error) { return "/home/test", nil }),
		WithEnv(envMap{}.Lookup),
		WithOverrides(Overrides{APIKey: &overrideKey}),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKey != overrideKey {
		t.Fatalf("expected override key to be preserved, got %q", cfg.APIKey)
	}
	if meta.Source("api_key") != SourceOverride {
		t.Fatalf("expected override source, got %s", meta.Source("api_key"))
	}
}

func TestRuntimeProviderSwitchToAnthropicPicksUpCLICreds(t *testing.T) {
	// Core scenario: config file uses openai with LLM_API_KEY, but runtime
	// override switches to anthropic. Claude CLI token should be used.
	fileData := []byte(`
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o-mini"
  api_key: "${LLM_API_KEY}"
`)

	overrideProvider := "anthropic"
	overrideModel := "claude-sonnet-4-6"
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/tmp/config.yaml":
			return fileData, nil
		case "/home/test/.claude/.credentials.json":
			return []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-setup-token"}}`), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	cfg, meta, err := Load(
		WithConfigPath("/tmp/config.yaml"),
		WithFileReader(readFile),
		WithHomeDir(func() (string, error) { return "/home/test", nil }),
		WithEnv(envMap{
			"LLM_API_KEY": "ark-openai-key",
		}.Lookup),
		WithOverrides(Overrides{
			LLMProvider: &overrideProvider,
			LLMModel:    &overrideModel,
		}),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "anthropic" {
		t.Fatalf("expected anthropic provider, got %q", cfg.LLMProvider)
	}
	if cfg.APIKey != "sk-ant-oat01-setup-token" {
		t.Fatalf("expected Claude CLI token, got %q", cfg.APIKey)
	}
	if meta.Source("api_key") != SourceClaudeCLI {
		t.Fatalf("expected claude_cli source, got %s", meta.Source("api_key"))
	}
}

func TestCodexProviderResolvesCodexCLIOverGenericKey(t *testing.T) {
	// Same pattern for codex: when provider is codex but key is from generic
	// source, Codex CLI credentials should be picked up.
	fileData := []byte(`
runtime:
  llm_provider: "codex"
  llm_model: "gpt-5.2-codex"
  api_key: "${LLM_API_KEY}"
  base_url: "https://chatgpt.com/backend-api/codex"
`)

	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/tmp/config.yaml":
			return fileData, nil
		case "/home/test/.codex/auth.json":
			return []byte(`{"tokens":{"access_token":"codex-cli-token","account_id":"acct-123"}}`), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	cfg, meta, err := Load(
		WithConfigPath("/tmp/config.yaml"),
		WithFileReader(readFile),
		WithHomeDir(func() (string, error) { return "/home/test", nil }),
		WithEnv(envMap{
			"LLM_API_KEY": "generic-key",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKey != "codex-cli-token" {
		t.Fatalf("expected Codex CLI token, got %q", cfg.APIKey)
	}
	if meta.Source("api_key") != SourceCodexCLI {
		t.Fatalf("expected codex_cli source, got %s", meta.Source("api_key"))
	}
}

func TestOpenAIProviderKeepsGenericKeyWhenNoSpecificKey(t *testing.T) {
	// OpenAI provider without OPENAI_API_KEY should keep LLM_API_KEY.
	fileData := []byte(`
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o-mini"
  api_key: "${LLM_API_KEY}"
`)

	cfg, _, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"LLM_API_KEY": "generic-key",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKey != "generic-key" {
		t.Fatalf("expected generic key to be kept for openai, got %q", cfg.APIKey)
	}
}
