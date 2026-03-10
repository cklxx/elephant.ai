package bootstrap

import (
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestExpandHome_TildePrefix(t *testing.T) {
	got := expandHome("~/documents/goals")
	if got == "~/documents/goals" {
		t.Fatal("expandHome should resolve ~ to home directory")
	}
	if len(got) < 10 {
		t.Fatalf("expandHome(~/documents/goals) = %q, looks too short", got)
	}
}

func TestExpandHome_TildeOnly(t *testing.T) {
	got := expandHome("~")
	if got == "~" {
		t.Fatal("expandHome should resolve bare ~ to home directory")
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	got := expandHome("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("expandHome(/absolute/path) = %q, want /absolute/path", got)
	}
}

func TestExpandHome_Empty(t *testing.T) {
	got := expandHome("")
	if got != "" {
		t.Errorf("expandHome('') = %q, want empty", got)
	}
}

func TestExpandHome_RelativePath(t *testing.T) {
	got := expandHome("relative/path")
	if got != "relative/path" {
		t.Errorf("expandHome(relative/path) = %q, want relative/path", got)
	}
}

func TestResolveGoalsRoot_FromConfig(t *testing.T) {
	cfg := Config{}
	cfg.Runtime.Proactive.OKR.GoalsRoot = "/custom/goals"
	got := resolveGoalsRoot(cfg)
	if got != "/custom/goals" {
		t.Errorf("resolveGoalsRoot = %q, want /custom/goals", got)
	}
}

func TestResolveGoalsRoot_ExpandsTilde(t *testing.T) {
	cfg := Config{}
	cfg.Runtime.Proactive.OKR.GoalsRoot = "~/my-goals"
	got := resolveGoalsRoot(cfg)
	if got == "~/my-goals" {
		t.Fatal("resolveGoalsRoot should expand ~ in the configured path")
	}
}

func TestResolveGoalsRoot_FallsBackToDefault(t *testing.T) {
	cfg := Config{}
	cfg.Runtime.Proactive.OKR.GoalsRoot = ""
	got := resolveGoalsRoot(cfg)
	if got == "" {
		t.Fatal("resolveGoalsRoot should return default when config is empty")
	}
}

func TestBuildNotifiers_NoConfig(t *testing.T) {
	cfg := Config{}
	notifier := BuildNotifiers(cfg, "Test", nil)
	if notifier == nil {
		t.Fatal("BuildNotifiers should return a non-nil notifier even without config")
	}
}

func TestBuildNotifiers_LarkDisabled(t *testing.T) {
	cfg := Config{}
	// Lark not enabled, no moltbook key => should return NopNotifier
	notifier := BuildNotifiers(cfg, "Test", nil)
	if notifier == nil {
		t.Fatal("expected NopNotifier, got nil")
	}
}

func TestLookupFirstNonEmptyEnv_Found(t *testing.T) {
	lookup := func(key string) (string, bool) {
		m := map[string]string{
			"EMPTY_VAR": "",
			"GOOD_VAR":  "value-1",
			"ALSO_GOOD": "value-2",
		}
		v, ok := m[key]
		return v, ok
	}
	got := lookupFirstNonEmptyEnv(lookup, "MISSING", "EMPTY_VAR", "GOOD_VAR", "ALSO_GOOD")
	if got != "value-1" {
		t.Errorf("lookupFirstNonEmptyEnv = %q, want value-1", got)
	}
}

func TestLookupFirstNonEmptyEnv_NoneFound(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "", false
	}
	got := lookupFirstNonEmptyEnv(lookup, "A", "B", "C")
	if got != "" {
		t.Errorf("lookupFirstNonEmptyEnv = %q, want empty", got)
	}
}

func TestLookupFirstNonEmptyEnv_NilLookup(t *testing.T) {
	got := lookupFirstNonEmptyEnv(nil, "A")
	if got != "" {
		t.Errorf("lookupFirstNonEmptyEnv(nil) = %q, want empty", got)
	}
}

func TestLookupFirstNonEmptyEnv_TrimsWhitespace(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "PADDED" {
			return "  trimmed  ", true
		}
		return "", false
	}
	got := lookupFirstNonEmptyEnv(lookup, "PADDED")
	if got != "trimmed" {
		t.Errorf("lookupFirstNonEmptyEnv = %q, want trimmed", got)
	}
}

func TestLookupFirstNonEmptyEnv_SkipsWhitespaceOnly(t *testing.T) {
	lookup := func(key string) (string, bool) {
		m := map[string]string{
			"SPACES": "   ",
			"REAL":   "found",
		}
		v, ok := m[key]
		return v, ok
	}
	got := lookupFirstNonEmptyEnv(lookup, "SPACES", "REAL")
	if got != "found" {
		t.Errorf("lookupFirstNonEmptyEnv = %q, want found", got)
	}
}

func TestLookupFirstNonEmptyEnv_NoKeys(t *testing.T) {
	lookup := func(key string) (string, bool) {
		return "value", true
	}
	got := lookupFirstNonEmptyEnv(lookup)
	if got != "" {
		t.Errorf("lookupFirstNonEmptyEnv with no keys = %q, want empty", got)
	}
}

func TestApplyServerHTTPConfig_NilServer(t *testing.T) {
	cfg := &Config{Port: "8080"}
	file := runtimeconfig.FileConfig{Server: nil}
	applyServerHTTPConfig(cfg, file)
	if cfg.Port != "8080" {
		t.Errorf("Port changed to %q, should stay 8080 when Server is nil", cfg.Port)
	}
}

func TestApplyServerHTTPConfig_OverridesPort(t *testing.T) {
	cfg := &Config{Port: "8080"}
	file := runtimeconfig.FileConfig{
		Server: &runtimeconfig.ServerConfig{
			Port: "9090",
		},
	}
	applyServerHTTPConfig(cfg, file)
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
}

func TestApplyServerHTTPConfig_IgnoresEmptyPort(t *testing.T) {
	cfg := &Config{Port: "8080"}
	file := runtimeconfig.FileConfig{
		Server: &runtimeconfig.ServerConfig{
			Port: "  ",
		},
	}
	applyServerHTTPConfig(cfg, file)
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, should remain 8080 for whitespace-only port", cfg.Port)
	}
}
