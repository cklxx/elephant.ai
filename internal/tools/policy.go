package tools

import (
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Core config types
// ---------------------------------------------------------------------------

// ToolRetryConfig controls retry behavior for tool executions.
type ToolRetryConfig struct {
	MaxRetries     int           `yaml:"max_retries" json:"max_retries"`
	InitialBackoff time.Duration `yaml:"initial_backoff" json:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff" json:"max_backoff"`
	BackoffFactor  float64       `yaml:"backoff_factor" json:"backoff_factor"`
}

// ToolTimeoutConfig controls per-tool timeout overrides.
type ToolTimeoutConfig struct {
	Default time.Duration            `yaml:"default" json:"default"`
	PerTool map[string]time.Duration `yaml:"per_tool" json:"per_tool"`
}

// ---------------------------------------------------------------------------
// Policy rules — per-context scoping
// ---------------------------------------------------------------------------

// PolicyRule is a YAML-driven rule that applies timeout/retry/access overrides
// when a tool call matches its selector. Rules are evaluated in order; the
// first matching rule wins for each setting it specifies.
//
// Example YAML:
//
//	rules:
//	  - name: lark-write-ops
//	    match:
//	      tools: ["lark_calendar_*", "lark_task_*"]
//	      categories: ["lark"]
//	      dangerous: true
//	    timeout: 30s
//	    retry:
//	      max_retries: 0
//	    enabled: true
//	  - name: sandbox-heavy
//	    match:
//	      tools: ["code_execute", "sandbox_*"]
//	      channels: ["web"]
//	    timeout: 300s
//	    retry:
//	      max_retries: 3
type PolicyRule struct {
	Name    string          `yaml:"name" json:"name"`
	Match   PolicySelector  `yaml:"match" json:"match"`
	Timeout *time.Duration  `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retry   *ToolRetryConfig `yaml:"retry,omitempty" json:"retry,omitempty"`
	Enabled *bool           `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// PolicySelector determines whether a rule applies. All non-empty fields
// must match (AND logic). Within a slice field (e.g. Tools), any match is
// sufficient (OR logic). Glob patterns with '*' are supported in Tools.
type PolicySelector struct {
	Tools      []string `yaml:"tools,omitempty" json:"tools,omitempty"`           // tool name globs
	Categories []string `yaml:"categories,omitempty" json:"categories,omitempty"` // ToolMetadata.Category
	Tags       []string `yaml:"tags,omitempty" json:"tags,omitempty"`             // any ToolMetadata.Tag
	Channels   []string `yaml:"channels,omitempty" json:"channels,omitempty"`     // delivery channel (cli, web, lark, wechat)
	Dangerous  *bool    `yaml:"dangerous,omitempty" json:"dangerous,omitempty"`   // ToolMetadata.Dangerous
}

// ToolCallContext carries runtime context about the current tool invocation
// so the policy engine can evaluate per-context selectors.
type ToolCallContext struct {
	ToolName  string
	Category  string
	Tags      []string
	Dangerous bool
	Channel   string // cli, web, lark, wechat
}

// ---------------------------------------------------------------------------
// ToolPolicyConfig — top-level YAML structure
// ---------------------------------------------------------------------------

// ToolPolicyConfig combines timeout, retry, and rule-based configuration.
type ToolPolicyConfig struct {
	Timeout ToolTimeoutConfig `yaml:"timeout" json:"timeout"`
	Retry   ToolRetryConfig   `yaml:"retry" json:"retry"`
	Rules   []PolicyRule      `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// DefaultToolPolicyConfig returns sensible defaults:
//   - Default timeout: 120s
//   - Max retries: 2 (caller should use RetryConfigFor to get 0 for dangerous tools)
//   - Initial backoff: 1s, max backoff: 30s, factor: 2.0
func DefaultToolPolicyConfig() ToolPolicyConfig {
	return ToolPolicyConfig{
		Timeout: ToolTimeoutConfig{
			Default: 120 * time.Second,
			PerTool: map[string]time.Duration{},
		},
		Retry: ToolRetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
		},
	}
}

// ---------------------------------------------------------------------------
// ToolPolicy interface + implementation
// ---------------------------------------------------------------------------

// ToolPolicy determines timeout and retry behavior per tool.
type ToolPolicy interface {
	// TimeoutFor returns the execution timeout for the named tool.
	TimeoutFor(toolName string) time.Duration

	// RetryConfigFor returns the retry configuration for the named tool.
	// When dangerous is true, retries are suppressed (MaxRetries = 0).
	RetryConfigFor(toolName string, dangerous bool) ToolRetryConfig

	// Resolve evaluates all rules against the full ToolCallContext and
	// returns the merged policy result.
	Resolve(ctx ToolCallContext) ResolvedPolicy
}

// ResolvedPolicy is the final, flattened result of evaluating all policy
// rules for a specific tool call.
type ResolvedPolicy struct {
	Timeout time.Duration
	Retry   ToolRetryConfig
	Enabled bool // false = tool call blocked by policy
}

// configToolPolicy implements ToolPolicy backed by ToolPolicyConfig.
type configToolPolicy struct {
	cfg ToolPolicyConfig
}

// NewToolPolicy creates a ToolPolicy from the given configuration.
func NewToolPolicy(cfg ToolPolicyConfig) ToolPolicy {
	return &configToolPolicy{cfg: cfg}
}

// TimeoutFor returns the per-tool timeout if configured, otherwise the global default.
func (p *configToolPolicy) TimeoutFor(toolName string) time.Duration {
	if d, ok := p.cfg.Timeout.PerTool[toolName]; ok && d > 0 {
		return d
	}
	if p.cfg.Timeout.Default > 0 {
		return p.cfg.Timeout.Default
	}
	return 120 * time.Second
}

// RetryConfigFor returns the global retry config for safe tools.
// Dangerous tools always receive zero retries to avoid repeating side-effects.
func (p *configToolPolicy) RetryConfigFor(toolName string, dangerous bool) ToolRetryConfig {
	if dangerous {
		return ToolRetryConfig{
			MaxRetries:     0,
			InitialBackoff: p.cfg.Retry.InitialBackoff,
			MaxBackoff:     p.cfg.Retry.MaxBackoff,
			BackoffFactor:  p.cfg.Retry.BackoffFactor,
		}
	}
	return p.cfg.Retry
}

// Resolve evaluates rules in order against the provided context.
// The first matching rule's non-nil fields override the defaults.
func (p *configToolPolicy) Resolve(ctx ToolCallContext) ResolvedPolicy {
	result := ResolvedPolicy{
		Timeout: p.TimeoutFor(ctx.ToolName),
		Retry:   p.RetryConfigFor(ctx.ToolName, ctx.Dangerous),
		Enabled: true,
	}

	for _, rule := range p.cfg.Rules {
		if !matchesSelector(rule.Match, ctx) {
			continue
		}
		if rule.Timeout != nil {
			result.Timeout = *rule.Timeout
		}
		if rule.Retry != nil {
			result.Retry = *rule.Retry
		}
		if rule.Enabled != nil {
			result.Enabled = *rule.Enabled
		}
		break // first match wins
	}

	return result
}

// ---------------------------------------------------------------------------
// Selector matching
// ---------------------------------------------------------------------------

func matchesSelector(sel PolicySelector, ctx ToolCallContext) bool {
	if len(sel.Tools) > 0 && !matchesAnyGlob(sel.Tools, ctx.ToolName) {
		return false
	}
	if len(sel.Categories) > 0 && !containsCI(sel.Categories, ctx.Category) {
		return false
	}
	if len(sel.Tags) > 0 && !intersectsCI(sel.Tags, ctx.Tags) {
		return false
	}
	if len(sel.Channels) > 0 && !containsCI(sel.Channels, ctx.Channel) {
		return false
	}
	if sel.Dangerous != nil && *sel.Dangerous != ctx.Dangerous {
		return false
	}
	return true
}

// matchesAnyGlob checks if name matches any pattern. Supports trailing '*'.
func matchesAnyGlob(patterns []string, name string) bool {
	for _, p := range patterns {
		if p == "*" || p == name {
			return true
		}
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	return false
}

func containsCI(haystack []string, needle string) bool {
	lower := strings.ToLower(needle)
	for _, h := range haystack {
		if strings.ToLower(h) == lower {
			return true
		}
	}
	return false
}

func intersectsCI(required, actual []string) bool {
	set := make(map[string]struct{}, len(actual))
	for _, a := range actual {
		set[strings.ToLower(a)] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[strings.ToLower(r)]; ok {
			return true
		}
	}
	return false
}
