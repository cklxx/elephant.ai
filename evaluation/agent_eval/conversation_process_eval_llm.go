package agent_eval

import (
	"os"
	"strings"
	"testing"

	llminfra "alex/internal/infra/llm"
	portsllm "alex/internal/domain/agent/ports/llm"
)

// newConversationEvalLLMClient creates an LLM client from environment variables.
// Auto-detects kimi keys (sk-kimi-*) and sets provider accordingly.
// Override with EVAL_PROVIDER, EVAL_MODEL, EVAL_BASE_URL.
func newConversationEvalLLMClient(t *testing.T) portsllm.LLMClient {
	t.Helper()

	// Priority: EVAL_* overrides > LLM_API_KEY (Ark) > OPENAI_API_KEY > ANTHROPIC_API_KEY
	provider := os.Getenv("EVAL_PROVIDER")
	model := os.Getenv("EVAL_MODEL")
	apiKey := os.Getenv("EVAL_API_KEY")
	baseURL := os.Getenv("EVAL_BASE_URL")

	// Fallback to production config from configs/config.yaml defaults.
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		t.Skip("no API key; set EVAL_API_KEY, LLM_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY")
	}

	// Auto-detect provider from key format.
	if provider == "" {
		if strings.HasPrefix(apiKey, "sk-kimi-") {
			provider = "kimi"
		} else if strings.HasPrefix(apiKey, "sk-ant-") || os.Getenv("ANTHROPIC_API_KEY") == apiKey {
			provider = "anthropic"
		} else {
			provider = "openai"
		}
	}
	if model == "" {
		switch provider {
		case "kimi":
			model = "kimi-k2-0711-preview"
		case "anthropic":
			model = "claude-sonnet-4-20250514"
		default:
			model = "gpt-4o-mini"
		}
	}

	factory := llminfra.NewFactory()
	factory.DisableRetry() // No retry/circuit breaker for eval — fail fast, report clearly.
	cfg := portsllm.LLMConfig{
		APIKey: apiKey,
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	// Auto-set base URL for known providers.
	if cfg.BaseURL == "" {
		switch provider {
		case "kimi":
			cfg.BaseURL = "https://api.moonshot.cn/v1"
		}
	}

	t.Logf("eval LLM: provider=%s model=%s", provider, model)

	client, err := factory.GetClient(provider, model, cfg)
	if err != nil {
		t.Fatalf("create LLM client: %v", err)
	}
	return client
}
