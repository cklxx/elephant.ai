package llm

import (
	"testing"

	portsllm "alex/internal/domain/agent/ports/llm"
)

func TestFactoryAcceptsOpenAICompatibleProviderAliases(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	factory.DisableRetry()

	config := portsllm.LLMConfig{}
	for _, provider := range []string{"openai", "openrouter", "deepseek", "kimi", "glm", "minimax"} {
		if _, err := factory.GetClient(provider, "gpt-4o-mini", config); err != nil {
			t.Fatalf("GetClient(%q) returned error: %v", provider, err)
		}
	}
}
