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
		DisableTUI:          true,
		FollowTranscript:    true,
		FollowStream:        false,
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
		"OPENAI_API_KEY":             "test-key",
		"OPENROUTER_API_KEY":         "test-key",
		"LLM_PROVIDER":               "openai",
		"ALEX_LLM_PROVIDER":          "openai",
		"LLM_MODEL":                  "gpt-4",
		"ALEX_MODEL_NAME":            "gpt-4",
		"LLM_BASE_URL":               "https://example.com",
		"TAVILY_API_KEY":             "tavily-key",
		"ALEX_TAVILY_API_KEY":        "tavily-key",
		"ALEX_ENV":                   "production",
		"ALEX_VERBOSE":               "true",
		"ALEX_NO_TUI":                "true",
		"ALEX_TUI_FOLLOW_TRANSCRIPT": "true",
		"ALEX_TUI_FOLLOW_STREAM":     "false",
		"ALEX_FOLLOW_TRANSCRIPT":     "true",
		"ALEX_FOLLOW_STREAM":         "false",
		"LLM_MAX_ITERATIONS":         "200",
		"LLM_MAX_TOKENS":             "4096",
		"LLM_TEMPERATURE":            "0.2",
		"LLM_TOP_P":                  "0.9",
		"LLM_STOP":                   "STOP,DONE",
		"ALEX_SESSION_DIR":           "/tmp/sessions",
		"ALEX_COST_DIR":              "/tmp/costs",
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
