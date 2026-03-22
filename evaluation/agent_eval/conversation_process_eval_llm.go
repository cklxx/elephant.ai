package agent_eval

import (
	"os"
	"testing"

	llminfra "alex/internal/infra/llm"
	portsllm "alex/internal/domain/agent/ports/llm"
)

// newConversationEvalLLMClient creates an LLM client from environment variables.
// Required: OPENAI_API_KEY (or ANTHROPIC_API_KEY) and optionally EVAL_MODEL, EVAL_PROVIDER, EVAL_BASE_URL.
func newConversationEvalLLMClient(t *testing.T) portsllm.LLMClient {
	t.Helper()

	provider := os.Getenv("EVAL_PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	model := os.Getenv("EVAL_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if provider == "anthropic" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		t.Skipf("no API key for provider %s; set OPENAI_API_KEY or ANTHROPIC_API_KEY", provider)
	}

	factory := llminfra.NewFactory()
	cfg := portsllm.LLMConfig{
		APIKey: apiKey,
	}
	if baseURL := os.Getenv("EVAL_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	client, err := factory.GetClient(provider, model, cfg)
	if err != nil {
		t.Fatalf("create LLM client: %v", err)
	}
	return client
}
