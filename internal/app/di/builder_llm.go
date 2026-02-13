package di

import (
	"strings"

	"alex/internal/app/agent/preparation"
	"alex/internal/infra/llm"
	runtimeconfig "alex/internal/shared/config"
	"golang.org/x/time/rate"
)

func (b *containerBuilder) buildLLMFactory() *llm.Factory {
	llmFactory := llm.NewFactory()
	llmFactory.SetCacheOptions(b.config.LLMCacheSize, b.config.LLMCacheTTL)
	if b.config.UserRateLimitRPS > 0 {
		llmFactory.EnableUserRateLimit(rate.Limit(b.config.UserRateLimitRPS), b.config.UserRateLimitBurst)
	}
	return llmFactory
}

// buildCredentialRefresher creates a function that re-resolves CLI credentials
// at task execution time. This ensures long-running servers (e.g. Lark) use
// fresh tokens even after the startup token expires (Codex).
func buildCredentialRefresher() preparation.CredentialRefresher {
	return func(provider string) (string, string, bool) {
		provider = strings.ToLower(strings.TrimSpace(provider))
		creds := runtimeconfig.LoadCLICredentials()
		switch provider {
		case "codex", "openai-responses", "responses":
			if creds.Codex.APIKey != "" {
				return creds.Codex.APIKey, creds.Codex.BaseURL, true
			}
		case "anthropic", "claude":
			if creds.Claude.APIKey != "" {
				return creds.Claude.APIKey, creds.Claude.BaseURL, true
			}
		}
		return "", "", false
	}
}
