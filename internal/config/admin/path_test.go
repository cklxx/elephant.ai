package admin

import (
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "alex/internal/config"
)

func TestResolveStorePathDefaultsToHomeWhenUnset(t *testing.T) {
	t.Setenv("CONFIG_ADMIN_STORE_PATH", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	expected := filepath.Join(home, ".alex", "runtime-overrides.json")
	if got := ResolveStorePath(runtimeconfig.DefaultEnvLookup); got != expected {
		t.Fatalf("expected default path %q, got %q", expected, got)
	}
}

func TestResolveStorePathFallsBackWhenHomeMissing(t *testing.T) {
	t.Setenv("CONFIG_ADMIN_STORE_PATH", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	got := ResolveStorePath(runtimeconfig.DefaultEnvLookup)
	suffix := filepath.Join(".alex", "runtime-overrides.json")
	if strings.HasSuffix(got, suffix) {
		return // os.UserHomeDir resolved a home directory even though env vars were empty
	}
	expected := filepath.Join("configs", "runtime-overrides.json")
	if got != expected {
		t.Fatalf("expected fallback path %q, got %q", expected, got)
	}
}

func TestResolveStorePathUsesEnv(t *testing.T) {
	t.Setenv("CONFIG_ADMIN_STORE_PATH", "./custom/path.json")
	if got := ResolveStorePath(runtimeconfig.DefaultEnvLookup); got != "./custom/path.json" {
		t.Fatalf("expected env path, got %q", got)
	}
}

func TestResolveStorePathHonorsAlias(t *testing.T) {
	t.Setenv("CONFIG_ADMIN_STORE_PATH", "")
	t.Setenv("ALEX_CONFIG_STORE_PATH", "/tmp/alias.json")
	lookup := runtimeconfig.DefaultEnvLookupWithAliases()
	if got := ResolveStorePath(lookup); got != "/tmp/alias.json" {
		t.Fatalf("expected alias path, got %q", got)
	}
}
