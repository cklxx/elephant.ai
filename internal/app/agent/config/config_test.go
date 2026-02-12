package config

import (
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestDefaultLLMProfilePrefersExplicitProfile(t *testing.T) {
	cfg := Config{
		LLMProvider: "mock",
		LLMModel:    "legacy-model",
		APIKey:      "legacy-key",
		BaseURL:     "https://legacy.local",
		LLMProfile: runtimeconfig.LLMProfile{
			Provider: "openai",
			Model:    "gpt-4o-mini",
			APIKey:   "profile-key",
			BaseURL:  "https://api.openai.com/v1",
		},
	}

	profile := cfg.DefaultLLMProfile()
	if profile.Provider != "openai" || profile.Model != "gpt-4o-mini" || profile.APIKey != "profile-key" {
		t.Fatalf("unexpected explicit profile: %+v", profile)
	}
}

func TestDefaultLLMProfileFallsBackToLegacyFields(t *testing.T) {
	cfg := Config{
		LLMProvider: "mock",
		LLMModel:    "legacy-model",
		APIKey:      "legacy-key",
		BaseURL:     "https://legacy.local",
	}

	profile := cfg.DefaultLLMProfile()
	if profile.Provider != "mock" || profile.Model != "legacy-model" || profile.APIKey != "legacy-key" {
		t.Fatalf("unexpected fallback profile: %+v", profile)
	}
}

func TestSmallAndVisionProfiles(t *testing.T) {
	cfg := Config{
		LLMProfile: runtimeconfig.LLMProfile{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "key",
			BaseURL:  "https://api.openai.com/v1",
		},
		LLMSmallProvider: "openai",
		LLMSmallModel:    "gpt-4o-mini",
		LLMVisionModel:   "gpt-4o-vision",
	}

	small, ok := cfg.SmallLLMProfile()
	if !ok {
		t.Fatal("expected small profile")
	}
	if small.Model != "gpt-4o-mini" || small.Provider != "openai" {
		t.Fatalf("unexpected small profile: %+v", small)
	}

	vision, ok := cfg.VisionLLMProfile()
	if !ok {
		t.Fatal("expected vision profile")
	}
	if vision.Model != "gpt-4o-vision" || vision.Provider != "openai" {
		t.Fatalf("unexpected vision profile: %+v", vision)
	}
}
