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

	result := p.Resolve(ToolCallContext{ToolName: "memory_search", Tags: []string{"memory", "fast"}})
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

func TestResolve_FieldLevelOverrides(t *testing.T) {
	t30 := 30 * time.Second
	retry := ToolRetryConfig{MaxRetries: 4, InitialBackoff: 2 * time.Second, MaxBackoff: 20 * time.Second, BackoffFactor: 2.0}
	disabled := false
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "timeout-only",
			Match:   PolicySelector{Tools: []string{"*"}},
			Timeout: &t30,
		},
		{
			Name:  "retry-only",
			Match: PolicySelector{Tools: []string{"*"}},
			Retry: &retry,
		},
		{
			Name:    "disable-only",
			Match:   PolicySelector{Tools: []string{"*"}},
			Enabled: &disabled,
		},
	}
	p := NewToolPolicy(cfg)

	result := p.Resolve(ToolCallContext{ToolName: "anything"})
	if result.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", result.Timeout)
	}
	if result.Retry.MaxRetries != 4 {
		t.Errorf("Retry.MaxRetries = %d, want 4", result.Retry.MaxRetries)
	}
	if result.Enabled {
		t.Error("Enabled = true, want false")
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

// ---------------------------------------------------------------------------
// Default policy rules tests
// ---------------------------------------------------------------------------

func TestDefaultPolicyRules_MediaGeneration(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	result := p.Resolve(ToolCallContext{ToolName: "text_to_image", Category: "design"})
	if result.Timeout != 300*time.Second {
		t.Errorf("media timeout = %v, want 300s", result.Timeout)
	}
	if result.Retry.MaxRetries != 2 {
		t.Errorf("media retries = %d, want 2", result.Retry.MaxRetries)
	}
}

func TestDefaultPolicyRules_LarkWriteOps(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	result := p.Resolve(ToolCallContext{
		ToolName:  "lark_calendar_create",
		Category:  "lark",
		Dangerous: true,
	})
	if result.Timeout != 30*time.Second {
		t.Errorf("lark write timeout = %v, want 30s", result.Timeout)
	}
	if result.Retry.MaxRetries != 0 {
		t.Errorf("lark write retries = %d, want 0", result.Retry.MaxRetries)
	}
}

func TestDefaultPolicyRules_LarkReadOps(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	// Lark read (not dangerous) should NOT match lark-write-ops rule
	result := p.Resolve(ToolCallContext{
		ToolName:  "lark_calendar_query",
		Category:  "lark",
		Dangerous: false,
	})
	// Falls through to default 120s (no rule matches lark+safe)
	if result.Timeout != 120*time.Second {
		t.Errorf("lark read timeout = %v, want 120s", result.Timeout)
	}
	if result.Retry.MaxRetries != 2 {
		t.Errorf("lark read retries = %d, want 2", result.Retry.MaxRetries)
	}
}

func TestDefaultPolicyRules_WebTools(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	result := p.Resolve(ToolCallContext{ToolName: "web_search", Category: "web"})
	if result.Timeout != 60*time.Second {
		t.Errorf("web timeout = %v, want 60s", result.Timeout)
	}
	if result.Retry.MaxRetries != 3 {
		t.Errorf("web retries = %d, want 3", result.Retry.MaxRetries)
	}
}

func TestDefaultPolicyRules_ExecutionLong(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	for _, tool := range []string{"code_execute", "shell_exec", "execute_code", "bash"} {
		result := p.Resolve(ToolCallContext{ToolName: tool, Category: "execution", Dangerous: true})
		if result.Timeout != 300*time.Second {
			t.Errorf("%s timeout = %v, want 300s", tool, result.Timeout)
		}
	}
}

func TestDefaultPolicyRules_DangerousCatchAll(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	// file_write: dangerous, category=file_operations — no earlier rule matches
	result := p.Resolve(ToolCallContext{
		ToolName:  "file_write",
		Category:  "file_operations",
		Dangerous: true,
	})
	if result.Retry.MaxRetries != 0 {
		t.Errorf("dangerous catch-all retries = %d, want 0", result.Retry.MaxRetries)
	}
	// Timeout should be default 120s (catch-all only sets retry)
	if result.Timeout != 120*time.Second {
		t.Errorf("dangerous catch-all timeout = %v, want 120s", result.Timeout)
	}
}

func TestDefaultPolicyRules_SafeToolFallsThrough(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())

	// A safe tool in an unmatched category uses global defaults
	result := p.Resolve(ToolCallContext{
		ToolName: "memory_search",
		Category: "memory",
	})
	if result.Timeout != 120*time.Second {
		t.Errorf("safe fallthrough timeout = %v, want 120s", result.Timeout)
	}
	if result.Retry.MaxRetries != 2 {
		t.Errorf("safe fallthrough retries = %d, want 2", result.Retry.MaxRetries)
	}
	if !result.Enabled {
		t.Error("safe tool should be enabled")
	}
}

