package admin

import (
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "alex/internal/config"
)

func TestResolveStorePathDefaultsToHomeWhenUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	expected := filepath.Join(home, ".alex", "config.yaml")
	if got := ResolveStorePath(runtimeconfig.DefaultEnvLookup); got != expected {
		t.Fatalf("expected default path %q, got %q", expected, got)
	}
}

func TestResolveStorePathFallsBackWhenHomeMissing(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	got := ResolveStorePath(runtimeconfig.DefaultEnvLookup)
	suffix := filepath.Join(".alex", "config.yaml")
	if strings.HasSuffix(got, suffix) {
		return // os.UserHomeDir resolved a home directory even though env vars were empty
	}
	expected := filepath.Join("configs", "config.yaml")
	if got != expected {
		t.Fatalf("expected fallback path %q, got %q", expected, got)
	}
}

func TestResolveStorePathUsesEnv(t *testing.T) {
	t.Setenv("ALEX_CONFIG_PATH", "./custom/path.yaml")
	if got := ResolveStorePath(runtimeconfig.DefaultEnvLookup); got != "./custom/path.yaml" {
		t.Fatalf("expected env path, got %q", got)
	}
}
