package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "eval-server.yaml")
	content := `
environment: staging
allowed_origins:
  - https://console.example
judge:
  enabled: true
  provider: openai
  model: gpt-5-mini
`
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Port != "8081" {
		t.Fatalf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.Environment != "staging" {
		t.Fatalf("Environment = %q, want staging", cfg.Environment)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://console.example" {
		t.Fatalf("AllowedOrigins = %#v, want one configured origin", cfg.AllowedOrigins)
	}
	if !cfg.Judge.Enabled || cfg.Judge.Provider != "openai" || cfg.Judge.Model != "gpt-5-mini" {
		t.Fatalf("Judge = %#v, want configured judge settings", cfg.Judge)
	}
}

func TestResolveConfig(t *testing.T) {
	t.Run("explicit path wins", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "custom.yaml")
		if err := os.WriteFile(configPath, []byte("port: \"9191\"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		cfg, err := resolveConfig(configPath)
		if err != nil {
			t.Fatalf("resolveConfig() error = %v", err)
		}
		if cfg.Port != "9191" {
			t.Fatalf("Port = %q, want 9191", cfg.Port)
		}
	})

	t.Run("configs candidate preferred", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v", err)
		}
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "configs"), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "configs", "eval-server.yaml"), []byte("port: \"7001\"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "eval-server.yaml"), []byte("port: \"7002\"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("Chdir() error = %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(wd)
		})

		cfg, err := resolveConfig("")
		if err != nil {
			t.Fatalf("resolveConfig() error = %v", err)
		}
		if cfg.Port != "7001" {
			t.Fatalf("Port = %q, want configs candidate", cfg.Port)
		}
	})

	t.Run("falls back to defaults", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v", err)
		}
		dir := t.TempDir()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("Chdir() error = %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(wd)
		})

		cfg, err := resolveConfig("")
		if err != nil {
			t.Fatalf("resolveConfig() error = %v", err)
		}
		if cfg.Port != "8081" || cfg.Environment != "evaluation" {
			t.Fatalf("Default config = %#v, want default values", cfg)
		}
	})
}

func TestCreateLLMJudgeReturnsWrappedErrorForUnknownProvider(t *testing.T) {
	_, err := createLLMJudge(JudgeConfig{
		Enabled:  true,
		Provider: "unknown-provider",
		Model:    "irrelevant",
		APIKey:   "test-key",
	})
	if err == nil {
		t.Fatal("createLLMJudge() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create LLM client") {
		t.Fatalf("error = %q, want wrapped factory error", err)
	}
}
