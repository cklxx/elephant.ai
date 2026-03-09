package di

import (
	"alex/internal/app/agent/preparation"
	"alex/internal/infra/llm"
	runtimeconfig "alex/internal/shared/config"
	providerinfo "alex/internal/shared/provider"
	"golang.org/x/time/rate"
)

func (b *containerBuilder) buildLLMFactory() *llm.Factory {
	llmFactory := llm.NewFactory()
	llmFactory.EnableHealth()
	llmFactory.SetCacheOptions(b.config.LLMCacheSize, b.config.LLMCacheTTL)
	if b.config.UserRateLimitRPS > 0 {
		llmFactory.EnableUserRateLimit(rate.Limit(b.config.UserRateLimitRPS), b.config.UserRateLimitBurst)
	}
	if b.config.KimiRateLimitRPS > 0 {
		llmFactory.EnableKimiRateLimit(rate.Limit(b.config.KimiRateLimitRPS), b.config.KimiRateLimitBurst)
	}
	if rules := buildFallbackRules(b.config.LLMFallbackRules); len(rules) > 0 {
		llmFactory.SetFallbackRules(rules)
		b.logger.Info("LLM fallback rules configured: %d rule(s)", len(rules))
	}
	return llmFactory
}

func buildFallbackRules(configs []runtimeconfig.LLMFallbackRuleConfig) map[string]llm.FallbackRule {
	if len(configs) == 0 {
		return nil
	}
	rules := make(map[string]llm.FallbackRule, len(configs))
	for _, cfg := range configs {
		if cfg.Model == "" || cfg.FallbackProvider == "" || cfg.FallbackModel == "" {
			continue
		}
		rules[cfg.Model] = llm.FallbackRule{
			Provider: cfg.FallbackProvider,
			Model:    cfg.FallbackModel,
			APIKey:   cfg.FallbackAPIKey,
			BaseURL:  cfg.FallbackBaseURL,
		}
	}
	return rules
}

// buildCredentialRefresher creates a function that re-resolves CLI credentials
// at task execution time. This ensures long-running servers (e.g. Lark) use
// fresh tokens even after the startup token expires (Codex).
func buildCredentialRefresher() preparation.CredentialRefresher {
	return func(provider string) (string, string, bool) {
		creds := runtimeconfig.LoadCLICredentials()
		switch providerinfo.Family(provider) {
		case providerinfo.FamilyCodex:
			if creds.Codex.APIKey != "" {
				return creds.Codex.APIKey, creds.Codex.BaseURL, true
			}
		case providerinfo.FamilyAnthropic:
			if creds.Claude.APIKey != "" {
				return creds.Claude.APIKey, creds.Claude.BaseURL, true
			}
		}
		return "", "", false
	}
}
