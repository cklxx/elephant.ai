package tools

import (
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestTimeoutFor_Default(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfig())
	got := p.TimeoutFor("unknown_tool")
	if got != 120*time.Second {
		t.Errorf("TimeoutFor(unknown) = %v, want 120s", got)
	}
}

func TestTimeoutFor_PerTool(t *testing.T) {
	cfg := DefaultToolPolicyConfig()
	cfg.Timeout.PerTool["slow_tool"] = 300 * time.Second
	p := NewToolPolicy(cfg)

	if got := p.TimeoutFor("slow_tool"); got != 300*time.Second {
		t.Errorf("TimeoutFor(slow_tool) = %v, want 300s", got)
	}
	if got := p.TimeoutFor("fast_tool"); got != 120*time.Second {
		t.Errorf("TimeoutFor(fast_tool) = %v, want 120s", got)
	}
}

func TestRetryConfigFor_Safe(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfig())
	rc := p.RetryConfigFor("safe_tool", false)
	if rc.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", rc.MaxRetries)
	}
}

func TestRetryConfigFor_Dangerous(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfig())
	rc := p.RetryConfigFor("dangerous_tool", true)
	if rc.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0 for dangerous", rc.MaxRetries)
	}
}

func TestResolve_NoRules(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfig())
	result := p.Resolve(ToolCallContext{ToolName: "test", Channel: "cli"})
	if !result.Enabled {
		t.Error("expected Enabled=true with no rules")
	}
	if result.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", result.Timeout)
	}
}

func TestResolve_MatchByToolGlob(t *testing.T) {
	timeout := 30 * time.Second
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "lark-write",
			Match:   PolicySelector{Tools: []string{"lark_*"}},
			Timeout: &timeout,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "lark_calendar_update"})
	if result.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", result.Timeout)
	}

	result2 := p.Resolve(ToolCallContext{ToolName: "web_search"})
	if result2.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s (no match)", result2.Timeout)
	}
}

func TestResolve_MatchByCategory(t *testing.T) {
	timeout := 60 * time.Second
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "media-tools",
			Match:   PolicySelector{Categories: []string{"media"}},
			Timeout: &timeout,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "seedream_image", Category: "media"})
	if result.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", result.Timeout)
	}
}

func TestResolve_MatchByChannel(t *testing.T) {
	retry := ToolRetryConfig{MaxRetries: 5, InitialBackoff: 2 * time.Second, MaxBackoff: 60 * time.Second, BackoffFactor: 2.0}
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:  "web-channel",
			Match: PolicySelector{Channels: []string{"web"}},
			Retry: &retry,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "web_search", Channel: "web"})
	if result.Retry.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", result.Retry.MaxRetries)
	}

	result2 := p.Resolve(ToolCallContext{ToolName: "web_search", Channel: "cli"})
	if result2.Retry.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2 (no match)", result2.Retry.MaxRetries)
	}
}

func TestResolve_MatchByDangerous(t *testing.T) {
	enabled := false
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "block-dangerous",
			Match:   PolicySelector{Dangerous: ptr(true)},
			Enabled: &enabled,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "lark_task_delete", Dangerous: true})
	if result.Enabled {
		t.Error("expected Enabled=false for dangerous tool")
	}

	result2 := p.Resolve(ToolCallContext{ToolName: "web_search", Dangerous: false})
	if !result2.Enabled {
		t.Error("expected Enabled=true for safe tool")
	}
}

func TestResolve_MatchByTags(t *testing.T) {
	timeout := 10 * time.Second
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "fast-tools",
			Match:   PolicySelector{Tags: []string{"fast"}},
			Timeout: &timeout,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "memory_recall", Tags: []string{"memory", "fast"}})
	if result.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", result.Timeout)
	}
}

func TestResolve_FirstMatchWins(t *testing.T) {
	t10 := 10 * time.Second
	t20 := 20 * time.Second
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "first",
			Match:   PolicySelector{Tools: []string{"*"}},
			Timeout: &t10,
		},
		{
			Name:    "second",
			Match:   PolicySelector{Tools: []string{"*"}},
			Timeout: &t20,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "anything"})
	if result.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s (first match)", result.Timeout)
	}
}

func TestResolve_ANDLogicAcrossFields(t *testing.T) {
	timeout := 5 * time.Second
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "lark-web-only",
			Match:   PolicySelector{Tools: []string{"lark_*"}, Channels: []string{"web"}},
			Timeout: &timeout,
		},
	}
	p := NewToolPolicy(cfg)

	// Both match → rule applies
	result := p.Resolve(ToolCallContext{ToolName: "lark_calendar_update", Channel: "web"})
	if result.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", result.Timeout)
	}

	// Tool matches but channel doesn't → no match
	result2 := p.Resolve(ToolCallContext{ToolName: "lark_calendar_update", Channel: "cli"})
	if result2.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s (no match)", result2.Timeout)
	}
}

func TestMatchesAnyGlob(t *testing.T) {
	tests := []struct {
		patterns []string
		name     string
		want     bool
	}{
		{[]string{"*"}, "anything", true},
		{[]string{"lark_*"}, "lark_calendar_update", true},
		{[]string{"lark_*"}, "web_search", false},
		{[]string{"exact"}, "exact", true},
		{[]string{"exact"}, "exactX", false},
		{[]string{"a_*", "b_*"}, "b_tool", true},
	}
	for _, tt := range tests {
		if got := matchesAnyGlob(tt.patterns, tt.name); got != tt.want {
			t.Errorf("matchesAnyGlob(%v, %q) = %v, want %v", tt.patterns, tt.name, got, tt.want)
		}
	}
}
