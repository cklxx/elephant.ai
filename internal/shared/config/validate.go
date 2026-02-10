package config

import "strings"

// ValidationIssue represents a single validation finding.
type ValidationIssue struct {
	ID      string
	Message string
	Hint    string
}

// DisabledTool captures a tool disabled by profile/capability policy.
type DisabledTool struct {
	Name   string
	Reason string
}

// ValidationReport summarizes runtime config validation findings.
type ValidationReport struct {
	Profile       string
	Errors        []ValidationIssue
	Warnings      []ValidationIssue
	DisabledTools []DisabledTool
}

// HasErrors reports whether the validation report contains blocking errors.
func (r ValidationReport) HasErrors() bool {
	return len(r.Errors) > 0
}

// NormalizeRuntimeProfile coerces profile values to supported runtime profile constants.
func NormalizeRuntimeProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", RuntimeProfileStandard:
		return RuntimeProfileStandard
	case RuntimeProfileQuickstart, "quick-start", "quick_start":
		return RuntimeProfileQuickstart
	case RuntimeProfileProduction, "prod":
		return RuntimeProfileProduction
	default:
		return RuntimeProfileStandard
	}
}

// ProviderRequiresAPIKey reports whether the provider requires API key authentication.
func ProviderRequiresAPIKey(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "mock", "llama.cpp", "llamacpp", "llama-cpp", "ollama":
		return false
	default:
		return true
	}
}

// ValidateRuntimeConfig validates runtime configuration and computes optional capability gating.
func ValidateRuntimeConfig(cfg RuntimeConfig) ValidationReport {
	profile := NormalizeRuntimeProfile(cfg.Profile)
	report := ValidationReport{Profile: profile}

	provider := strings.TrimSpace(cfg.LLMProvider)
	model := strings.TrimSpace(cfg.LLMModel)
	apiKey := strings.TrimSpace(cfg.APIKey)
	tavilyKey := strings.TrimSpace(cfg.TavilyAPIKey)
	arkKey := strings.TrimSpace(cfg.ArkAPIKey)

	if provider == "" {
		report.Errors = append(report.Errors, ValidationIssue{
			ID:      "llm-provider",
			Message: "llm_provider is required",
			Hint:    "Set runtime.llm_provider in config.yaml or via managed override.",
		})
	}

	if model == "" {
		report.Errors = append(report.Errors, ValidationIssue{
			ID:      "llm-model",
			Message: "llm_model is required",
			Hint:    "Set runtime.llm_model to a valid model name.",
		})
	}

	if ProviderRequiresAPIKey(provider) && apiKey == "" {
		issue := ValidationIssue{
			ID:      "llm-api-key",
			Message: "API key is required for the selected provider",
			Hint:    "Set runtime.api_key, provider-specific env key, or LLM_API_KEY.",
		}
		if profile == RuntimeProfileQuickstart {
			issue.Hint = "Set runtime.api_key/provider key/LLM_API_KEY for real responses. Quickstart can continue with mock fallback."
			report.Warnings = append(report.Warnings, issue)
		} else {
			report.Errors = append(report.Errors, issue)
		}
	}

	if tavilyKey == "" {
		report.Warnings = append(report.Warnings, ValidationIssue{
			ID:      "tavily-key",
			Message: "Tavily API key is not configured",
			Hint:    "Set TAVILY_API_KEY to enable web_search with external retrieval.",
		})
	}

	if profile == RuntimeProfileQuickstart {
		addDisabled := func(name, reason string) {
			report.DisabledTools = append(report.DisabledTools, DisabledTool{Name: name, Reason: reason})
		}

		if tavilyKey == "" {
			addDisabled("web_search", "missing TAVILY_API_KEY in quickstart profile")
		}
		if arkKey == "" {
			addDisabled("text_to_image", "missing ARK_API_KEY in quickstart profile")
			addDisabled("image_to_image", "missing ARK_API_KEY in quickstart profile")
			addDisabled("vision_analyze", "missing ARK_API_KEY in quickstart profile")
			addDisabled("video_generate", "missing ARK_API_KEY in quickstart profile")
		}
	}

	return report
}
