package config

import "testing"

func TestValidateRuntimeConfigProductionRequiresAPIKey(t *testing.T) {
	report := ValidateRuntimeConfig(RuntimeConfig{
		Profile:     RuntimeProfileProduction,
		LLMProvider: "openai",
		LLMModel:    "gpt-4o-mini",
	})
	if len(report.Errors) == 0 {
		t.Fatalf("expected validation errors in production when API key is missing")
	}
	if report.Errors[0].ID != "llm-api-key" {
		t.Fatalf("expected llm-api-key error, got %q", report.Errors[0].ID)
	}
}

func TestValidateRuntimeConfigQuickstartWarnsAndDisablesOptionalTools(t *testing.T) {
	report := ValidateRuntimeConfig(RuntimeConfig{
		Profile:     RuntimeProfileQuickstart,
		LLMProvider: "openai",
		LLMModel:    "gpt-4o-mini",
	})
	if len(report.Errors) != 0 {
		t.Fatalf("expected no blocking errors in quickstart, got %d", len(report.Errors))
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected warnings in quickstart profile")
	}
	if len(report.DisabledTools) == 0 {
		t.Fatalf("expected disabled tools in quickstart profile when optional keys are missing")
	}
}

func TestNormalizeRuntimeProfileDefaultsToStandard(t *testing.T) {
	if got := NormalizeRuntimeProfile(""); got != RuntimeProfileStandard {
		t.Fatalf("expected empty profile to normalize to standard, got %q", got)
	}
	if got := NormalizeRuntimeProfile("unknown-profile"); got != RuntimeProfileStandard {
		t.Fatalf("expected invalid profile to normalize to standard, got %q", got)
	}
	if got := NormalizeRuntimeProfile("prod"); got != RuntimeProfileProduction {
		t.Fatalf("expected prod alias to normalize to production, got %q", got)
	}
}
