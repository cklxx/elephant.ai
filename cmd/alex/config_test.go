package main

import "testing"

func TestLoadConfigDefaultTemperatureUsesPresetButNotMarkedSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LLM_TEMPERATURE", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.Temperature != 0.7 {
		t.Fatalf("expected default temperature 0.7, got %.2f", cfg.Temperature)
	}
	if cfg.TemperatureProvided {
		t.Fatalf("expected default temperature to not be marked as explicitly set")
	}
}

func TestLoadConfigHonorsZeroTemperatureFromEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LLM_TEMPERATURE", "0")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.Temperature != 0 {
		t.Fatalf("expected temperature to be 0, got %.2f", cfg.Temperature)
	}
	if !cfg.TemperatureProvided {
		t.Fatalf("expected temperature to be marked as explicitly set when provided via env")
	}
}

func TestLoadConfigVerboseAndEnvironmentFromEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ALEX_VERBOSE", "yes")
	t.Setenv("ALEX_ENV", "staging")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if !cfg.Verbose {
		t.Fatalf("expected verbose true when env set")
	}
	if cfg.Environment != "staging" {
		t.Fatalf("expected environment staging, got %s", cfg.Environment)
	}
}
