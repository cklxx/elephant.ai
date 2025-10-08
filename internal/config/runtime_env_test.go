package config

import "testing"

func TestRuntimeEnvLookup(t *testing.T) {
	cfg := RuntimeConfig{
		LLMProvider:         "openai",
		LLMModel:            "gpt-4",
		APIKey:              "test-key",
		BaseURL:             "https://example.com",
		TavilyAPIKey:        "tavily-key",
		Environment:         "production",
		Verbose:             true,
		MaxIterations:       200,
		MaxTokens:           4096,
		Temperature:         0.2,
		TemperatureProvided: true,
		TopP:                0.9,
		StopSequences:       []string{"STOP", "DONE"},
		SessionDir:          "/tmp/sessions",
		CostDir:             "/tmp/costs",
	}

	lookup := RuntimeEnvLookup(cfg, nil)

	cases := map[string]string{
		"OPENAI_API_KEY":      "test-key",
		"LLM_PROVIDER":        "openai",
		"ALEX_LLM_PROVIDER":   "openai",
		"LLM_MODEL":           "gpt-4",
		"ALEX_MODEL_NAME":     "gpt-4",
		"LLM_BASE_URL":        "https://example.com",
		"TAVILY_API_KEY":      "tavily-key",
		"ALEX_TAVILY_API_KEY": "tavily-key",
		"ALEX_ENV":            "production",
		"ALEX_VERBOSE":        "true",
		"LLM_MAX_ITERATIONS":  "200",
		"LLM_MAX_TOKENS":      "4096",
		"LLM_TEMPERATURE":     "0.2",
		"LLM_TOP_P":           "0.9",
		"LLM_STOP":            "STOP,DONE",
		"ALEX_SESSION_DIR":    "/tmp/sessions",
		"ALEX_COST_DIR":       "/tmp/costs",
	}

	for key, expected := range cases {
		value, ok := lookup(key)
		if !ok {
			t.Fatalf("expected key %s to be present", key)
		}
		if value != expected {
			t.Fatalf("lookup(%s) = %q, want %q", key, value, expected)
		}
	}

	if _, ok := lookup("OPENROUTER_API_KEY"); ok {
		t.Fatal("expected OPENROUTER_API_KEY to be absent for openai provider")
	}
}

func TestRuntimeEnvLookupProviderSpecificAPIKeys(t *testing.T) {
	baseValues := map[string]string{
		"OPENAI_API_KEY":     "env-openai",
		"OPENROUTER_API_KEY": "env-openrouter",
	}

	base := func(key string) (string, bool) {
		value, ok := baseValues[key]
		return value, ok
	}

	cfg := RuntimeConfig{
		LLMProvider: "openai",
		APIKey:      "cfg-openai",
	}

	lookup := RuntimeEnvLookup(cfg, base)

	assertValue := func(key, expected string) {
		t.Helper()
		value, ok := lookup(key)
		if !ok {
			t.Fatalf("expected %s to be present", key)
		}
		if value != expected {
			t.Fatalf("expected %s=%q, got %q", key, expected, value)
		}
	}

	assertValue("OPENAI_API_KEY", "cfg-openai")
	assertValue("OPENROUTER_API_KEY", "env-openrouter")

	cfg = RuntimeConfig{
		LLMProvider: "openrouter",
		APIKey:      "cfg-openrouter",
	}

	lookup = RuntimeEnvLookup(cfg, base)

	assertValue("OPENROUTER_API_KEY", "cfg-openrouter")
	assertValue("OPENAI_API_KEY", "env-openai")
}

func TestRuntimeEnvLookupFallsBack(t *testing.T) {
	base := func(key string) (string, bool) {
		if key == "FROM_BASE" {
			return "base-value", true
		}
		return "", false
	}

	lookup := RuntimeEnvLookup(RuntimeConfig{}, base)

	value, ok := lookup("FROM_BASE")
	if !ok {
		t.Fatal("expected lookup to fall back to base")
	}
	if value != "base-value" {
		t.Fatalf("expected base value, got %q", value)
	}
}