func TestDefaultPolicyRules_Count(t *testing.T) {
	rules := DefaultPolicyRules()
	if len(rules) != 7 {
		t.Errorf("DefaultPolicyRules() has %d rules, want 7", len(rules))
	}
}

func TestDefaultToolPolicyConfigWithRules(t *testing.T) {
	cfg := DefaultToolPolicyConfigWithRules()
	if len(cfg.Rules) == 0 {
		t.Error("DefaultToolPolicyConfigWithRules should have rules")
	}
	if cfg.Timeout.Default != 120*time.Second {
		t.Errorf("default timeout = %v, want 120s", cfg.Timeout.Default)
	}
}

// ---------------------------------------------------------------------------
// Safety level tests
// ---------------------------------------------------------------------------

func TestResolve_MatchBySafetyLevel(t *testing.T) {
	timeout := 45 * time.Second
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:    "l4-rule",
			Match:   PolicySelector{SafetyLevels: []int{4}},
			Timeout: &timeout,
		},
	}
	p := NewToolPolicy(cfg)

	// L4 tool should match.
	result := p.Resolve(ToolCallContext{ToolName: "delete_all", SafetyLevel: 4})
	if result.Timeout != 45*time.Second {
		t.Errorf("L4 Timeout = %v, want 45s", result.Timeout)
	}

	// L3 tool should NOT match.
	result2 := p.Resolve(ToolCallContext{ToolName: "exec_code", SafetyLevel: 3})
	if result2.Timeout != 120*time.Second {
		t.Errorf("L3 Timeout = %v, want 120s (no match)", result2.Timeout)
	}

	// L0 (unset) tool should NOT match.
	result3 := p.Resolve(ToolCallContext{ToolName: "unknown"})
	if result3.Timeout != 120*time.Second {
		t.Errorf("L0 Timeout = %v, want 120s (no match)", result3.Timeout)
	}
}

func TestResolve_SafetyLevelInResult(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfig())
	result := p.Resolve(ToolCallContext{ToolName: "bash", SafetyLevel: 3})
	if result.SafetyLevel != 3 {
		t.Errorf("SafetyLevel = %d, want 3", result.SafetyLevel)
	}
}

func TestResolve_SafetyLevelANDWithCategory(t *testing.T) {
	noRetry := ToolRetryConfig{MaxRetries: 0}
	cfg := DefaultToolPolicyConfig()
	cfg.Rules = []PolicyRule{
		{
			Name:  "l3-lark",
			Match: PolicySelector{SafetyLevels: []int{3}, Categories: []string{"lark"}},
			Retry: &noRetry,
		},
	}
	p := NewToolPolicy(cfg)

	// L3 + lark → matches.
	result := p.Resolve(ToolCallContext{ToolName: "lark_create", Category: "lark", SafetyLevel: 3})
	if result.Retry.MaxRetries != 0 {
		t.Errorf("L3+lark retries = %d, want 0", result.Retry.MaxRetries)
	}

	// L3 + web → no match.
	result2 := p.Resolve(ToolCallContext{ToolName: "web_action", Category: "web", SafetyLevel: 3})
	if result2.Retry.MaxRetries != 2 {
		t.Errorf("L3+web retries = %d, want 2 (no match)", result2.Retry.MaxRetries)
	}
}

func TestDefaultPolicyRules_L4Irreversible(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())
	result := p.Resolve(ToolCallContext{
		ToolName:    "lark_calendar_delete",
		SafetyLevel: 4,
	})
	if result.Retry.MaxRetries != 0 {
		t.Errorf("L4 retries = %d, want 0", result.Retry.MaxRetries)
	}
	if result.Timeout != 60*time.Second {
		t.Errorf("L4 timeout = %v, want 60s", result.Timeout)
	}
}

func TestDefaultPolicyRules_L3HighImpact(t *testing.T) {
	p := NewToolPolicy(DefaultToolPolicyConfigWithRules())
	result := p.Resolve(ToolCallContext{
		ToolName:    "bash",
		SafetyLevel: 3,
	})
	if result.Retry.MaxRetries != 0 {
		t.Errorf("L3 retries = %d, want 0", result.Retry.MaxRetries)
	}
	if result.Timeout != 60*time.Second {
		t.Errorf("L3 timeout = %v, want 60s", result.Timeout)
	}
}

func TestContainsInt(t *testing.T) {
	if !containsInt([]int{1, 2, 3}, 2) {
		t.Error("expected true for 2 in [1,2,3]")
	}
	if containsInt([]int{1, 2, 3}, 4) {
		t.Error("expected false for 4 in [1,2,3]")
	}
	if containsInt(nil, 1) {
		t.Error("expected false for nil slice")
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
