package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLark_FailsWhenLarkDisabled(t *testing.T) {
	// No Lark config → Enabled defaults to false → fail fast.
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	err := RunLark("")
	if err == nil {
		t.Fatal("expected RunLark to fail when Lark is disabled")
	}
	if !strings.Contains(err.Error(), "channels.lark.enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLark_FailsWhenCredentialsMissing(t *testing.T) {
	// Enable Lark via YAML but leave AppID/AppSecret empty.
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `channels:
  lark:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("LLM_PROVIDER", "mock")

	err := RunLark("")
	if err == nil {
		t.Fatal("expected RunLark to fail when credentials are missing")
	}
	if !strings.Contains(err.Error(), "app_id") && !strings.Contains(err.Error(), "app_secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}
